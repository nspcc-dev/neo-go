package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/rpc/response"
	"github.com/CityOfZion/neo-go/pkg/rpc/response/result"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type executor struct {
	chain   *core.Blockchain
	handler http.HandlerFunc
}

const (
	defaultJSONRPC = "2.0"
	defaultID      = 1
)

type rpcTestCase struct {
	name   string
	params string
	fail   bool
	result func(e *executor) interface{}
	check  func(t *testing.T, e *executor, result interface{})
}

var rpcTestCases = map[string][]rpcTestCase{
	"getaccountstate": {
		{
			name:   "positive",
			params: `["AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU"]`,
			result: func(e *executor) interface{} { return &GetAccountStateResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*GetAccountStateResponse)
				require.True(t, ok)
				assert.Equal(t, 1, len(res.Result.Balances))
				assert.Equal(t, false, res.Result.Frozen)
			},
		},
		{
			name:   "positive null",
			params: `["AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"]`,
			result: func(e *executor) interface{} { return &GetAccountStateResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*GetAccountStateResponse)
				require.True(t, ok)
				assert.Equal(t, 0, len(res.Result.Balances))
				assert.Equal(t, false, res.Result.Frozen)
			},
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid address",
			params: `["notabase58"]`,
			fail:   true,
		},
	},
	"getcontractstate": {
		{
			name:   "positive",
			params: `["1a696b32e239dd5eace3f025cac0a193a5746a27"]`,
			result: func(e *executor) interface{} { return &GetContractStateResponce{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*GetContractStateResponce)
				require.True(t, ok)
				assert.Equal(t, byte(0), res.Result.Version)
				assert.Equal(t, util.Uint160{0x1a, 0x69, 0x6b, 0x32, 0xe2, 0x39, 0xdd, 0x5e, 0xac, 0xe3, 0xf0, 0x25, 0xca, 0xc0, 0xa1, 0x93, 0xa5, 0x74, 0x6a, 0x27}, res.Result.ScriptHash)
				assert.Equal(t, "0.99", res.Result.CodeVersion)
			},
		},
		{
			name:   "negative",
			params: `["6d1eeca891ee93de2b7a77eb91c26f3b3c04d6c3"]`,
			fail:   true,
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid hash",
			params: `["notahex"]`,
			fail:   true,
		},
	},
	"getstorage": {
		{
			name:   "positive",
			params: `["1a696b32e239dd5eace3f025cac0a193a5746a27", "746573746b6579"]`,
			result: func(e *executor) interface{} { return &StringResultResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*StringResultResponse)
				require.True(t, ok)
				assert.Equal(t, hex.EncodeToString([]byte("testvalue")), res.Result)
			},
		},
		{
			name:   "missing key",
			params: `["1a696b32e239dd5eace3f025cac0a193a5746a27", "7465"]`,
			result: func(e *executor) interface{} { return &StringResultResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*StringResultResponse)
				require.True(t, ok)
				assert.Equal(t, "", res.Result)
			},
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "no second parameter",
			params: `["1a696b32e239dd5eace3f025cac0a193a5746a27"]`,
			fail:   true,
		},
		{
			name:   "invalid hash",
			params: `["notahex"]`,
			fail:   true,
		},
		{
			name:   "invalid key",
			params: `["1a696b32e239dd5eace3f025cac0a193a5746a27", "notahex"]`,
			fail:   true,
		},
	},
	"getassetstate": {
		{
			name:   "positive",
			params: `["602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"]`,
			result: func(e *executor) interface{} { return &GetAssetResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*GetAssetResponse)
				require.True(t, ok)
				assert.Equal(t, "00", res.Result.Owner)
				assert.Equal(t, "AWKECj9RD8rS8RPcpCgYVjk1DeYyHwxZm3", res.Result.Admin)
			},
		},
		{
			name:   "negative",
			params: `["602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de2"]`,
			fail:   true,
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid hash",
			params: `["notahex"]`,
			fail:   true,
		},
	},
	"getbestblockhash": {
		{
			params: "[]",
			result: func(e *executor) interface{} {
				return "0x" + e.chain.CurrentBlockHash().StringLE()
			},
		},
		{
			params: "1",
			fail:   true,
		},
	},
	"gettxout": {
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid hash",
			params: `["notahex"]`,
			fail:   true,
		},
		{
			name:   "missing hash",
			params: `["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 0]`,
			fail:   true,
		},
		{
			name:   "invalid index",
			params: `["7aadf91ca8ac1e2c323c025a7e492bee2dd90c783b86ebfc3b18db66b530a76d", "string"]`,
			fail:   true,
		},
		{
			name:   "negative index",
			params: `["7aadf91ca8ac1e2c323c025a7e492bee2dd90c783b86ebfc3b18db66b530a76d", -1]`,
			fail:   true,
		},
		{
			name:   "too big index",
			params: `["7aadf91ca8ac1e2c323c025a7e492bee2dd90c783b86ebfc3b18db66b530a76d", 100]`,
			fail:   true,
		},
	},
	"getblock": {
		{
			name:   "positive",
			params: "[1, 1]",
			result: func(e *executor) interface{} { return &GetBlockResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*GetBlockResponse)
				require.True(t, ok)

				block, err := e.chain.GetBlock(e.chain.GetHeaderHash(1))
				require.NoErrorf(t, err, "could not get block")

				assert.Equal(t, block.Hash(), res.Result.Hash)
				for i := range res.Result.Tx {
					tx := res.Result.Tx[i]
					require.Equal(t, transaction.MinerType, tx.Type)

					miner, ok := block.Transactions[i].Data.(*transaction.MinerTX)
					require.True(t, ok)
					require.Equal(t, miner.Nonce, tx.Nonce)
					require.Equal(t, block.Transactions[i].Hash(), tx.TxID)
				}
			},
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "bad params",
			params: `[[]]`,
			fail:   true,
		},
		{
			name:   "invalid height",
			params: `[-1]`,
			fail:   true,
		},
		{
			name:   "invalid hash",
			params: `["notahex"]`,
			fail:   true,
		},
		{
			name:   "missing hash",
			params: `["` + util.Uint256{}.String() + `"]`,
			fail:   true,
		},
	},
	"getblockcount": {
		{
			params: "[]",
			result: func(e *executor) interface{} { return int(e.chain.BlockHeight() + 1) },
		},
	},
	"getblockhash": {
		{
			params: "[1]",
			result: func(e *executor) interface{} { return "" },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*StringResultResponse)
				require.True(t, ok)

				block, err := e.chain.GetBlock(e.chain.GetHeaderHash(1))
				require.NoErrorf(t, err, "could not get block")

				expectedHash := "0x" + block.Hash().StringLE()
				assert.Equal(t, expectedHash, res.Result)
			},
		},
		{
			name:   "string height",
			params: `["first"]`,
			fail:   true,
		},
		{
			name:   "invalid number height",
			params: `[-2]`,
			fail:   true,
		},
	},
	"getconnectioncount": {
		{
			params: "[]",
			result: func(*executor) interface{} { return 0 },
		},
	},
	"getpeers": {
		{
			params: "[]",
			result: func(*executor) interface{} {
				return &GetPeersResponse{
					Jsonrpc: defaultJSONRPC,
					Result: struct {
						Unconnected []int `json:"unconnected"`
						Connected   []int `json:"connected"`
						Bad         []int `json:"bad"`
					}{
						Unconnected: []int{},
						Connected:   []int{},
						Bad:         []int{},
					},
					ID: defaultID,
				}
			},
		},
	},
	"getrawtransaction": {
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid hash",
			params: `["notahex"]`,
			fail:   true,
		},
		{
			name:   "missing hash",
			params: `["` + util.Uint256{}.String() + `"]`,
			fail:   true,
		},
	},
	"getunspents": {
		{
			name:   "positive",
			params: `["AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU"]`,
			result: func(e *executor) interface{} { return &GetUnspents{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*GetUnspents)
				require.True(t, ok)
				require.Equal(t, 1, len(res.Result.Balance))
				assert.Equal(t, 1, len(res.Result.Balance[0].Unspents))
			},
		},
		{
			name:   "positive null",
			params: `["AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"]`,
			result: func(e *executor) interface{} { return &GetUnspents{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*GetUnspents)
				require.True(t, ok)
				require.Equal(t, 0, len(res.Result.Balance))
			},
		},
	},
	"getversion": {
		{
			params: "[]",
			result: func(*executor) interface{} { return &GetVersionResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				resp, ok := result.(*GetVersionResponse)
				require.True(t, ok)
				require.Equal(t, "/NEO-GO:/", resp.Result.UserAgent)
			},
		},
	},
	"invoke": {
		{
			name:   "positive",
			params: `["50befd26fdf6e4d957c11e078b24ebce6291456f", [{"type": "String", "value": "qwerty"}]]`,
			result: func(e *executor) interface{} { return &InvokeFunctionResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*InvokeFunctionResponse)
				require.True(t, ok)
				assert.Equal(t, "06717765727479676f459162ceeb248b071ec157d9e4f6fd26fdbe50", res.Result.Script)
				assert.NotEqual(t, "", res.Result.State)
				assert.NotEqual(t, 0, res.Result.GasConsumed)
			},
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "not a string",
			params: `[42, []]`,
			fail:   true,
		},
		{
			name:   "not a scripthash",
			params: `["qwerty", []]`,
			fail:   true,
		},
		{
			name:   "not an array",
			params: `["50befd26fdf6e4d957c11e078b24ebce6291456f", 42]`,
			fail:   true,
		},
		{
			name:   "bad params",
			params: `["50befd26fdf6e4d957c11e078b24ebce6291456f", [{"type": "Integer", "value": "qwerty"}]]`,
			fail:   true,
		},
	},
	"invokefunction": {
		{
			name:   "positive",
			params: `["50befd26fdf6e4d957c11e078b24ebce6291456f", "test", []]`,
			result: func(e *executor) interface{} { return &InvokeFunctionResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*InvokeFunctionResponse)
				require.True(t, ok)
				assert.NotEqual(t, "", res.Result.Script)
				assert.NotEqual(t, "", res.Result.State)
				assert.NotEqual(t, 0, res.Result.GasConsumed)
			},
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "not a string",
			params: `[42, "test", []]`,
			fail:   true,
		},
		{
			name:   "not a scripthash",
			params: `["qwerty", "test", []]`,
			fail:   true,
		},
		{
			name:   "bad params",
			params: `["50befd26fdf6e4d957c11e078b24ebce6291456f", "test", [{"type": "Integer", "value": "qwerty"}]]`,
			fail:   true,
		},
	},
	"invokescript": {
		{
			name:   "positive",
			params: `["51c56b0d48656c6c6f2c20776f726c6421680f4e656f2e52756e74696d652e4c6f67616c7566"]`,
			result: func(e *executor) interface{} { return &InvokeFunctionResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*InvokeFunctionResponse)
				require.True(t, ok)
				assert.NotEqual(t, "", res.Result.Script)
				assert.NotEqual(t, "", res.Result.State)
				assert.NotEqual(t, 0, res.Result.GasConsumed)
			},
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "not a string",
			params: `[42]`,
			fail:   true,
		},
		{
			name:   "bas string",
			params: `["qwerty"]`,
			fail:   true,
		},
	},
	"sendrawtransaction": {
		{
			name:   "positive",
			params: `["d1001b00046e616d6567d3d8602814a429a91afdbaa3914884a1c90c733101201cc9c05cefffe6cdd7b182816a9152ec218d2ec000000141403387ef7940a5764259621e655b3c621a6aafd869a611ad64adcc364d8dd1edf84e00a7f8b11b630a377eaef02791d1c289d711c08b7ad04ff0d6c9caca22cfe6232103cbb45da6072c14761c9da545749d9cfd863f860c351066d16df480602a2024c6ac"]`,
			result: func(e *executor) interface{} { return &SendTXResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*SendTXResponse)
				require.True(t, ok)
				assert.True(t, res.Result)
			},
		},
		{
			name:   "negative",
			params: `["0274d792072617720636f6e7472616374207472616e73616374696f6e206465736372697074696f6e01949354ea0a8b57dfee1e257a1aedd1e0eea2e5837de145e8da9c0f101bfccc8e0100029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500a3e11100000000ea610aa6db39bd8c8556c9569d94b5e5a5d0ad199b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc5004f2418010000001cc9c05cefffe6cdd7b182816a9152ec218d2ec0014140dbd3cddac5cb2bd9bf6d93701f1a6f1c9dbe2d1b480c54628bbb2a4d536158c747a6af82698edf9f8af1cac3850bcb772bd9c8e4ac38f80704751cc4e0bd0e67232103cbb45da6072c14761c9da545749d9cfd863f860c351066d16df480602a2024c6ac"]`,
			fail:   true,
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid string",
			params: `["notahex"]`,
			fail:   true,
		},
		{
			name:   "invalid tx",
			params: `["0274d792072617720636f6e747261637"]`,
			fail:   true,
		},
	},
	"validateaddress": {
		{
			name:   "positive",
			params: `["AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i"]`,
			result: func(*executor) interface{} { return &ValidateAddrResponse{} },
			check: func(t *testing.T, e *executor, result interface{}) {
				res, ok := result.(*ValidateAddrResponse)
				require.True(t, ok)
				assert.Equal(t, "AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i", res.Result.Address)
				assert.True(t, res.Result.IsValid)
			},
		},
		{
			name:   "negative",
			params: "[1]",
			result: func(*executor) interface{} {
				return &ValidateAddrResponse{
					Jsonrpc: defaultJSONRPC,
					Result: result.ValidateAddress{
						Address: float64(1),
						IsValid: false,
					},
					ID: defaultID,
				}
			},
		},
	},
}

func TestRPC(t *testing.T) {
	chain, handler := initServerWithInMemoryChain(t)

	defer chain.Close()

	e := &executor{chain: chain, handler: handler}
	for method, cases := range rpcTestCases {
		t.Run(method, func(t *testing.T) {
			rpc := `{"jsonrpc": "2.0", "id": 1, "method": "%s", "params": %s}`

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					body := doRPCCall(fmt.Sprintf(rpc, method, tc.params), handler, t)
					checkErrResponse(t, body, tc.fail)
					if tc.fail {
						return
					}

					expected, res := tc.getResultPair(e)
					err := json.Unmarshal(body, res)
					require.NoErrorf(t, err, "could not parse response: %s", body)

					if tc.check == nil {
						assert.Equal(t, expected, res)
					} else {
						tc.check(t, e, res)
					}
				})
			}
		})
	}

	t.Run("getrawtransaction", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s"]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res StringResultResponse
		err := json.Unmarshal(body, &res)
		require.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, "400000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000", res.Result)
	})

	t.Run("getrawtransaction 2 arguments", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 0]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)
		var res StringResultResponse
		err := json.Unmarshal(body, &res)
		require.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, "400000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000", res.Result)
	})

	t.Run("gettxout", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		tx := block.Transactions[3]
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "gettxout", "params": [%s, %d]}"`,
			`"`+tx.Hash().StringLE()+`"`, 0)
		body := doRPCCall(rpc, handler, t)
		checkErrResponse(t, body, false)

		var result response.GetTxOut
		err := json.Unmarshal(body, &result)
		require.NoErrorf(t, err, "could not parse response: %s", body)
		assert.Equal(t, 0, result.Result.N)
		assert.Equal(t, "0x9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc5", result.Result.Asset)
		assert.Equal(t, util.Fixed8FromInt64(100000000), result.Result.Value)
		assert.Equal(t, "AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU", result.Result.Address)
	})
}

func (tc rpcTestCase) getResultPair(e *executor) (expected interface{}, res interface{}) {
	expected = tc.result(e)
	switch exp := expected.(type) {
	case string:
		res = new(StringResultResponse)
		expected = &StringResultResponse{
			Jsonrpc: defaultJSONRPC,
			Result:  exp,
			ID:      defaultID,
		}
	case int:
		res = new(IntResultResponse)
		expected = &IntResultResponse{
			Jsonrpc: defaultJSONRPC,
			Result:  exp,
			ID:      defaultID,
		}
	default:
		resVal := reflect.New(reflect.TypeOf(expected).Elem())
		res = resVal.Interface()
	}

	return
}

func checkErrResponse(t *testing.T, body []byte, expectingFail bool) {
	var errresp ErrorResponse
	err := json.Unmarshal(body, &errresp)
	require.Nil(t, err)
	if expectingFail {
		assert.NotEqual(t, 0, errresp.Error.Code)
		assert.NotEqual(t, "", errresp.Error.Message)
	} else {
		assert.Equal(t, 0, errresp.Error.Code)
		assert.Equal(t, "", errresp.Error.Message)
	}
}

func doRPCCall(rpcCall string, handler http.HandlerFunc, t *testing.T) []byte {
	req := httptest.NewRequest("POST", "http://0.0.0.0:20333/", strings.NewReader(rpcCall))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	resp := w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoErrorf(t, err, "could not read response from the request: %s", rpcCall)
	return bytes.TrimSpace(body)
}
