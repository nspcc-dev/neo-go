package server

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type executor struct {
	chain   *core.Blockchain
	httpSrv *httptest.Server
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

const testContractHash = "b0fda4dd46b8e5d207e86e774a4a133c6db69ee7"
const deploymentTxHash = "59f7b22b90e26f883a56b916c1580e3ee4f13caded686353cd77577e6194c173"

const verifyContractHash = "c1213693b22cb0454a436d6e0bd76b8c0a3bfdf7"
const verifyContractAVM = "570300412d51083021700c14aa8acf859d4fe402b34e673f2156821796a488ebdb30716813cedb2869db289740"

var rpcTestCases = map[string][]rpcTestCase{
	"getapplicationlog": {
		{
			name:   "positive",
			params: `["` + deploymentTxHash + `"]`,
			result: func(e *executor) interface{} { return &state.AppExecResult{} },
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*state.AppExecResult)
				require.True(t, ok)
				expectedTxHash, err := util.Uint256DecodeStringLE(deploymentTxHash)
				require.NoError(t, err)
				assert.Equal(t, expectedTxHash, res.TxHash)
				assert.Equal(t, trigger.Application, res.Trigger)
				assert.Equal(t, vm.HaltState, res.VMState)
			},
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid address",
			params: `["notahash"]`,
			fail:   true,
		},
		{
			name:   "invalid tx hash",
			params: `["d24cc1d52b5c0216cbf3835bb5bac8ccf32639fa1ab6627ec4e2b9f33f7ec02f"]`,
			fail:   true,
		},
	},
	"getcontractstate": {
		{
			name:   "positive",
			params: fmt.Sprintf(`["%s"]`, testContractHash),
			result: func(e *executor) interface{} { return &state.Contract{} },
			check: func(t *testing.T, e *executor, cs interface{}) {
				res, ok := cs.(*state.Contract)
				require.True(t, ok)
				assert.Equal(t, testContractHash, res.ScriptHash().StringLE())
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

	"getnep5balances": {
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid address",
			params: `["notahex"]`,
			fail:   true,
		},
		{
			name:   "positive",
			params: `["` + testchain.PrivateKeyByID(0).GetScriptHash().StringLE() + `"]`,
			result: func(e *executor) interface{} { return &result.NEP5Balances{} },
			check:  checkNep5Balances,
		},
		{
			name:   "positive_address",
			params: `["` + address.Uint160ToString(testchain.PrivateKeyByID(0).GetScriptHash()) + `"]`,
			result: func(e *executor) interface{} { return &result.NEP5Balances{} },
			check:  checkNep5Balances,
		},
	},
	"getnep5transfers": {
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid address",
			params: `["notahex"]`,
			fail:   true,
		},
		{
			name:   "invalid timestamp",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", "notanumber"]`,
			fail:   true,
		},
		{
			name:   "invalid stop timestamp",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "blah"]`,
			fail:   true,
		},
		{
			name:   "invalid limit",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "0"]`,
			fail:   true,
		},
		{
			name:   "invalid limit 2",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "bleh"]`,
			fail:   true,
		},
		{
			name:   "invalid limit 3",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "100500"]`,
			fail:   true,
		},
		{
			name:   "invalid page",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "3", "-1"]`,
			fail:   true,
		},
		{
			name:   "invalid page 2",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "3", "jajaja"]`,
			fail:   true,
		},
		{
			name:   "positive",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", 0]`,
			result: func(e *executor) interface{} { return &result.NEP5Transfers{} },
			check:  checkNep5Transfers,
		},
		{
			name:   "positive_hash",
			params: `["` + testchain.PrivateKeyByID(0).GetScriptHash().StringLE() + `", 0]`,
			result: func(e *executor) interface{} { return &result.NEP5Transfers{} },
			check:  checkNep5Transfers,
		},
	},
	"getstorage": {
		{
			name:   "positive",
			params: fmt.Sprintf(`["%s", "746573746b6579"]`, testContractHash),
			result: func(e *executor) interface{} {
				v := hex.EncodeToString([]byte("testvalue"))
				return &v
			},
		},
		{
			name:   "missing key",
			params: fmt.Sprintf(`["%s", "7465"]`, testContractHash),
			result: func(e *executor) interface{} {
				v := ""
				return &v
			},
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "no second parameter",
			params: fmt.Sprintf(`["%s"]`, testContractHash),
			fail:   true,
		},
		{
			name:   "invalid hash",
			params: `["notahex"]`,
			fail:   true,
		},
		{
			name:   "invalid key",
			params: fmt.Sprintf(`["%s", "notahex"]`, testContractHash),
			fail:   true,
		},
	},
	"getbestblockhash": {
		{
			params: "[]",
			result: func(e *executor) interface{} {
				v := "0x" + e.chain.CurrentBlockHash().StringLE()
				return &v
			},
		},
		{
			params: "1",
			fail:   true,
		},
	},
	"getblock": {
		{
			name:   "positive",
			params: "[3, 1]",
			result: func(_ *executor) interface{} { return &result.Block{} },
			check: func(t *testing.T, e *executor, blockRes interface{}) {
				res, ok := blockRes.(*result.Block)
				require.True(t, ok)

				block, err := e.chain.GetBlock(e.chain.GetHeaderHash(3))
				require.NoErrorf(t, err, "could not get block")

				assert.Equal(t, block.Hash(), res.Hash())
				for i, tx := range res.Transactions {
					actualTx := block.Transactions[i]
					require.True(t, ok)
					require.Equal(t, actualTx.Nonce, tx.Nonce)
					require.Equal(t, block.Transactions[i].Hash(), tx.Hash())
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
			result: func(e *executor) interface{} {
				v := int(e.chain.BlockHeight() + 1)
				return &v
			},
		},
	},
	"getblockhash": {
		{
			params: "[1]",
			result: func(e *executor) interface{} {
				// We don't have `t` here for proper handling, but
				// error here would lead to panic down below.
				block, _ := e.chain.GetBlock(e.chain.GetHeaderHash(1))
				expectedHash := "0x" + block.Hash().StringLE()
				return &expectedHash
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
	"getblockheader": {
		{
			name:   "invalid verbose type",
			params: `["9673799c5b5a294427401cb07d6cc615ada3a0d5c5bf7ed6f0f54f24abb2e2ac", true]`,
			fail:   true,
		},
		{
			name:   "invalid block hash",
			params: `["notahash"]`,
			fail:   true,
		},
		{
			name:   "unknown block",
			params: `["a6e526375a780335112299f2262501e5e9574c3ba61b16bbc1e282b344f6c141"]`,
			fail:   true,
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
	},
	"getblocksysfee": {
		{
			name:   "positive",
			params: "[1]",
			result: func(e *executor) interface{} {
				block, _ := e.chain.GetBlock(e.chain.GetHeaderHash(1))

				var expectedBlockSysFee int64
				for _, tx := range block.Transactions {
					expectedBlockSysFee += tx.SystemFee
				}
				return &expectedBlockSysFee
			},
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
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
	"getcommittee": {
		{
			params: "[]",
			result: func(e *executor) interface{} {
				// it's a test chain, so committee is a sorted standby committee
				expected := e.chain.GetStandByCommittee()
				sort.Sort(expected)
				return &expected
			},
		},
	},
	"getconnectioncount": {
		{
			params: "[]",
			result: func(*executor) interface{} {
				v := 0
				return &v
			},
		},
	},
	"getpeers": {
		{
			params: "[]",
			result: func(*executor) interface{} {
				return &result.GetPeers{
					Unconnected: []result.Peer{},
					Connected:   []result.Peer{},
					Bad:         []result.Peer{},
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
	"gettransactionheight": {
		{
			name:   "positive",
			params: `["` + deploymentTxHash + `"]`,
			result: func(e *executor) interface{} {
				h := 0
				return &h
			},
			check: func(t *testing.T, e *executor, resp interface{}) {
				h, ok := resp.(*int)
				require.True(t, ok)
				assert.Equal(t, 2, *h)
			},
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
		{
			name:   "missing hash",
			params: `["` + util.Uint256{}.String() + `"]`,
			fail:   true,
		},
	},
	"getunclaimedgas": {
		{
			name:   "no params",
			params: "[]",
			fail:   true,
		},
		{
			name:   "invalid address",
			params: `["invalid"]`,
			fail:   true,
		},
		{
			name:   "positive",
			params: `["` + testchain.MultisigAddress() + `"]`,
			result: func(*executor) interface{} {
				return &result.UnclaimedGas{}
			},
			check: func(t *testing.T, e *executor, resp interface{}) {
				actual, ok := resp.(*result.UnclaimedGas)
				require.True(t, ok)
				expected := result.UnclaimedGas{
					Address:   testchain.MultisigScriptHash(),
					Unclaimed: *big.NewInt(3500),
				}
				assert.Equal(t, expected, *actual)
			},
		},
	},
	"getnextblockvalidators": {
		{
			params: "[]",
			result: func(*executor) interface{} {
				return &[]result.Validator{}
			},
			/* preview3 doesn't return any validators until there is a vote
			check: func(t *testing.T, e *executor, validators interface{}) {
				var expected []result.Validator
				sBValidators := e.chain.GetStandByValidators()
				for _, sbValidator := range sBValidators {
					expected = append(expected, result.Validator{
						PublicKey: *sbValidator,
						Votes:     0,
						Active:    true,
					})
				}

				actual, ok := validators.(*[]result.Validator)
				require.True(t, ok)

				assert.ElementsMatch(t, expected, *actual)
			},
			*/
		},
	},
	"getversion": {
		{
			params: "[]",
			result: func(*executor) interface{} { return &result.Version{} },
			check: func(t *testing.T, e *executor, ver interface{}) {
				resp, ok := ver.(*result.Version)
				require.True(t, ok)
				require.Equal(t, "/NEO-GO:/", resp.UserAgent)
			},
		},
	},
	"invokefunction": {
		{
			name:   "positive",
			params: `["50befd26fdf6e4d957c11e078b24ebce6291456f", "test", []]`,
			result: func(e *executor) interface{} { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv interface{}) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.NotEqual(t, "", res.Script)
				assert.NotEqual(t, "", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
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
			result: func(e *executor) interface{} { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv interface{}) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.NotEqual(t, "", res.Script)
				assert.NotEqual(t, "", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
			},
		},
		{
			name: "positive, good witness",
			// script is hex-encoded `test_verify.avm` representation, hashes are hex-encoded LE bytes of hashes used in the contract with `0x` prefix
			params: `["5707000c14010c030e05060c0d020e0f0d030e070900000000db307068115541f827ec8c21aa270700000011400c140d0f03020900020103070304050201000e060c09db307169115541f827ec8c21aa270700000012401340",["0x0000000009070e030d0f0e020d0c06050e030c01","0x090c060e00010205040307030102000902030f0d"]]`,
			result: func(e *executor) interface{} { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv interface{}) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				require.Equal(t, 1, len(res.Stack))
				require.Equal(t, big.NewInt(3), res.Stack[0].Value())
			},
		},
		{
			name:   "positive, bad witness of second hash",
			params: `["5707000c14010c030e05060c0d020e0f0d030e070900000000db307068115541f827ec8c21aa270700000011400c140d0f03020900020103070304050201000e060c09db307169115541f827ec8c21aa270700000012401340",["0x0000000009070e030d0f0e020d0c06050e030c01"]]`,
			result: func(e *executor) interface{} { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv interface{}) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				require.Equal(t, 1, len(res.Stack))
				require.Equal(t, big.NewInt(2), res.Stack[0].Value())
			},
		},
		{
			name:   "positive, no good hashes",
			params: `["5707000c14010c030e05060c0d020e0f0d030e070900000000db307068115541f827ec8c21aa270700000011400c140d0f03020900020103070304050201000e060c09db307169115541f827ec8c21aa270700000012401340"]`,
			result: func(e *executor) interface{} { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv interface{}) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				require.Equal(t, 1, len(res.Stack))
				require.Equal(t, big.NewInt(1), res.Stack[0].Value())
			},
		},
		{
			name:   "positive, bad hashes witness",
			params: `["5707000c14010c030e05060c0d020e0f0d030e070900000000db307068115541f827ec8c21aa270700000011400c140d0f03020900020103070304050201000e060c09db307169115541f827ec8c21aa270700000012401340",["0x0000000009070e030d0f0e020d0c06050e030c02"]]`,
			result: func(e *executor) interface{} { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv interface{}) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				assert.Equal(t, 1, len(res.Stack))
				assert.Equal(t, big.NewInt(1), res.Stack[0].Value())
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
			params: `["000b0000008096980000000000261c130000000000b004000001aa8acf859d4fe402b34e673f2156821796a488eb01005d0300e87648170000000c1478ba4c24009fe510e136c9995a2e05215e1be4dc0c14aa8acf859d4fe402b34e673f2156821796a488eb13c00c087472616e736665720c1425059ecb4878d3a875f91c51ceded330d4575fde41627d5b523801420c40ea2f56acf7f64629dc922d65a60176f3963afd4b7c259f2017a3a5139346f8ea54704624590832acb7794069ab2983ddc862b03b6a33d4428cd4c45cbc0941c2290c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b4195440d78"]`,
			result: func(e *executor) interface{} { return &result.RelayResult{} },
			check: func(t *testing.T, e *executor, inv interface{}) {
				res, ok := inv.(*result.RelayResult)
				require.True(t, ok)
				expectedHash, err := util.Uint256DecodeStringLE("ab5573cfc8d70774f04aa7d5521350cfc1aa1239c44c24e490e139408cd46a57")
				require.NoError(t, err)
				assert.Equal(t, expectedHash, res.Hash)
			},
		},
		{
			name:   "negative",
			params: `["000a000000316e851039019d39dfc2c37d6c3fee19fd5809870000000000000000f2ad050000000000b00400000001316e851039019d39dfc2c37d6c3fee19fd580987015d0300e87648170000000c1420728274afafc36f43a071d328cfa3e629d9cbb00c14316e851039019d39dfc2c37d6c3fee19fd58098713c00c087472616e736665720c14897720d8cd76f4f00abfa37c0edd889c208fde9b41627d5b523801420c40df953141271169421cebab5d27a0163e294d7c7f2d0525b4498745344814fd3d6c5c591c9b1723d05d42856f409adb084cf67acc921cfafc629133a5eb5e7a7e290c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906aff"]`,
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
	"submitblock": {
		{
			name:   "invalid hex",
			params: `["000000005gb86f62eafe8e9246bc0d1648e4e5c8389dee9fb7fe03fcc6772ec8c5e4ec2aedb908054ac1409be5f77d5369c6e03490b2f6676d68d0b3370f8159e0fdadf99bc05f5e030000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd0401406f299c82b513f59f5bd120317974852c9694c6e10db1ef2f1bb848b1a33e47a08f8dc03ee784166b2060a94cd4e7af88899b39787938f7f2763ea4d2182776ed40f3bafd85214fef38a4836ca97793001ea411f553c51e88781f7b916c59c145bff28314b6e7ea246789422a996fc4937e290a1b40f6b97c5222540f65b0d47aca40d2b3d19203d456428bfdb529e846285052105957385b65388b9a617f6e2d56a64ec41aa73439eafccb52987bb1975c9b67518b053d9e61b445e4a3377dbc206640bd688489bd62adf6bed9d61a73905b9591eb87053c6f0f4dd70f3bee7295541b490caef044b55b6f9f01dc4a05a756a3f2edd06f5adcbe4e984c1e552f9023f08b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae0100000000000000000000"]`,
			fail:   true,
		},
		{
			name:   "invalid block bytes",
			params: `["0000000027"]`,
			fail:   true,
		},
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
	},
	"validateaddress": {
		{
			name:   "positive",
			params: `["Nbb1qkwcwNSBs9pAnrVVrnFbWnbWBk91U2"]`,
			result: func(*executor) interface{} { return &result.ValidateAddress{} },
			check: func(t *testing.T, e *executor, va interface{}) {
				res, ok := va.(*result.ValidateAddress)
				require.True(t, ok)
				assert.Equal(t, "Nbb1qkwcwNSBs9pAnrVVrnFbWnbWBk91U2", res.Address)
				assert.True(t, res.IsValid)
			},
		},
		{
			name:   "negative",
			params: "[1]",
			result: func(*executor) interface{} {
				return &result.ValidateAddress{
					Address: float64(1),
					IsValid: false,
				}
			},
		},
	},
}

func TestRPC(t *testing.T) {
	t.Run("http", func(t *testing.T) {
		testRPCProtocol(t, doRPCCallOverHTTP)
	})

	t.Run("websocket", func(t *testing.T) {
		testRPCProtocol(t, doRPCCallOverWS)
	})
}

// testRPCProtocol runs a full set of tests using given callback to make actual
// calls. Some tests change the chain state, thus we reinitialize the chain from
// scratch here.
func testRPCProtocol(t *testing.T, doRPCCall func(string, string, *testing.T) []byte) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)

	defer chain.Close()
	defer rpcSrv.Shutdown()

	e := &executor{chain: chain, httpSrv: httpSrv}
	for method, cases := range rpcTestCases {
		t.Run(method, func(t *testing.T) {
			rpc := `{"jsonrpc": "2.0", "id": 1, "method": "%s", "params": %s}`

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					body := doRPCCall(fmt.Sprintf(rpc, method, tc.params), httpSrv.URL, t)
					result := checkErrGetResult(t, body, tc.fail)
					if tc.fail {
						return
					}

					expected, res := tc.getResultPair(e)
					err := json.Unmarshal(result, res)
					require.NoErrorf(t, err, "could not parse response: %s", result)

					if tc.check == nil {
						assert.Equal(t, expected, res)
					} else {
						tc.check(t, e, res)
					}
				})
			}
		})
	}

	t.Run("getapplicationlog for block", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getapplicationlog", "params": ["%s"]}`
		body := doRPCCall(fmt.Sprintf(rpc, e.chain.GetHeaderHash(1).StringLE()), httpSrv.URL, t)
		data := checkErrGetResult(t, body, false)
		var res state.AppExecResult
		require.NoError(t, json.Unmarshal(data, &res))
		require.Equal(t, trigger.System, res.Trigger)
		require.Equal(t, vm.HaltState, res.VMState)
	})

	t.Run("submit", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "submitblock", "params": ["%s"]}`
		t.Run("invalid signature", func(t *testing.T) {
			s := testchain.NewBlock(t, chain, 1, 0)
			s.Script.VerificationScript[8] ^= 0xff
			body := doRPCCall(fmt.Sprintf(rpc, encodeBlock(t, s)), httpSrv.URL, t)
			checkErrGetResult(t, body, true)
		})

		priv0 := testchain.PrivateKeyByID(0)
		acc0, err := wallet.NewAccountFromWIF(priv0.WIF())
		require.NoError(t, err)

		addNetworkFee := func(tx *transaction.Transaction) {
			size := io.GetVarSize(tx)
			netFee, sizeDelta := core.CalculateNetworkFee(acc0.Contract.Script)
			tx.NetworkFee += netFee
			size += sizeDelta
			tx.NetworkFee += int64(size) * chain.FeePerByte()
		}

		newTx := func() *transaction.Transaction {
			height := chain.BlockHeight()
			tx := transaction.New(testchain.Network(), []byte{byte(opcode.PUSH1)}, 0)
			tx.Nonce = height + 1
			tx.ValidUntilBlock = height + 10
			tx.Signers = []transaction.Signer{{Account: acc0.PrivateKey().GetScriptHash()}}
			addNetworkFee(tx)
			require.NoError(t, acc0.SignTx(tx))
			return tx
		}

		t.Run("invalid height", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 2, 0, newTx())
			body := doRPCCall(fmt.Sprintf(rpc, encodeBlock(t, b)), httpSrv.URL, t)
			checkErrGetResult(t, body, true)
		})

		t.Run("positive", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 1, 0, newTx())
			body := doRPCCall(fmt.Sprintf(rpc, encodeBlock(t, b)), httpSrv.URL, t)
			data := checkErrGetResult(t, body, false)
			var res = new(result.RelayResult)
			require.NoError(t, json.Unmarshal(data, res))
			require.Equal(t, b.Hash(), res.Hash)
		})
	})

	t.Run("getrawtransaction", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		tx := block.Transactions[0]
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s"]}"`, tx.Hash().StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		result := checkErrGetResult(t, body, false)
		var res string
		err := json.Unmarshal(result, &res)
		require.NoErrorf(t, err, "could not parse response: %s", result)
		txBin, err := testserdes.EncodeBinary(tx)
		require.NoError(t, err)
		expected := hex.EncodeToString(txBin)
		assert.Equal(t, expected, res)
	})

	t.Run("getrawtransaction 2 arguments", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		tx := block.Transactions[0]
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 0]}"`, tx.Hash().StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		result := checkErrGetResult(t, body, false)
		var res string
		err := json.Unmarshal(result, &res)
		require.NoErrorf(t, err, "could not parse response: %s", result)
		txBin, err := testserdes.EncodeBinary(tx)
		require.NoError(t, err)
		expected := hex.EncodeToString(txBin)
		assert.Equal(t, expected, res)
	})

	t.Run("getrawtransaction 2 arguments, verbose", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[0].Hash()
		_ = block.Transactions[0].Size()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 1]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		txOut := checkErrGetResult(t, body, false)
		actual := result.TransactionOutputRaw{Transaction: transaction.Transaction{Network: testchain.Network()}}
		err := json.Unmarshal(txOut, &actual)
		require.NoErrorf(t, err, "could not parse response: %s", txOut)

		assert.Equal(t, *block.Transactions[0], actual.Transaction)
		assert.Equal(t, 9, actual.Confirmations)
		assert.Equal(t, TXHash, actual.Transaction.Hash())
	})

	t.Run("getblockheader_positive", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getblockheader", "params": %s}`
		testHeaderHash := chain.GetHeaderHash(1).StringLE()
		hdr := e.getHeader(testHeaderHash)

		runCase := func(t *testing.T, rpc string, expected, actual interface{}) {
			body := doRPCCall(rpc, httpSrv.URL, t)
			data := checkErrGetResult(t, body, false)
			require.NoError(t, json.Unmarshal(data, actual))
			require.Equal(t, expected, actual)
		}

		t.Run("no verbose", func(t *testing.T) {
			w := io.NewBufBinWriter()
			hdr.EncodeBinary(w.BinWriter)
			require.NoError(t, w.Err)
			encoded := hex.EncodeToString(w.Bytes())

			t.Run("missing", func(t *testing.T) {
				runCase(t, fmt.Sprintf(rpc, `["`+testHeaderHash+`"]`), &encoded, new(string))
			})

			t.Run("verbose=0", func(t *testing.T) {
				runCase(t, fmt.Sprintf(rpc, `["`+testHeaderHash+`", 0]`), &encoded, new(string))
			})

			t.Run("by number", func(t *testing.T) {
				runCase(t, fmt.Sprintf(rpc, `[1]`), &encoded, new(string))
			})
		})

		t.Run("verbose != 0", func(t *testing.T) {
			nextHash := chain.GetHeaderHash(int(hdr.Index) + 1)
			expected := &result.Header{
				Hash:          hdr.Hash(),
				Size:          io.GetVarSize(hdr),
				Version:       hdr.Version,
				PrevBlockHash: hdr.PrevHash,
				MerkleRoot:    hdr.MerkleRoot,
				Timestamp:     hdr.Timestamp,
				Index:         hdr.Index,
				NextConsensus: address.Uint160ToString(hdr.NextConsensus),
				Witnesses:     []transaction.Witness{hdr.Script},
				Confirmations: e.chain.BlockHeight() - hdr.Index + 1,
				NextBlockHash: &nextHash,
			}

			rpc := fmt.Sprintf(rpc, `["`+testHeaderHash+`", 2]`)
			runCase(t, rpc, expected, new(result.Header))
		})
	})

	t.Run("getrawmempool", func(t *testing.T) {
		mp := chain.GetMemPool()
		// `expected` stores hashes of previously added txs
		expected := make([]util.Uint256, 0)
		for _, tx := range mp.GetVerifiedTransactions() {
			expected = append(expected, tx.Hash())
		}
		for i := 0; i < 5; i++ {
			tx := transaction.New(testchain.Network(), []byte{byte(opcode.PUSH1)}, 0)
			tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
			assert.NoError(t, mp.Add(tx, &FeerStub{}))
			expected = append(expected, tx.Hash())
		}

		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getrawmempool", "params": []}`
		body := doRPCCall(rpc, httpSrv.URL, t)
		res := checkErrGetResult(t, body, false)

		var actual []util.Uint256
		err := json.Unmarshal(res, &actual)
		require.NoErrorf(t, err, "could not parse response: %s", res)

		assert.ElementsMatch(t, expected, actual)
	})

	t.Run("getnep5transfers", func(t *testing.T) {
		testNEP5T := func(t *testing.T, start, stop, limit, page int, sent, rcvd []int) {
			ps := []string{`"` + testchain.PrivateKeyByID(0).Address() + `"`}
			if start != 0 {
				h, err := e.chain.GetHeader(e.chain.GetHeaderHash(start))
				var ts uint64
				if err == nil {
					ts = h.Timestamp
				} else {
					ts = uint64(time.Now().UnixNano() / 1_000_000)
				}
				ps = append(ps, strconv.FormatUint(ts, 10))
			}
			if stop != 0 {
				h, err := e.chain.GetHeader(e.chain.GetHeaderHash(stop))
				var ts uint64
				if err == nil {
					ts = h.Timestamp
				} else {
					ts = uint64(time.Now().UnixNano() / 1_000_000)
				}
				ps = append(ps, strconv.FormatUint(ts, 10))
			}
			if limit != 0 {
				ps = append(ps, strconv.FormatInt(int64(limit), 10))
			}
			if page != 0 {
				ps = append(ps, strconv.FormatInt(int64(page), 10))
			}
			p := strings.Join(ps, ", ")
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getnep5transfers", "params": [%s]}`, p)
			body := doRPCCall(rpc, httpSrv.URL, t)
			res := checkErrGetResult(t, body, false)
			actual := new(result.NEP5Transfers)
			require.NoError(t, json.Unmarshal(res, actual))
			checkNep5TransfersAux(t, e, actual, sent, rcvd)
		}
		t.Run("time frame only", func(t *testing.T) { testNEP5T(t, 4, 5, 0, 0, []int{3, 4, 5, 6}, []int{1, 2}) })
		t.Run("no res", func(t *testing.T) { testNEP5T(t, 100, 100, 0, 0, []int{}, []int{}) })
		t.Run("limit", func(t *testing.T) { testNEP5T(t, 1, 7, 3, 0, []int{0, 1}, []int{0}) })
		t.Run("limit 2", func(t *testing.T) { testNEP5T(t, 4, 5, 2, 0, []int{3}, []int{1}) })
		t.Run("limit with page", func(t *testing.T) { testNEP5T(t, 1, 7, 3, 1, []int{2, 3}, []int{1}) })
		t.Run("limit with page 2", func(t *testing.T) { testNEP5T(t, 1, 7, 3, 2, []int{4, 5}, []int{2}) })
	})
}

func (e *executor) getHeader(s string) *block.Header {
	hash, err := util.Uint256DecodeStringLE(s)
	if err != nil {
		panic("can not decode hash parameter")
	}
	block, err := e.chain.GetBlock(hash)
	if err != nil {
		panic("unknown block (update block hash)")
	}
	return block.Header()
}

func encodeBlock(t *testing.T, b *block.Block) string {
	w := io.NewBufBinWriter()
	b.EncodeBinary(w.BinWriter)
	require.NoError(t, w.Err)
	return hex.EncodeToString(w.Bytes())
}

func (tc rpcTestCase) getResultPair(e *executor) (expected interface{}, res interface{}) {
	expected = tc.result(e)
	resVal := reflect.New(reflect.TypeOf(expected).Elem())
	res = resVal.Interface()
	switch r := res.(type) {
	case *result.Block:
		r.Network = testchain.Network()
	}
	return expected, res
}

func checkErrGetResult(t *testing.T, body []byte, expectingFail bool) json.RawMessage {
	var resp response.Raw
	err := json.Unmarshal(body, &resp)
	require.Nil(t, err)
	if expectingFail {
		require.NotNil(t, resp.Error)
		assert.NotEqual(t, 0, resp.Error.Code)
		assert.NotEqual(t, "", resp.Error.Message)
	} else {
		assert.Nil(t, resp.Error)
	}
	return resp.Result
}

func doRPCCallOverWS(rpcCall string, url string, t *testing.T) []byte {
	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	url = "ws" + strings.TrimPrefix(url, "http")
	c, _, err := dialer.Dial(url+"/ws", nil)
	require.NoError(t, err)
	c.SetWriteDeadline(time.Now().Add(time.Second))
	require.NoError(t, c.WriteMessage(1, []byte(rpcCall)))
	c.SetReadDeadline(time.Now().Add(time.Second))
	_, body, err := c.ReadMessage()
	require.NoError(t, err)
	require.NoError(t, c.Close())
	return bytes.TrimSpace(body)
}

func doRPCCallOverHTTP(rpcCall string, url string, t *testing.T) []byte {
	cl := http.Client{Timeout: time.Second}
	resp, err := cl.Post(url, "application/json", strings.NewReader(rpcCall))
	require.NoErrorf(t, err, "could not make a POST request")
	body, err := ioutil.ReadAll(resp.Body)
	assert.NoErrorf(t, err, "could not read response from the request: %s", rpcCall)
	return bytes.TrimSpace(body)
}

func checkNep5Balances(t *testing.T, e *executor, acc interface{}) {
	res, ok := acc.(*result.NEP5Balances)
	require.True(t, ok)
	rubles, err := util.Uint160DecodeStringLE(testContractHash)
	require.NoError(t, err)
	expected := result.NEP5Balances{
		Balances: []result.NEP5Balance{
			{
				Asset:       rubles,
				Amount:      "8.77",
				LastUpdated: 6,
			},
			{
				Asset:       e.chain.GoverningTokenHash(),
				Amount:      "99998000",
				LastUpdated: 4,
			},
			{
				Asset:       e.chain.UtilityTokenHash(),
				Amount:      "799.59641770",
				LastUpdated: 7,
			}},
		Address: testchain.PrivateKeyByID(0).GetScriptHash().StringLE(),
	}
	require.Equal(t, testchain.PrivateKeyByID(0).Address(), res.Address)
	require.ElementsMatch(t, expected.Balances, res.Balances)
}

func checkNep5Transfers(t *testing.T, e *executor, acc interface{}) {
	checkNep5TransfersAux(t, e, acc, []int{0, 1, 2, 3, 4, 5, 6, 7, 8}, []int{0, 1, 2, 3, 4, 5, 6})
}

func checkNep5TransfersAux(t *testing.T, e *executor, acc interface{}, sent, rcvd []int) {
	res, ok := acc.(*result.NEP5Transfers)
	require.True(t, ok)
	rublesHash, err := util.Uint160DecodeStringLE(testContractHash)
	require.NoError(t, err)

	blockDeploy2, err := e.chain.GetBlock(e.chain.GetHeaderHash(7))
	require.NoError(t, err)
	require.Equal(t, 1, len(blockDeploy2.Transactions))
	txDeploy2 := blockDeploy2.Transactions[0]

	blockSendRubles, err := e.chain.GetBlock(e.chain.GetHeaderHash(6))
	require.NoError(t, err)
	require.Equal(t, 1, len(blockSendRubles.Transactions))
	txSendRubles := blockSendRubles.Transactions[0]
	blockGASBounty := blockSendRubles // index 6 = size of committee

	blockReceiveRubles, err := e.chain.GetBlock(e.chain.GetHeaderHash(5))
	require.NoError(t, err)
	require.Equal(t, 2, len(blockReceiveRubles.Transactions))
	txInitCall := blockReceiveRubles.Transactions[0]
	txReceiveRubles := blockReceiveRubles.Transactions[1]

	blockSendNEO, err := e.chain.GetBlock(e.chain.GetHeaderHash(4))
	require.NoError(t, err)
	require.Equal(t, 1, len(blockSendNEO.Transactions))
	txSendNEO := blockSendNEO.Transactions[0]

	blockCtrInv1, err := e.chain.GetBlock(e.chain.GetHeaderHash(3))
	require.NoError(t, err)
	require.Equal(t, 1, len(blockCtrInv1.Transactions))
	txCtrInv1 := blockCtrInv1.Transactions[0]

	blockCtrDeploy, err := e.chain.GetBlock(e.chain.GetHeaderHash(2))
	require.NoError(t, err)
	require.Equal(t, 1, len(blockCtrDeploy.Transactions))
	txCtrDeploy := blockCtrDeploy.Transactions[0]

	blockReceiveGAS, err := e.chain.GetBlock(e.chain.GetHeaderHash(1))
	require.NoError(t, err)
	require.Equal(t, 2, len(blockReceiveGAS.Transactions))
	txReceiveNEO := blockReceiveGAS.Transactions[0]
	txReceiveGAS := blockReceiveGAS.Transactions[1]

	blockGASBounty0, err := e.chain.GetBlock(e.chain.GetHeaderHash(0))
	require.NoError(t, err)

	// These are laid out here explicitly for 2 purposes:
	//  * to be able to reference any particular event for paging
	//  * to check chain events consistency
	// Technically these could be retrieved from application log, but that would almost
	// duplicate the Server method.
	expected := result.NEP5Transfers{
		Sent: []result.NEP5Transfer{
			{
				Timestamp: blockDeploy2.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    amountToString(big.NewInt(txDeploy2.SystemFee+txDeploy2.NetworkFee), 8),
				Index:     7,
				TxHash:    blockDeploy2.Hash(),
			},
			{
				Timestamp:   blockSendRubles.Timestamp,
				Asset:       rublesHash,
				Address:     testchain.PrivateKeyByID(1).Address(),
				Amount:      "1.23",
				Index:       6,
				NotifyIndex: 0,
				TxHash:      txSendRubles.Hash(),
			},
			{
				Timestamp: blockSendRubles.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    amountToString(big.NewInt(txSendRubles.SystemFee+txSendRubles.NetworkFee), 8),
				Index:     6,
				TxHash:    blockSendRubles.Hash(),
			},
			{
				Timestamp: blockReceiveRubles.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    amountToString(big.NewInt(txReceiveRubles.SystemFee+txReceiveRubles.NetworkFee), 8),
				Index:     5,
				TxHash:    blockReceiveRubles.Hash(),
			},
			{
				Timestamp: blockReceiveRubles.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    amountToString(big.NewInt(txInitCall.SystemFee+txInitCall.NetworkFee), 8),
				Index:     5,
				TxHash:    blockReceiveRubles.Hash(),
			},
			{
				Timestamp:   blockSendNEO.Timestamp,
				Asset:       e.chain.GoverningTokenHash(),
				Address:     testchain.PrivateKeyByID(1).Address(),
				Amount:      "1000",
				Index:       4,
				NotifyIndex: 0,
				TxHash:      txSendNEO.Hash(),
			},
			{
				Timestamp: blockSendNEO.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    amountToString(big.NewInt(txSendNEO.SystemFee+txSendNEO.NetworkFee), 8),
				Index:     4,
				TxHash:    blockSendNEO.Hash(),
			},
			{
				Timestamp: blockCtrInv1.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn has empty receiver
				Amount:    amountToString(big.NewInt(txCtrInv1.SystemFee+txCtrInv1.NetworkFee), 8),
				Index:     3,
				TxHash:    blockCtrInv1.Hash(),
			},
			{
				Timestamp: blockCtrDeploy.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn has empty receiver
				Amount:    amountToString(big.NewInt(txCtrDeploy.SystemFee+txCtrDeploy.NetworkFee), 8),
				Index:     2,
				TxHash:    blockCtrDeploy.Hash(),
			},
		},
		Received: []result.NEP5Transfer{
			{
				Timestamp:   blockGASBounty.Timestamp,
				Asset:       e.chain.UtilityTokenHash(),
				Address:     "",
				Amount:      "0.25000000",
				Index:       6,
				NotifyIndex: 0,
				TxHash:      blockGASBounty.Hash(),
			},
			{
				Timestamp:   blockReceiveRubles.Timestamp,
				Asset:       rublesHash,
				Address:     address.Uint160ToString(rublesHash),
				Amount:      "10",
				Index:       5,
				NotifyIndex: 0,
				TxHash:      txReceiveRubles.Hash(),
			},
			{
				Timestamp:   blockSendNEO.Timestamp,
				Asset:       e.chain.UtilityTokenHash(),
				Address:     "", // Minted GAS.
				Amount:      "1.49998500",
				Index:       4,
				NotifyIndex: 0,
				TxHash:      txSendNEO.Hash(),
			},
			{
				Timestamp:   blockReceiveGAS.Timestamp,
				Asset:       e.chain.UtilityTokenHash(),
				Address:     testchain.MultisigAddress(),
				Amount:      "1000",
				Index:       1,
				NotifyIndex: 0,
				TxHash:      txReceiveGAS.Hash(),
			},
			{
				Timestamp:   blockReceiveGAS.Timestamp,
				Asset:       e.chain.GoverningTokenHash(),
				Address:     testchain.MultisigAddress(),
				Amount:      "99999000",
				Index:       1,
				NotifyIndex: 0,
				TxHash:      txReceiveNEO.Hash(),
			},
			{
				Timestamp: blockGASBounty0.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "",
				Amount:    "0.25000000",
				Index:     0,
				TxHash:    blockGASBounty0.Hash(),
			},
		},
		Address: testchain.PrivateKeyByID(0).Address(),
	}

	require.Equal(t, expected.Address, res.Address)

	arr := make([]result.NEP5Transfer, 0, len(expected.Sent))
	for i := range expected.Sent {
		for _, j := range sent {
			if i == j {
				arr = append(arr, expected.Sent[i])
				break
			}
		}
	}
	require.Equal(t, arr, res.Sent)

	arr = arr[:0]
	for i := range expected.Received {
		for _, j := range rcvd {
			if i == j {
				arr = append(arr, expected.Received[i])
				break
			}
		}
	}
	require.Equal(t, arr, res.Received)
}
