package core

import (
	"bytes"
	"errors"
	gio "io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const oracleModulePath = "../services/oracle/"

func getOracleConfig(t *testing.T, bc *Blockchain, w, pass string) oracle.Config {
	return oracle.Config{
		Log:     zaptest.NewLogger(t),
		Network: netmode.UnitTestNet,
		MainCfg: config.OracleConfiguration{
			RefreshInterval: time.Second,
			UnlockWallet: config.Wallet{
				Path:     path.Join(oracleModulePath, w),
				Password: pass,
			},
		},
		Chain:          bc,
		Client:         newDefaultHTTPClient(),
		OracleScript:   bc.contracts.Oracle.NEF.Script,
		OracleResponse: bc.contracts.Oracle.GetOracleResponseScript(),
		OracleHash:     bc.contracts.Oracle.Hash,
	}
}

func getTestOracle(t *testing.T, bc *Blockchain, walletPath, pass string) (
	*wallet.Account,
	*oracle.Oracle,
	map[uint64]*responseWithSig,
	chan *transaction.Transaction) {
	m := make(map[uint64]*responseWithSig)
	ch := make(chan *transaction.Transaction, 5)
	orcCfg := getOracleConfig(t, bc, walletPath, pass)
	orcCfg.ResponseHandler = saveToMapBroadcaster{m}
	orcCfg.OnTransaction = saveTxToChan(ch)
	orcCfg.URIValidator = func(u *url.URL) error {
		if strings.HasPrefix(u.Host, "private") {
			return errors.New("private network")
		}
		return nil
	}
	orc, err := oracle.NewOracle(orcCfg)
	require.NoError(t, err)

	w, err := wallet.NewWalletFromFile(path.Join(oracleModulePath, walletPath))
	require.NoError(t, err)
	require.NoError(t, w.Accounts[0].Decrypt(pass))
	return w.Accounts[0], orc, m, ch
}

// Compatibility test from C# code.
// https://github.com/neo-project/neo-modules/blob/master/tests/Neo.Plugins.OracleService.Tests/UT_OracleService.cs#L61
func TestCreateResponseTx(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	require.Equal(t, int64(30), bc.GetBaseExecFee())
	require.Equal(t, int64(1000), bc.FeePerByte())
	acc, orc, _, _ := getTestOracle(t, bc, "./testdata/oracle1.json", "one")
	req := &state.OracleRequest{
		OriginalTxID:     util.Uint256{},
		GasForResponse:   100000000,
		URL:              "https://127.0.0.1/test",
		Filter:           new(string),
		CallbackContract: util.Uint160{},
		CallbackMethod:   "callback",
		UserData:         []byte{},
	}
	resp := &transaction.OracleResponse{
		ID:     1,
		Code:   transaction.Success,
		Result: []byte{0},
	}
	require.NoError(t, bc.contracts.Oracle.PutRequestInternal(1, req, bc.dao))
	orc.UpdateOracleNodes(keys.PublicKeys{acc.PrivateKey().PublicKey()})
	tx, err := orc.CreateResponseTx(int64(req.GasForResponse), 1, resp)
	require.NoError(t, err)
	assert.Equal(t, 167, tx.Size())
	assert.Equal(t, int64(2216640), tx.NetworkFee)
	assert.Equal(t, int64(97783360), tx.SystemFee)
}

func TestOracle_InvalidWallet(t *testing.T) {
	bc := newTestChain(t)

	_, err := oracle.NewOracle(getOracleConfig(t, bc, "./testdata/oracle1.json", "invalid"))
	require.Error(t, err)

	_, err = oracle.NewOracle(getOracleConfig(t, bc, "./testdata/oracle1.json", "one"))
	require.NoError(t, err)
}

func TestOracle(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	oracleCtr := bc.contracts.Oracle
	acc1, orc1, m1, ch1 := getTestOracle(t, bc, "./testdata/oracle1.json", "one")
	acc2, orc2, m2, ch2 := getTestOracle(t, bc, "./testdata/oracle2.json", "two")
	oracleNodes := keys.PublicKeys{acc1.PrivateKey().PublicKey(), acc2.PrivateKey().PublicKey()}
	// Must be set in native contract for tx verification.
	bc.setNodesByRole(t, true, native.RoleOracle, oracleNodes)
	orc1.UpdateOracleNodes(oracleNodes.Copy())
	orc2.UpdateOracleNodes(oracleNodes.Copy())

	cs := getOracleContractState(bc.contracts.Oracle.Hash)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))

	putOracleRequest(t, cs.Hash, bc, "http://get.1234", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "http://get.1234", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "http://get.timeout", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "http://get.notfound", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "http://get.forbidden", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "http://private.url", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "http://get.big", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "http://get.maxallowed", nil, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "http://get.maxallowed", nil, "handle", []byte{}, 100_000_000)

	flt := "Values[1]"
	putOracleRequest(t, cs.Hash, bc, "http://get.filter", &flt, "handle", []byte{}, 10_000_000)
	putOracleRequest(t, cs.Hash, bc, "http://get.filterinv", &flt, "handle", []byte{}, 10_000_000)

	checkResp := func(t *testing.T, id uint64, resp *transaction.OracleResponse) *state.OracleRequest {
		req, err := oracleCtr.GetRequestInternal(bc.dao, id)
		require.NoError(t, err)

		reqs := map[uint64]*state.OracleRequest{id: req}
		orc1.ProcessRequestsInternal(reqs)
		require.NotNil(t, m1[id])
		require.Equal(t, resp, m1[id].resp)
		require.Empty(t, ch1)
		return req
	}

	// Checks if tx is ready and valid.
	checkEmitTx := func(t *testing.T, ch chan *transaction.Transaction) {
		require.Len(t, ch, 1)
		tx := <-ch
		require.NoError(t, bc.verifyAndPoolTx(tx, bc.GetMemPool(), bc))
	}

	t.Run("NormalRequest", func(t *testing.T) {
		resp := &transaction.OracleResponse{
			ID:     0,
			Code:   transaction.Success,
			Result: []byte{1, 2, 3, 4},
		}
		req := checkResp(t, 0, resp)

		reqs := map[uint64]*state.OracleRequest{0: req}
		orc2.ProcessRequestsInternal(reqs)
		require.Equal(t, resp, m2[0].resp)
		require.Empty(t, ch2)

		t.Run("InvalidSignature", func(t *testing.T) {
			orc1.AddResponse(acc2.PrivateKey().PublicKey(), m2[0].resp.ID, []byte{1, 2, 3})
			require.Empty(t, ch1)
		})
		orc1.AddResponse(acc2.PrivateKey().PublicKey(), m2[0].resp.ID, m2[0].txSig)
		checkEmitTx(t, ch1)

		t.Run("FirstOtherThenMe", func(t *testing.T) {
			const reqID = 1

			resp := &transaction.OracleResponse{
				ID:     reqID,
				Code:   transaction.Success,
				Result: []byte{1, 2, 3, 4},
			}
			req := checkResp(t, reqID, resp)
			orc2.AddResponse(acc1.PrivateKey().PublicKey(), reqID, m1[reqID].txSig)
			require.Empty(t, ch2)

			reqs := map[uint64]*state.OracleRequest{reqID: req}
			orc2.ProcessRequestsInternal(reqs)
			require.Equal(t, resp, m2[reqID].resp)
			checkEmitTx(t, ch2)
		})
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("Timeout", func(t *testing.T) {
			checkResp(t, 2, &transaction.OracleResponse{
				ID:   2,
				Code: transaction.Timeout,
			})
		})
		t.Run("NotFound", func(t *testing.T) {
			checkResp(t, 3, &transaction.OracleResponse{
				ID:   3,
				Code: transaction.NotFound,
			})
		})
		t.Run("Forbidden", func(t *testing.T) {
			checkResp(t, 4, &transaction.OracleResponse{
				ID:   4,
				Code: transaction.Forbidden,
			})
		})
		t.Run("PrivateNetwork", func(t *testing.T) {
			checkResp(t, 5, &transaction.OracleResponse{
				ID:   5,
				Code: transaction.Forbidden,
			})
		})
		t.Run("Big", func(t *testing.T) {
			checkResp(t, 6, &transaction.OracleResponse{
				ID:   6,
				Code: transaction.ResponseTooLarge,
			})
		})
		t.Run("MaxAllowedSmallGAS", func(t *testing.T) {
			checkResp(t, 7, &transaction.OracleResponse{
				ID:   7,
				Code: transaction.InsufficientFunds,
			})
		})
	})
	t.Run("MaxAllowedEnoughGAS", func(t *testing.T) {
		checkResp(t, 8, &transaction.OracleResponse{
			ID:     8,
			Code:   transaction.Success,
			Result: make([]byte, transaction.MaxOracleResultSize),
		})
	})
	t.Run("WithFilter", func(t *testing.T) {
		checkResp(t, 9, &transaction.OracleResponse{
			ID:     9,
			Code:   transaction.Success,
			Result: []byte(`[2]`),
		})
		t.Run("invalid response", func(t *testing.T) {
			checkResp(t, 10, &transaction.OracleResponse{
				ID:   10,
				Code: transaction.Error,
			})
		})
	})
}

func TestOracleFull(t *testing.T) {
	bc := initTestChain(t, nil, nil)
	acc, orc, _, _ := getTestOracle(t, bc, "./testdata/oracle2.json", "two")
	mp := bc.GetMemPool()
	orc.OnTransaction = func(tx *transaction.Transaction) { _ = mp.Add(tx, bc) }
	bc.SetOracle(orc)

	cs := getOracleContractState(bc.contracts.Oracle.Hash)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))

	go bc.Run()
	defer bc.Close()
	go orc.Run()
	defer orc.Shutdown()

	bc.setNodesByRole(t, true, native.RoleOracle, keys.PublicKeys{acc.PrivateKey().PublicKey()})
	putOracleRequest(t, cs.Hash, bc, "http://get.1234", new(string), "handle", []byte{}, 10_000_000)

	require.Eventually(t, func() bool { return mp.Count() == 1 },
		time.Second*3, time.Millisecond*200)

	txes := mp.GetVerifiedTransactions()
	require.Len(t, txes, 1)
	require.True(t, txes[0].HasAttribute(transaction.OracleResponseT))
}

type saveToMapBroadcaster struct {
	m map[uint64]*responseWithSig
}

func (b saveToMapBroadcaster) SendResponse(_ *keys.PrivateKey, resp *transaction.OracleResponse, txSig []byte) {
	b.m[resp.ID] = &responseWithSig{
		resp:  resp,
		txSig: txSig,
	}
}
func (saveToMapBroadcaster) Run()      {}
func (saveToMapBroadcaster) Shutdown() {}

type responseWithSig struct {
	resp  *transaction.OracleResponse
	txSig []byte
}

func saveTxToChan(ch chan *transaction.Transaction) oracle.TxCallback {
	return func(tx *transaction.Transaction) {
		ch <- tx
	}
}

type (
	// httpClient implements oracle.HTTPClient with
	// mocked URL or responses.
	httpClient struct {
		responses map[string]testResponse
	}

	testResponse struct {
		code int
		body []byte
	}
)

// Get implements oracle.HTTPClient interface.
func (c *httpClient) Get(url string) (*http.Response, error) {
	resp, ok := c.responses[url]
	if ok {
		return &http.Response{
			StatusCode: resp.code,
			Body:       newResponseBody(resp.body),
		}, nil
	}
	return nil, errors.New("error during request")
}

func newDefaultHTTPClient() oracle.HTTPClient {
	return &httpClient{
		responses: map[string]testResponse{
			"http://get.1234": {
				code: http.StatusOK,
				body: []byte{1, 2, 3, 4},
			},
			"http://get.4321": {
				code: http.StatusOK,
				body: []byte{4, 3, 2, 1},
			},
			"http://get.timeout": {
				code: http.StatusRequestTimeout,
				body: []byte{},
			},
			"http://get.notfound": {
				code: http.StatusNotFound,
				body: []byte{},
			},
			"http://get.forbidden": {
				code: http.StatusForbidden,
				body: []byte{},
			},
			"http://private.url": {
				code: http.StatusOK,
				body: []byte("passwords"),
			},
			"http://get.big": {
				code: http.StatusOK,
				body: make([]byte, transaction.MaxOracleResultSize+1),
			},
			"http://get.maxallowed": {
				code: http.StatusOK,
				body: make([]byte, transaction.MaxOracleResultSize),
			},
			"http://get.filter": {
				code: http.StatusOK,
				body: []byte(`{"Values":["one", 2, 3],"Another":null}`),
			},
			"http://get.filterinv": {
				code: http.StatusOK,
				body: []byte{0xFF},
			},
		},
	}
}

func newResponseBody(resp []byte) gio.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(resp))
}
