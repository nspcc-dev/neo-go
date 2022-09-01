package native_test

import (
	"encoding/json"
	"math"
	"math/big"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/contracts"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

var pathToInternalContracts = filepath.Join("..", "..", "..", "..", "internal", "contracts")

func newOracleClient(t *testing.T) *neotest.ContractInvoker {
	return newNativeClient(t, nativenames.Oracle)
}

func TestOracle_GetSetPrice(t *testing.T) {
	testGetSet(t, newOracleClient(t), "Price", native.DefaultOracleRequestPrice, 1, math.MaxInt64)
}

func TestOracle_GetSetPriceCache(t *testing.T) {
	testGetSetCache(t, newOracleClient(t), "Price", native.DefaultOracleRequestPrice)
}

func putOracleRequest(t *testing.T, oracleInvoker *neotest.ContractInvoker,
	url string, filter *string, cb string, userData []byte, gas int64, errStr ...string) {
	var filtItem interface{}
	if filter != nil {
		filtItem = *filter
	}
	if len(errStr) == 0 {
		oracleInvoker.Invoke(t, stackitem.Null{}, "requestURL", url, filtItem, cb, userData, gas)
		return
	}
	oracleInvoker.InvokeFail(t, errStr[0], "requestURL", url, filtItem, cb, userData, gas)
}

func TestOracle_Request(t *testing.T) {
	oracleCommitteeInvoker := newOracleClient(t)
	e := oracleCommitteeInvoker.Executor
	managementCommitteeInvoker := e.CommitteeInvoker(e.NativeHash(t, nativenames.Management))
	designationCommitteeInvoker := e.CommitteeInvoker(e.NativeHash(t, nativenames.Designation))
	gasCommitteeInvoker := e.CommitteeInvoker(e.NativeHash(t, nativenames.Gas))

	cs := contracts.GetOracleContractState(t, pathToInternalContracts, e.Validator.ScriptHash(), 1)
	nBytes, err := cs.NEF.Bytes()
	require.NoError(t, err)
	mBytes, err := json.Marshal(cs.Manifest)
	require.NoError(t, err)
	expected, err := cs.ToStackItem()
	require.NoError(t, err)
	managementCommitteeInvoker.Invoke(t, expected, "deploy", nBytes, mBytes)
	helperValidatorInvoker := e.ValidatorInvoker(cs.Hash)

	gasForResponse := int64(2000_1234)
	var filter = "flt"
	userData := []byte("custom info")
	putOracleRequest(t, helperValidatorInvoker, "url", &filter, "handle", userData, gasForResponse)

	// Designate single Oracle node.
	oracleNode := e.NewAccount(t)
	designationCommitteeInvoker.Invoke(t, stackitem.Null{}, "designateAsRole", int(noderoles.Oracle), []interface{}{oracleNode.(neotest.SingleSigner).Account().PublicKey().Bytes()})
	err = oracleNode.(neotest.SingleSigner).Account().ConvertMultisig(1, []*keys.PublicKey{oracleNode.(neotest.SingleSigner).Account().PublicKey()})
	require.NoError(t, err)
	oracleNodeMulti := neotest.NewMultiSigner(oracleNode.(neotest.SingleSigner).Account())
	gasCommitteeInvoker.Invoke(t, true, "transfer", gasCommitteeInvoker.CommitteeHash, oracleNodeMulti.ScriptHash(), 100_0000_0000, nil)

	// Finish.
	prepareResponseTx := func(t *testing.T, requestID uint64) *transaction.Transaction {
		script := native.CreateOracleResponseScript(oracleCommitteeInvoker.Hash)

		tx := transaction.New(script, 1000_0000)
		tx.Nonce = neotest.Nonce()
		tx.ValidUntilBlock = e.Chain.BlockHeight() + 1
		tx.Attributes = []transaction.Attribute{{
			Type: transaction.OracleResponseT,
			Value: &transaction.OracleResponse{
				ID:     requestID,
				Code:   transaction.Success,
				Result: []byte{4, 8, 15, 16, 23, 42},
			},
		}}
		tx.Signers = []transaction.Signer{
			{
				Account: oracleNodeMulti.ScriptHash(),
				Scopes:  transaction.None,
			},
			{
				Account: oracleCommitteeInvoker.Hash,
				Scopes:  transaction.None,
			},
		}
		tx.NetworkFee = 1000_1234
		tx.Scripts = []transaction.Witness{
			{
				InvocationScript:   oracleNodeMulti.SignHashable(uint32(e.Chain.GetConfig().Magic), tx),
				VerificationScript: oracleNodeMulti.Script(),
			},
			{
				InvocationScript:   []byte{},
				VerificationScript: []byte{},
			},
		}
		return tx
	}
	tx := prepareResponseTx(t, 0)
	e.AddNewBlock(t, tx)
	e.CheckHalt(t, tx.Hash(), stackitem.Null{})

	// Ensure that callback was called.
	si := e.Chain.GetStorageItem(cs.ID, []byte("lastOracleResponse"))
	require.NotNil(t, si)
	actual, err := stackitem.Deserialize(si)
	require.NoError(t, err)
	require.Equal(t, stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray([]byte("url")),
		stackitem.NewByteArray(userData),
		stackitem.NewBigInteger(big.NewInt(int64(tx.Attributes[0].Value.(*transaction.OracleResponse).Code))),
		stackitem.NewByteArray(tx.Attributes[0].Value.(*transaction.OracleResponse).Result),
	}), actual)

	// Check that the processed request is removed. We can't access GetRequestInternal directly,
	// but adding a response to this request should fail due to invalid request error.
	tx = prepareResponseTx(t, 0)
	err = e.Chain.VerifyTx(tx)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "oracle tx points to invalid request"))

	t.Run("ErrorOnFinish", func(t *testing.T) {
		putOracleRequest(t, helperValidatorInvoker, "url", nil, "handle", []byte{1, 2}, gasForResponse)
		tx := prepareResponseTx(t, 1)
		e.AddNewBlock(t, tx)
		e.CheckFault(t, tx.Hash(), "ABORT")

		// Check that the processed request is cleaned up even if callback failed. We can't
		// access GetRequestInternal directly, but adding a response to this request
		// should fail due to invalid request error.
		tx = prepareResponseTx(t, 1)
		err = e.Chain.VerifyTx(tx)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "oracle tx points to invalid request"))
	})
	t.Run("Reentrant", func(t *testing.T) {
		putOracleRequest(t, helperValidatorInvoker, "url", nil, "handleRecursive", []byte{}, gasForResponse)
		tx := prepareResponseTx(t, 2)
		e.AddNewBlock(t, tx)
		e.CheckFault(t, tx.Hash(), "Oracle.finish called from non-entry script")
		aer, err := e.Chain.GetAppExecResults(tx.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 2, len(aer[0].Events)) // OracleResponse + Invocation
	})
	t.Run("BadRequest", func(t *testing.T) {
		t.Run("non-UTF8 url", func(t *testing.T) {
			putOracleRequest(t, helperValidatorInvoker, "\xff", nil, "", []byte{1, 2}, gasForResponse, "invalid value: not UTF-8")
		})
		t.Run("non-UTF8 filter", func(t *testing.T) {
			var f = "\xff"
			putOracleRequest(t, helperValidatorInvoker, "url", &f, "", []byte{1, 2}, gasForResponse, "invalid value: not UTF-8")
		})
		t.Run("not enough gas", func(t *testing.T) {
			putOracleRequest(t, helperValidatorInvoker, "url", nil, "", nil, 1000, "not enough gas for response")
		})
		t.Run("disallowed callback", func(t *testing.T) {
			putOracleRequest(t, helperValidatorInvoker, "url", nil, "_deploy", nil, 1000_0000, "disallowed callback method (starts with '_')")
		})
	})
}
