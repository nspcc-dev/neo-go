package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rpcClientTestCase struct {
	name           string
	invoke         func(c *Client) (interface{}, error)
	serverResponse string
	result         func(c *Client) interface{}
	check          func(t *testing.T, c *Client, result interface{})
}

// getResultBlock202 returns data for block number 1 which is used by several tests.
func getResultBlock202() *result.Block {
	nextBlockHash, err := util.Uint256DecodeStringLE("13283c93aec07dc90be3ddd65e2de15e9212f1b3205303f688d6df85129f6b22")
	if err != nil {
		panic(err)
	}
	prevBlockHash, err := util.Uint256DecodeStringLE("93b540424c1173e487a47582a652686b2885f959ffd895b30e184842403990ef")
	if err != nil {
		panic(err)
	}
	merkleRoot, err := util.Uint256DecodeStringLE("b8e923148dede20901d6fb225579f6d430cc58f24461d1b0f860ee32abbfcc8d")
	if err != nil {
		panic(err)
	}
	invScript, err := hex.DecodeString("0c403620ef8f02d7884c553fb6c54d2fe717cfddd9450886c5fc88a669a29a82fa1a7c715076996567a5a56747f20f10d7e4db071d73b306ccbf17f9a916fcfa1d020c4099e27d87bbb3fb4ce1c77dca85cf3eac46c9c3de87d8022ef7ad2b0a2bb980339293849cf85e5a0a5615ea7bc5bb0a7f28e31f278dc19d628f64c49b888df4c60c40616eefc9286843c2f3f2cf1815988356e409b3f10ffaf60b3468dc0a92dd929cbc8d5da74052c303e7474412f6beaddd551e9056c4e7a5fccdc06107e48f3fe10c40fd2d25d4156e969345c0522669b509e5ced70e4265066eadaf85eea3919d5ded525f8f52d6f0dfa0186c964dd0302fca5bc2dc0540b4ed21085be478c3123996")
	if err != nil {
		panic(err)
	}
	verifScript, err := hex.DecodeString("130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb")
	if err != nil {
		panic(err)
	}
	sender, err := address.StringToUint160("ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG")
	if err != nil {
		panic(err)
	}
	txInvScript, err := hex.DecodeString("0c402caebbee911a1f159aa05ab40093d086090a817e837f3f87e8b3e47f6b083649137770f6dda0349ddd611bc47402aca457a89b3b7b0076307ab6a47fd57048eb")
	if err != nil {
		panic(err)
	}
	txVerifScript, err := hex.DecodeString("0c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4")
	if err != nil {
		panic(err)
	}
	vin, err := util.Uint256DecodeStringLE("33e045101301854a0e07ff96a92ca1ba9b23c19501f1b7eb15ae9eea07b5f370")
	if err != nil {
		panic(err)
	}
	outAddress, err := address.StringToUint160("ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG")
	if err != nil {
		panic(err)
	}
	tx := transaction.NewContractTX()
	tx.Nonce = 3
	tx.ValidUntilBlock = 1200
	tx.Sender = sender
	tx.Scripts = []transaction.Witness{
		{
			InvocationScript:   txInvScript,
			VerificationScript: txVerifScript,
		},
	}
	tx.Inputs = []transaction.Input{
		{
			PrevHash:  vin,
			PrevIndex: 0,
		},
	}
	tx.Outputs = []transaction.Output{
		{
			AssetID:    core.GoverningTokenID(),
			Amount:     util.Fixed8FromInt64(99999000),
			ScriptHash: outAddress,
			Position:   0,
		},
	}

	var nonce uint64
	i, err := fmt.Sscanf("0000000000000457", "%016x", &nonce)
	if i != 1 {
		panic("can't decode nonce")
	}
	if err != nil {
		panic(err)
	}
	nextCon, err := address.StringToUint160("AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL")
	if err != nil {
		panic(err)
	}
	blck := &block.Block{
		Base: block.Base{
			Version:       0,
			PrevHash:      prevBlockHash,
			MerkleRoot:    merkleRoot,
			Timestamp:     1589300496,
			Index:         202,
			NextConsensus: nextCon,
			Script: transaction.Witness{
				InvocationScript:   invScript,
				VerificationScript: verifScript,
			},
		},
		ConsensusData: block.ConsensusData{
			PrimaryIndex: 0,
			Nonce:        nonce,
		},
		Transactions: []*transaction.Transaction{tx},
	}
	// Update hashes for correct result comparison.
	_ = tx.Hash()
	_ = blck.Hash()
	return &result.Block{
		Block: blck,
		BlockMetadata: result.BlockMetadata{
			Size:          781,
			Confirmations: 6,
			NextBlockHash: &nextBlockHash,
		},
	}
}

// rpcClientTestCases contains `serverResponse` json data fetched from examples
// published in official C# JSON-RPC API v2.10.3 reference
// (see https://docs.neo.org/docs/en-us/reference/rpc/latest-version/api.html)
var rpcClientTestCases = map[string][]rpcClientTestCase{
	"getaccountstate": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetAccountState("")
			},
			serverResponse: `{"jsonrpc":"2.0","id": 1,"result":{"version":0,"script_hash":"0x1179716da2e9523d153a35fb3ad10c561b1e5b1a","frozen":false,"votes":[],"balances":[{"asset":"0x1a5e0e3eac2abced7de9ee2de0820a5c85e63756fcdfc29b82fead86a7c07c78","value":"94"}]}}`,
			result: func(c *Client) interface{} {
				scriptHash, err := util.Uint160DecodeStringLE("1179716da2e9523d153a35fb3ad10c561b1e5b1a")
				if err != nil {
					panic(err)
				}
				return &result.AccountState{
					Version:    0,
					ScriptHash: scriptHash,
					IsFrozen:   false,
					Balances: result.Balances{
						result.Balance{
							Asset: core.GoverningTokenID(),
							Value: util.Fixed8FromInt64(94),
						},
					},
				}
			},
		},
	},
	"getapplicationlog": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{})
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"txid":"0x17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521","executions":[{"trigger":"Application","contract":"0xb9fa3b421eb749d5dd585fe1c1133b311a14bcb1","vmstate":"HALT","gas_consumed":"1","stack":[{"type":"Integer","value":1}],"notifications":[]}]}}`,
			result: func(c *Client) interface{} {
				txHash, err := util.Uint256DecodeStringLE("17145a039fca704fcdbeb46e6b210af98a1a9e5b9768e46ffc38f71c79ac2521")
				if err != nil {
					panic(err)
				}
				scriptHash, err := util.Uint160DecodeStringLE("b9fa3b421eb749d5dd585fe1c1133b311a14bcb1")
				if err != nil {
					panic(err)
				}
				return &result.ApplicationLog{
					TxHash: txHash,
					Executions: []result.Execution{
						{
							Trigger:     "Application",
							ScriptHash:  scriptHash,
							VMState:     "HALT",
							GasConsumed: util.Fixed8FromInt64(1),
							Stack:       []smartcontract.Parameter{{Type: smartcontract.IntegerType, Value: int64(1)}},
							Events:      []result.NotificationEvent{},
						},
					},
				}
			},
		},
	},
	"getbestblockhash": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"0x773dd2dae4a9c9275290f89b56e67d7363ea4826dfd4fc13cc01cf73a44b0d0e"}`,
			result: func(c *Client) interface{} {
				result, err := util.Uint256DecodeStringLE("773dd2dae4a9c9275290f89b56e67d7363ea4826dfd4fc13cc01cf73a44b0d0e")
				if err != nil {
					panic(err)
				}
				return result
			},
		},
	},
	"getblock": {
		{
			name: "byIndex_positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(202)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"00000000ef9039404248180eb395d8ff59f985286b6852a68275a487e473114c4240b5938dccbfab32ee60f8b0d16144f258cc30d4f6795522fbd60109e2ed8d1423e9b810cdba5e00000000ca000000abec5362f11e75b6e02e407bb98d63675d14384101fd08010c403620ef8f02d7884c553fb6c54d2fe717cfddd9450886c5fc88a669a29a82fa1a7c715076996567a5a56747f20f10d7e4db071d73b306ccbf17f9a916fcfa1d020c4099e27d87bbb3fb4ce1c77dca85cf3eac46c9c3de87d8022ef7ad2b0a2bb980339293849cf85e5a0a5615ea7bc5bb0a7f28e31f278dc19d628f64c49b888df4c60c40616eefc9286843c2f3f2cf1815988356e409b3f10ffaf60b3468dc0a92dd929cbc8d5da74052c303e7474412f6beaddd551e9056c4e7a5fccdc06107e48f3fe10c40fd2d25d4156e969345c0522669b509e5ced70e4265066eadaf85eea3919d5ded525f8f52d6f0dfa0186c964dd0302fca5bc2dc0540b4ed21085be478c312399694130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb02005704000000000000800003000000316e851039019d39dfc2c37d6c3fee19fd58098700000000000000000000000000000000b004000000000170f3b507ea9eae15ebb7f10195c1239bbaa12ca996ff070e4a8501131045e033000001787cc0a786adfe829bc2dffc5637e6855c0a82e02deee97dedbc2aac3e0e5e1a00184a27db862300316e851039019d39dfc2c37d6c3fee19fd58098701420c402caebbee911a1f159aa05ab40093d086090a817e837f3f87e8b3e47f6b083649137770f6dda0349ddd611bc47402aca457a89b3b7b0076307ab6a47fd57048eb290c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4"}`,
			result:         func(c *Client) interface{} { return &block.Block{} },
			check: func(t *testing.T, c *Client, result interface{}) {
				res, ok := result.(*block.Block)
				require.True(t, ok)
				assert.Equal(t, uint32(0), res.Version)
				assert.Equal(t, "cbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86", res.Hash().StringLE())
				assert.Equal(t, "93b540424c1173e487a47582a652686b2885f959ffd895b30e184842403990ef", res.PrevHash.StringLE())
				assert.Equal(t, "b8e923148dede20901d6fb225579f6d430cc58f24461d1b0f860ee32abbfcc8d", res.MerkleRoot.StringLE())
				assert.Equal(t, 1, len(res.Transactions))
				assert.Equal(t, "96ef00d2efe03101f5b302f7fc3c8fcd22688944bdc83ab99071d77edbb08581", res.Transactions[0].Hash().StringLE())
			},
		},
		{
			name: "byIndex_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				return c.GetBlockByIndexVerbose(202)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"hash":"0xcbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86","size":781,"version":0,"nextblockhash":"0x13283c93aec07dc90be3ddd65e2de15e9212f1b3205303f688d6df85129f6b22","previousblockhash":"0x93b540424c1173e487a47582a652686b2885f959ffd895b30e184842403990ef","merkleroot":"0xb8e923148dede20901d6fb225579f6d430cc58f24461d1b0f860ee32abbfcc8d","time":1589300496,"index":202,"consensus_data":{"primary":0,"nonce":"0000000000000457"},"nextconsensus":"AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL","confirmations":6,"witnesses":[{"invocation":"0c403620ef8f02d7884c553fb6c54d2fe717cfddd9450886c5fc88a669a29a82fa1a7c715076996567a5a56747f20f10d7e4db071d73b306ccbf17f9a916fcfa1d020c4099e27d87bbb3fb4ce1c77dca85cf3eac46c9c3de87d8022ef7ad2b0a2bb980339293849cf85e5a0a5615ea7bc5bb0a7f28e31f278dc19d628f64c49b888df4c60c40616eefc9286843c2f3f2cf1815988356e409b3f10ffaf60b3468dc0a92dd929cbc8d5da74052c303e7474412f6beaddd551e9056c4e7a5fccdc06107e48f3fe10c40fd2d25d4156e969345c0522669b509e5ced70e4265066eadaf85eea3919d5ded525f8f52d6f0dfa0186c964dd0302fca5bc2dc0540b4ed21085be478c3123996","verification":"130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb"}],"tx":[{"txid":"0x96ef00d2efe03101f5b302f7fc3c8fcd22688944bdc83ab99071d77edbb08581","size":254,"type":"ContractTransaction","version":0,"nonce":3,"sender":"ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG","sys_fee":"0","net_fee":"0","valid_until_block":1200,"attributes":[],"cosigners":[],"vin":[{"txid":"0x33e045101301854a0e07ff96a92ca1ba9b23c19501f1b7eb15ae9eea07b5f370","vout":0}],"vout":[{"address":"ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG","asset":"0x1a5e0e3eac2abced7de9ee2de0820a5c85e63756fcdfc29b82fead86a7c07c78","n":0,"value":"99999000"}],"scripts":[{"invocation":"0c402caebbee911a1f159aa05ab40093d086090a817e837f3f87e8b3e47f6b083649137770f6dda0349ddd611bc47402aca457a89b3b7b0076307ab6a47fd57048eb","verification":"0c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4"}]}]}}`,
			result: func(c *Client) interface{} {
				return getResultBlock202()
			},
		},
		{
			name: "byHash_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("86fe1061140b2ea791b0739fb9732abc6e5e47de4927228a1ac41de3d93eb7cb")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHash(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"00000000ef9039404248180eb395d8ff59f985286b6852a68275a487e473114c4240b5938dccbfab32ee60f8b0d16144f258cc30d4f6795522fbd60109e2ed8d1423e9b810cdba5e00000000ca000000abec5362f11e75b6e02e407bb98d63675d14384101fd08010c403620ef8f02d7884c553fb6c54d2fe717cfddd9450886c5fc88a669a29a82fa1a7c715076996567a5a56747f20f10d7e4db071d73b306ccbf17f9a916fcfa1d020c4099e27d87bbb3fb4ce1c77dca85cf3eac46c9c3de87d8022ef7ad2b0a2bb980339293849cf85e5a0a5615ea7bc5bb0a7f28e31f278dc19d628f64c49b888df4c60c40616eefc9286843c2f3f2cf1815988356e409b3f10ffaf60b3468dc0a92dd929cbc8d5da74052c303e7474412f6beaddd551e9056c4e7a5fccdc06107e48f3fe10c40fd2d25d4156e969345c0522669b509e5ced70e4265066eadaf85eea3919d5ded525f8f52d6f0dfa0186c964dd0302fca5bc2dc0540b4ed21085be478c312399694130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb02005704000000000000800003000000316e851039019d39dfc2c37d6c3fee19fd58098700000000000000000000000000000000b004000000000170f3b507ea9eae15ebb7f10195c1239bbaa12ca996ff070e4a8501131045e033000001787cc0a786adfe829bc2dffc5637e6855c0a82e02deee97dedbc2aac3e0e5e1a00184a27db862300316e851039019d39dfc2c37d6c3fee19fd58098701420c402caebbee911a1f159aa05ab40093d086090a817e837f3f87e8b3e47f6b083649137770f6dda0349ddd611bc47402aca457a89b3b7b0076307ab6a47fd57048eb290c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4"}`,
			result:         func(c *Client) interface{} { return &block.Block{} },
			check: func(t *testing.T, c *Client, result interface{}) {
				res, ok := result.(*block.Block)
				require.True(t, ok)
				assert.Equal(t, uint32(0), res.Version)
				assert.Equal(t, "cbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86", res.Hash().StringLE())
				assert.Equal(t, "93b540424c1173e487a47582a652686b2885f959ffd895b30e184842403990ef", res.PrevHash.StringLE())
				assert.Equal(t, "b8e923148dede20901d6fb225579f6d430cc58f24461d1b0f860ee32abbfcc8d", res.MerkleRoot.StringLE())
				assert.Equal(t, 1, len(res.Transactions))
				assert.Equal(t, "96ef00d2efe03101f5b302f7fc3c8fcd22688944bdc83ab99071d77edbb08581", res.Transactions[0].Hash().StringLE())
			},
		},
		{
			name: "byHash_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("86fe1061140b2ea791b0739fb9732abc6e5e47de4927228a1ac41de3d93eb7cb")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHashVerbose(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"hash":"0xcbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86","size":781,"version":0,"nextblockhash":"0x13283c93aec07dc90be3ddd65e2de15e9212f1b3205303f688d6df85129f6b22","previousblockhash":"0x93b540424c1173e487a47582a652686b2885f959ffd895b30e184842403990ef","merkleroot":"0xb8e923148dede20901d6fb225579f6d430cc58f24461d1b0f860ee32abbfcc8d","time":1589300496,"index":202,"consensus_data":{"primary":0,"nonce":"0000000000000457"},"nextconsensus":"AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL","confirmations":6,"witnesses":[{"invocation":"0c403620ef8f02d7884c553fb6c54d2fe717cfddd9450886c5fc88a669a29a82fa1a7c715076996567a5a56747f20f10d7e4db071d73b306ccbf17f9a916fcfa1d020c4099e27d87bbb3fb4ce1c77dca85cf3eac46c9c3de87d8022ef7ad2b0a2bb980339293849cf85e5a0a5615ea7bc5bb0a7f28e31f278dc19d628f64c49b888df4c60c40616eefc9286843c2f3f2cf1815988356e409b3f10ffaf60b3468dc0a92dd929cbc8d5da74052c303e7474412f6beaddd551e9056c4e7a5fccdc06107e48f3fe10c40fd2d25d4156e969345c0522669b509e5ced70e4265066eadaf85eea3919d5ded525f8f52d6f0dfa0186c964dd0302fca5bc2dc0540b4ed21085be478c3123996","verification":"130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb"}],"tx":[{"txid":"0x96ef00d2efe03101f5b302f7fc3c8fcd22688944bdc83ab99071d77edbb08581","size":254,"type":"ContractTransaction","version":0,"nonce":3,"sender":"ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG","sys_fee":"0","net_fee":"0","valid_until_block":1200,"attributes":[],"cosigners":[],"vin":[{"txid":"0x33e045101301854a0e07ff96a92ca1ba9b23c19501f1b7eb15ae9eea07b5f370","vout":0}],"vout":[{"address":"ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG","asset":"0x1a5e0e3eac2abced7de9ee2de0820a5c85e63756fcdfc29b82fead86a7c07c78","n":0,"value":"99999000"}],"scripts":[{"invocation":"0c402caebbee911a1f159aa05ab40093d086090a817e837f3f87e8b3e47f6b083649137770f6dda0349ddd611bc47402aca457a89b3b7b0076307ab6a47fd57048eb","verification":"0c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4"}]}]}}`,
			result: func(c *Client) interface{} {
				return getResultBlock202()
			},
		},
	},
	"getblockcount": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockCount()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":991991}`,
			result: func(c *Client) interface{} {
				return uint32(991991)
			},
		},
	},
	"getblockhash": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(1)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"0x4c1e879872344349067c3b1a30781eeb4f9040d3795db7922f513f6f9660b9b2"}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint256DecodeStringLE("4c1e879872344349067c3b1a30781eeb4f9040d3795db7922f513f6f9660b9b2")
				if err != nil {
					panic(err)
				}
				return hash
			},
		},
	},
	"getblockheader": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("68e4bd688b852e807eef13a0ff7da7b02223e359a35153667e88f9cb4a3b0801")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"00000000d039da5e49d63eb0533437d24ff8ceb6aeacf88680599c39f0ffca8948dfcdb94a3def1fca91cf45d69358414e3be77f7621e557f4cebbdb79a47d3cf56ac007f920a05e0000000001000000d60ac443bb800fb08261e75fa5925d747d48586101fd04014055041db6a59c99ab98137cc57e1e56a0a89856a311b2d2fc0aec76ec714c7616edc8fc5c9b81b27f25b7db1a61f64be0730a9cc103efcea1195cc3fe55843e264027e49c647f48bb08d3c32b79ee3432005ea577d7e497f78b46f1e81858848f961b557fb42a92e8eb4433fed203c917cbebb2138a31ed86750fb769d1e70956c0404c20054aa8bd45b520cba9410a9dd6c256481066bb657d7793fbba5551898c91b6dde81285fac841753ccfdd3193d08f19d5431313fa0d926ca965072a5fa3384026b0705078409bcc62fb98bb985edc387edeaaeba37bb7642d88a90762b2c2a62d9b61d53c097d548a368e450c4d995a178d5af28d4c93698233c52de05e3f0094534c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e4c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd624c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc24c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee6995450683073b3bb00"}`,
			result:         func(c *Client) interface{} { return &block.Header{} },
			check: func(t *testing.T, c *Client, result interface{}) {
				res, ok := result.(*block.Header)
				require.True(t, ok)
				assert.Equal(t, uint32(0), res.Version)
				assert.Equal(t, "68e4bd688b852e807eef13a0ff7da7b02223e359a35153667e88f9cb4a3b0801", res.Hash().StringLE())
				assert.Equal(t, "b9cddf4889cafff0399c598086f8acaeb6cef84fd2373453b03ed6495eda39d0", res.PrevHash.StringLE())
				assert.Equal(t, "07c06af53c7da479dbbbcef457e521767fe73b4e415893d645cf91ca1fef3d4a", res.MerkleRoot.StringLE())
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("cbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeaderVerbose(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"hash":"0xcbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86","size":781,"version":0,"nextblockhash":"0x13283c93aec07dc90be3ddd65e2de15e9212f1b3205303f688d6df85129f6b22","previousblockhash":"0x93b540424c1173e487a47582a652686b2885f959ffd895b30e184842403990ef","merkleroot":"0xb8e923148dede20901d6fb225579f6d430cc58f24461d1b0f860ee32abbfcc8d","time":1589300496,"index":202,"nextconsensus":"AXSvJVzydxXuL9da4GVwK25zdesCrVKkHL","confirmations":6,"witnesses":[{"invocation":"0c403620ef8f02d7884c553fb6c54d2fe717cfddd9450886c5fc88a669a29a82fa1a7c715076996567a5a56747f20f10d7e4db071d73b306ccbf17f9a916fcfa1d020c4099e27d87bbb3fb4ce1c77dca85cf3eac46c9c3de87d8022ef7ad2b0a2bb980339293849cf85e5a0a5615ea7bc5bb0a7f28e31f278dc19d628f64c49b888df4c60c40616eefc9286843c2f3f2cf1815988356e409b3f10ffaf60b3468dc0a92dd929cbc8d5da74052c303e7474412f6beaddd551e9056c4e7a5fccdc06107e48f3fe10c40fd2d25d4156e969345c0522669b509e5ced70e4265066eadaf85eea3919d5ded525f8f52d6f0dfa0186c964dd0302fca5bc2dc0540b4ed21085be478c3123996","verification":"130c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e0c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd620c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699140b413073b3bb"}]}}`,
			result: func(c *Client) interface{} {
				b := getResultBlock202()
				return &result.Header{
					Hash:          b.Hash(),
					Size:          781,
					Version:       b.Version,
					NextBlockHash: b.NextBlockHash,
					PrevBlockHash: b.PrevHash,
					MerkleRoot:    b.MerkleRoot,
					Timestamp:     b.Timestamp,
					Index:         b.Index,
					NextConsensus: address.Uint160ToString(b.NextConsensus),
					Witnesses:     []transaction.Witness{b.Script},
					Confirmations: 6,
				}
			},
		},
	},
	"getblocksysfee": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"195500"}`,
			result: func(c *Client) interface{} {
				return util.Fixed8FromInt64(195500)
			},
		},
	},
	"getconnectioncount": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":10}`,
			result: func(c *Client) interface{} {
				return 10
			},
		},
	},
	"getcontractstate": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("dc675afc61a7c0f7b3d2682bf6e1d8ed865a0e5f")
				if err != nil {
					panic(err)
				}
				return c.GetContractState(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"version":0,"hash":"0xdc675afc61a7c0f7b3d2682bf6e1d8ed865a0e5f","script":"5fc56b6c766b00527ac46c766b51527ac46107576f6f6c6f6e676c766b52527ac403574e476c766b53527ac4006c766b54527ac4210354ae498221046c666efebbaee9bd0eb4823469c98e748494a92a71f346b1a6616c766b55527ac46c766b00c3066465706c6f79876c766b56527ac46c766b56c36416006c766b55c36165f2026c766b57527ac462d8016c766b55c36165d801616c766b00c30b746f74616c537570706c79876c766b58527ac46c766b58c36440006168164e656f2e53746f726167652e476574436f6e7465787406737570706c79617c680f4e656f2e53746f726167652e4765746c766b57527ac46270016c766b00c3046e616d65876c766b59527ac46c766b59c36412006c766b52c36c766b57527ac46247016c766b00c30673796d626f6c876c766b5a527ac46c766b5ac36412006c766b53c36c766b57527ac4621c016c766b00c308646563696d616c73876c766b5b527ac46c766b5bc36412006c766b54c36c766b57527ac462ef006c766b00c30962616c616e63654f66876c766b5c527ac46c766b5cc36440006168164e656f2e53746f726167652e476574436f6e746578746c766b51c351c3617c680f4e656f2e53746f726167652e4765746c766b57527ac46293006c766b51c300c36168184e656f2e52756e74696d652e436865636b5769746e657373009c6c766b5d527ac46c766b5dc3640e00006c766b57527ac46255006c766b00c3087472616e73666572876c766b5e527ac46c766b5ec3642c006c766b51c300c36c766b51c351c36c766b51c352c36165d40361527265c9016c766b57527ac4620e00006c766b57527ac46203006c766b57c3616c756653c56b6c766b00527ac4616168164e656f2e53746f726167652e476574436f6e746578746c766b00c3617c680f4e656f2e53746f726167652e4765746165700351936c766b51527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b51c361651103615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e7465787406737570706c79617c680f4e656f2e53746f726167652e4765746165f40251936c766b52527ac46168164e656f2e53746f726167652e476574436f6e7465787406737570706c796c766b52c361659302615272680f4e656f2e53746f726167652e50757461616c756653c56b6c766b00527ac461516c766b51527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b51c361654002615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e7465787406737570706c796c766b51c361650202615272680f4e656f2e53746f726167652e50757461516c766b52527ac46203006c766b52c3616c756659c56b6c766b00527ac46c766b51527ac46c766b52527ac4616168164e656f2e53746f726167652e476574436f6e746578746c766b00c3617c680f4e656f2e53746f726167652e4765746c766b53527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b51c3617c680f4e656f2e53746f726167652e4765746c766b54527ac46c766b53c3616576016c766b52c3946c766b55527ac46c766b54c3616560016c766b52c3936c766b56527ac46c766b55c300a2640d006c766b52c300a2620400006c766b57527ac46c766b57c364ec00616168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b55c36165d800615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e746578746c766b51c36c766b56c361659c00615272680f4e656f2e53746f726167652e5075746155c57600135472616e73666572205375636365737366756cc476516c766b00c3c476526c766b51c3c476536c766b52c3c476546168184e656f2e426c6f636b636861696e2e476574486569676874c46168124e656f2e52756e74696d652e4e6f7469667961516c766b58527ac4620e00006c766b58527ac46203006c766b58c3616c756653c56b6c766b00527ac4616c766b00c36c766b51527ac46c766b51c36c766b52527ac46203006c766b52c3616c756653c56b6c766b00527ac461516c766b00c36a527a527ac46c766b51c36c766b52527ac46203006c766b52c3616c7566","parameters":["ByteArray"],"returntype":"ByteArray","name":"Woolong","code_version":"0.9.2","author":"lllwvlvwlll","email":"lllwvlvwlll@gmail.com","description":"GO NEO!!!","properties":{"storage":true,"dynamic_invoke":false}}}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint160DecodeStringLE("dc675afc61a7c0f7b3d2682bf6e1d8ed865a0e5f")
				if err != nil {
					panic(err)
				}
				script, err := hex.DecodeString("5fc56b6c766b00527ac46c766b51527ac46107576f6f6c6f6e676c766b52527ac403574e476c766b53527ac4006c766b54527ac4210354ae498221046c666efebbaee9bd0eb4823469c98e748494a92a71f346b1a6616c766b55527ac46c766b00c3066465706c6f79876c766b56527ac46c766b56c36416006c766b55c36165f2026c766b57527ac462d8016c766b55c36165d801616c766b00c30b746f74616c537570706c79876c766b58527ac46c766b58c36440006168164e656f2e53746f726167652e476574436f6e7465787406737570706c79617c680f4e656f2e53746f726167652e4765746c766b57527ac46270016c766b00c3046e616d65876c766b59527ac46c766b59c36412006c766b52c36c766b57527ac46247016c766b00c30673796d626f6c876c766b5a527ac46c766b5ac36412006c766b53c36c766b57527ac4621c016c766b00c308646563696d616c73876c766b5b527ac46c766b5bc36412006c766b54c36c766b57527ac462ef006c766b00c30962616c616e63654f66876c766b5c527ac46c766b5cc36440006168164e656f2e53746f726167652e476574436f6e746578746c766b51c351c3617c680f4e656f2e53746f726167652e4765746c766b57527ac46293006c766b51c300c36168184e656f2e52756e74696d652e436865636b5769746e657373009c6c766b5d527ac46c766b5dc3640e00006c766b57527ac46255006c766b00c3087472616e73666572876c766b5e527ac46c766b5ec3642c006c766b51c300c36c766b51c351c36c766b51c352c36165d40361527265c9016c766b57527ac4620e00006c766b57527ac46203006c766b57c3616c756653c56b6c766b00527ac4616168164e656f2e53746f726167652e476574436f6e746578746c766b00c3617c680f4e656f2e53746f726167652e4765746165700351936c766b51527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b51c361651103615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e7465787406737570706c79617c680f4e656f2e53746f726167652e4765746165f40251936c766b52527ac46168164e656f2e53746f726167652e476574436f6e7465787406737570706c796c766b52c361659302615272680f4e656f2e53746f726167652e50757461616c756653c56b6c766b00527ac461516c766b51527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b51c361654002615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e7465787406737570706c796c766b51c361650202615272680f4e656f2e53746f726167652e50757461516c766b52527ac46203006c766b52c3616c756659c56b6c766b00527ac46c766b51527ac46c766b52527ac4616168164e656f2e53746f726167652e476574436f6e746578746c766b00c3617c680f4e656f2e53746f726167652e4765746c766b53527ac46168164e656f2e53746f726167652e476574436f6e746578746c766b51c3617c680f4e656f2e53746f726167652e4765746c766b54527ac46c766b53c3616576016c766b52c3946c766b55527ac46c766b54c3616560016c766b52c3936c766b56527ac46c766b55c300a2640d006c766b52c300a2620400006c766b57527ac46c766b57c364ec00616168164e656f2e53746f726167652e476574436f6e746578746c766b00c36c766b55c36165d800615272680f4e656f2e53746f726167652e507574616168164e656f2e53746f726167652e476574436f6e746578746c766b51c36c766b56c361659c00615272680f4e656f2e53746f726167652e5075746155c57600135472616e73666572205375636365737366756cc476516c766b00c3c476526c766b51c3c476536c766b52c3c476546168184e656f2e426c6f636b636861696e2e476574486569676874c46168124e656f2e52756e74696d652e4e6f7469667961516c766b58527ac4620e00006c766b58527ac46203006c766b58c3616c756653c56b6c766b00527ac4616c766b00c36c766b51527ac46c766b51c36c766b52527ac46203006c766b52c3616c756653c56b6c766b00527ac461516c766b00c36a527a527ac46c766b51c36c766b52527ac46203006c766b52c3616c7566")
				if err != nil {
					panic(err)
				}
				return &result.ContractState{
					Version:     0,
					ScriptHash:  hash,
					Script:      script,
					ParamList:   []smartcontract.ParamType{smartcontract.ByteArrayType},
					ReturnType:  smartcontract.ByteArrayType,
					Name:        "Woolong",
					CodeVersion: "0.9.2",
					Author:      "lllwvlvwlll",
					Email:       "lllwvlvwlll@gmail.com",
					Description: "GO NEO!!!",
					Properties: result.Properties{
						HasStorage:       true,
						HasDynamicInvoke: false,
						IsPayable:        false,
					},
				}
			},
		},
	},
	"getnep5balances": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("1aada0032aba1ef6d1f07bbd8bec1d85f5380fb3")
				if err != nil {
					panic(err)
				}
				return c.GetNEP5Balances(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"balance":[{"asset_hash":"a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8","amount":"50000000000","last_updated_block":251604}],"address":"AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w"}}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint160DecodeStringLE("a48b6e1291ba24211ad11bb90ae2a10bf1fcd5a8")
				if err != nil {
					panic(err)
				}
				return &result.NEP5Balances{
					Balances: []result.NEP5Balance{{
						Asset:       hash,
						Amount:      "50000000000",
						LastUpdated: 251604,
					}},
					Address: "AY6eqWjsUFCzsVELG7yG72XDukKvC34p2w",
				}
			},
		},
	},
	"getnep5transfers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Transfers("AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"sent":[],"received":[{"timestamp":1555651816,"asset_hash":"600c4f5200db36177e3e8a09e9f18e2fc7d12a0f","transfer_address":"AYwgBNMepiv5ocGcyNT4mA8zPLTQ8pDBis","amount":"1000000","block_index":436036,"transfer_notify_index":0,"tx_hash":"df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58"}],"address":"AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF"}}`,
			result: func(c *Client) interface{} {
				assetHash, err := util.Uint160DecodeStringLE("600c4f5200db36177e3e8a09e9f18e2fc7d12a0f")
				if err != nil {
					panic(err)
				}
				txHash, err := util.Uint256DecodeStringLE("df7683ece554ecfb85cf41492c5f143215dd43ef9ec61181a28f922da06aba58")
				if err != nil {
					panic(err)
				}
				return &result.NEP5Transfers{
					Sent: []result.NEP5Transfer{},
					Received: []result.NEP5Transfer{
						{
							Timestamp:   1555651816,
							Asset:       assetHash,
							Address:     "AYwgBNMepiv5ocGcyNT4mA8zPLTQ8pDBis",
							Amount:      "1000000",
							Index:       436036,
							NotifyIndex: 0,
							TxHash:      txHash,
						},
					},
					Address: "AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF",
				}
			},
		},
	},
	"getpeers": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetPeers()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"unconnected":[{"address":"172.200.0.1","port":"20333"}],"connected":[{"address":"127.0.0.1","port":"20335"}],"bad":[{"address":"172.200.0.254","port":"20332"}]}}`,
			result: func(c *Client) interface{} {
				return &result.GetPeers{
					Unconnected: result.Peers{
						{
							Address: "172.200.0.1",
							Port:    "20333",
						},
					},
					Connected: result.Peers{
						{
							Address: "127.0.0.1",
							Port:    "20335",
						},
					},
					Bad: result.Peers{
						{
							Address: "172.200.0.254",
							Port:    "20332",
						},
					},
				}
			},
		},
	},
	"getrawmempool": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawMemPool()
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":["0x9786cce0dddb524c40ddbdd5e31a41ed1f6b5c8a683c122f627ca4a007a7cf4e"]}`,
			result: func(c *Client) interface{} {
				hash, err := util.Uint256DecodeStringLE("9786cce0dddb524c40ddbdd5e31a41ed1f6b5c8a683c122f627ca4a007a7cf4e")
				if err != nil {
					panic(err)
				}
				return []util.Uint256{hash}
			},
		},
	},
	"getrawtransaction": {
		{
			name: "positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("8185b0db7ed77190b93ac8bd44896822cd8f3cfcf702b3f50131e0efd200ef96")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"800003000000316e851039019d39dfc2c37d6c3fee19fd58098700000000000000000000000000000000b004000000000170f3b507ea9eae15ebb7f10195c1239bbaa12ca996ff070e4a8501131045e033000001787cc0a786adfe829bc2dffc5637e6855c0a82e02deee97dedbc2aac3e0e5e1a00184a27db862300316e851039019d39dfc2c37d6c3fee19fd58098701420c402caebbee911a1f159aa05ab40093d086090a817e837f3f87e8b3e47f6b083649137770f6dda0349ddd611bc47402aca457a89b3b7b0076307ab6a47fd57048eb290c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4"}`,
			result:         func(c *Client) interface{} { return &transaction.Transaction{} },
			check: func(t *testing.T, c *Client, result interface{}) {
				res, ok := result.(*transaction.Transaction)
				require.True(t, ok)
				assert.Equal(t, uint8(0), res.Version)
				assert.Equal(t, "8185b0db7ed77190b93ac8bd44896822cd8f3cfcf702b3f50131e0efd200ef96", res.Hash().StringBE())
				assert.Equal(t, transaction.ContractType, res.Type)
				assert.Equal(t, false, res.Trimmed)
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("8185b0db7ed77190b93ac8bd44896822cd8f3cfcf702b3f50131e0efd200ef96")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransactionVerbose(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"blockhash":"0xcbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86","confirmations":8,"blocktime":1589300496,"txid":"0x96ef00d2efe03101f5b302f7fc3c8fcd22688944bdc83ab99071d77edbb08581","size":254,"type":"ContractTransaction","version":0,"nonce":3,"sender":"ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG","sys_fee":"0","net_fee":"0","valid_until_block":1200,"attributes":[],"cosigners":[],"vin":[{"txid":"0x33e045101301854a0e07ff96a92ca1ba9b23c19501f1b7eb15ae9eea07b5f370","vout":0}],"vout":[{"address":"ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG","asset":"0x1a5e0e3eac2abced7de9ee2de0820a5c85e63756fcdfc29b82fead86a7c07c78","n":0,"value":"99999000"}],"scripts":[{"invocation":"0c402caebbee911a1f159aa05ab40093d086090a817e837f3f87e8b3e47f6b083649137770f6dda0349ddd611bc47402aca457a89b3b7b0076307ab6a47fd57048eb","verification":"0c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4"}]}}`,
			result: func(c *Client) interface{} {
				blockHash, err := util.Uint256DecodeStringLE("cbb73ed9e31dc41a8a222749de475e6ebc2a73b99f73b091a72e0b146110fe86")
				if err != nil {
					panic(err)
				}
				sender, err := address.StringToUint160("ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG")
				if err != nil {
					panic(err)
				}
				invocation, err := hex.DecodeString("0c402caebbee911a1f159aa05ab40093d086090a817e837f3f87e8b3e47f6b083649137770f6dda0349ddd611bc47402aca457a89b3b7b0076307ab6a47fd57048eb")
				if err != nil {
					panic(err)
				}
				verification, err := hex.DecodeString("0c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc20b410a906ad4")
				if err != nil {
					panic(err)
				}
				vin, err := util.Uint256DecodeStringLE("33e045101301854a0e07ff96a92ca1ba9b23c19501f1b7eb15ae9eea07b5f370")
				if err != nil {
					panic(err)
				}
				outAddress, err := address.StringToUint160("ALHF9wsXZVEuCGgmDA6ZNsCLtrb4A1g4yG")
				if err != nil {
					panic(err)
				}
				tx := transaction.NewContractTX()
				tx.Nonce = 3
				tx.ValidUntilBlock = 1200
				tx.Sender = sender
				tx.Scripts = []transaction.Witness{
					{
						InvocationScript:   invocation,
						VerificationScript: verification,
					},
				}
				tx.Inputs = []transaction.Input{
					{
						PrevHash:  vin,
						PrevIndex: 0,
					},
				}
				tx.Outputs = []transaction.Output{
					{
						AssetID:    core.GoverningTokenID(),
						Amount:     util.Fixed8FromInt64(99999000),
						ScriptHash: outAddress,
						Position:   0,
					},
				}

				// Update hashes for correct result comparison.
				_ = tx.Hash()

				return &result.TransactionOutputRaw{
					Transaction: tx,
					TransactionMetadata: result.TransactionMetadata{
						Blockhash:     blockHash,
						Confirmations: 8,
						Timestamp:     uint64(1589300496),
					},
				}
			},
		},
	},
	"getstorage": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("03febccf81ac85e3d795bc5cbd4e84e907812aa3")
				if err != nil {
					panic(err)
				}
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorage(hash, key)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":"4c696e"}`,
			result: func(c *Client) interface{} {
				value, err := hex.DecodeString("4c696e")
				if err != nil {
					panic(err)
				}
				return value
			},
		},
	},
	"gettransactionheight": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2")
				if err != nil {
					panic(err)
				}
				return c.GetTransactionHeight(hash)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":1}`,
			result: func(c *Client) interface{} {
				return uint32(1)
			},
		},
	},
	"gettxout": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("f4250dab094c38d8265acc15c366dc508d2e14bf5699e12d9df26577ed74d657")
				if err != nil {
					panic(err)
				}
				return c.GetTxOut(hash, 0)
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"N":0,"Asset":"c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","Value":"2950","Address":"AHCNSDkh2Xs66SzmyKGdoDKY752uyeXDrt"}}`,
			result: func(c *Client) interface{} {
				return &result.TransactionOutput{
					N:       0,
					Asset:   "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
					Value:   util.Fixed8FromInt64(2950),
					Address: "AHCNSDkh2Xs66SzmyKGdoDKY752uyeXDrt",
				}
			},
		},
	},
	"getunclaimed": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimed("AGofsxAUDwt52KjaB664GYsqVAkULYvKNt")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"available":750.032,"unavailable":2815.408,"unclaimed":3565.44}}`,
			result: func(c *Client) interface{} {
				return &result.Unclaimed{
					Available:   util.Fixed8FromFloat(750.032),
					Unavailable: util.Fixed8FromFloat(2815.408),
					Unclaimed:   util.Fixed8FromFloat(3565.44),
				}
			},
		},
	},
	"getvalidators": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetValidators()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":[{"publickey":"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2","votes":"0","active":true},{"publickey":"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e","votes":"0","active":true},{"publickey":"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699","votes":"0","active":true},{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"0","active":true}]}`,
			result:         func(c *Client) interface{} { return []result.Validator{} },
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.([]result.Validator)
				require.True(t, ok)
				assert.Equal(t, 4, len(res))
			},
		},
	},
	"getversion": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetVersion()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"tcp_port":20332,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`,
			result: func(c *Client) interface{} {
				return &result.Version{
					Port:      uint16(20332),
					Nonce:     2153672787,
					UserAgent: "/NEO-GO:0.73.1-pre-273-ge381358/",
				}
			},
		},
	},
	"invokefunction": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("91b83e96f2a7c4fdf0c1688441ec61986c7cae26")
				if err != nil {
					panic(err)
				}
				return c.InvokeFunction("af7c7328eee5a275a3bcaee2bf0cf662b5e739be", "balanceOf", []smartcontract.Parameter{
					{
						Type:  smartcontract.Hash160Type,
						Value: hash,
					},
				})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"1426ae7c6c9861ec418468c1f0fdc4a7f2963eb89151c10962616c616e63654f6667be39e7b562f60cbfe2aebca375a2e5ee28737caf","state":"HALT","gas_consumed":"0.311","stack":[{"type":"ByteArray","value":"262bec084432"}],"tx":"d101361426ae7c6c9861ec418468c1f0fdc4a7f2963eb89151c10962616c616e63654f6667be39e7b562f60cbfe2aebca375a2e5ee28737caf000000000000000000000000"}}`,
			result: func(c *Client) interface{} {
				bytes, err := hex.DecodeString("262bec084432")
				if err != nil {
					panic(err)
				}
				return &result.Invoke{
					State:       "HALT",
					GasConsumed: "0.311",
					Script:      "1426ae7c6c9861ec418468c1f0fdc4a7f2963eb89151c10962616c616e63654f6667be39e7b562f60cbfe2aebca375a2e5ee28737caf",
					Stack: []smartcontract.Parameter{
						{
							Type:  smartcontract.ByteArrayType,
							Value: bytes,
						},
					},
				}
			},
		},
	},
	"invokescript": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript("00046e616d656724058e5e1b6008847cd662728549088a9ee82191")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"script":"00046e616d656724058e5e1b6008847cd662728549088a9ee82191","state":"HALT","gas_consumed":"0.161","stack":[{"type":"ByteArray","value":"4e45503520474153"}],"tx":"d1011b00046e616d656724058e5e1b6008847cd662728549088a9ee82191000000000000000000000000"}}`,
			result: func(c *Client) interface{} {
				bytes, err := hex.DecodeString("4e45503520474153")
				if err != nil {
					panic(err)
				}
				return &result.Invoke{
					State:       "HALT",
					GasConsumed: "0.161",
					Script:      "00046e616d656724058e5e1b6008847cd662728549088a9ee82191",
					Stack: []smartcontract.Parameter{
						{
							Type:  smartcontract.ByteArrayType,
							Value: bytes,
						},
					},
				}
			},
		},
	},
	"sendrawtransaction": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SendRawTransaction(transaction.NewContractTX())
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":true}`,
			result: func(c *Client) interface{} {
				// no error expected
				return nil
			},
		},
	},
	"submitblock": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SubmitBlock(block.Block{
					Base:         block.Base{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":true}`,
			result: func(c *Client) interface{} {
				// no error expected
				return nil
			},
		},
	},
	"validateaddress": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"address":"AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i","isvalid":true}}`,
			result: func(c *Client) interface{} {
				// no error expected
				return nil
			},
		},
	},
}

type rpcClientErrorCase struct {
	name   string
	invoke func(c *Client) (interface{}, error)
}

var rpcClientErrorCases = map[string][]rpcClientErrorCase{
	`{"jsonrpc":"2.0","id":1,"result":"not-a-hex-string"}`: {
		{
			name: "getblock_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblockheader_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
		},
		{
			name: "getrawtransaction_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
		},
		{
			name: "getstorage_not_a_hex_response",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint160DecodeStringLE("03febccf81ac85e3d795bc5cbd4e84e907812aa3")
				if err != nil {
					panic(err)
				}
				key, err := hex.DecodeString("5065746572")
				if err != nil {
					panic(err)
				}
				return c.GetStorage(hash, key)
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":"01"}`: {
		{
			name: "getblock_decodebin_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getheader_decodebin_err",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
		},
		{
			name: "getrawtransaction_decodebin_err",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
		},
	},
	`{"jsonrpc":"2.0","id":1,"result":false}`: {
		{
			name: "sendrawtransaction_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SendRawTransaction(transaction.NewContractTX())
			},
		},
		{
			name: "submitblock_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SubmitBlock(block.Block{
					Base:         block.Base{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
		},
		{
			name: "validateaddress_bad_server_answer",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i")
			},
		},
	},
	`{"id":1,"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"}}`: {
		{
			name: "getaccountstate_invalid_params_error",
			invoke: func(c *Client) (i interface{}, err error) {
				return c.GetAccountState("")
			},
		},
		{
			name: "getapplicationlog_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{})
			},
		},
		{
			name: "getbestblockhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
		},
		{
			name: "getblock_byindex_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblock_byindex_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndexVerbose(1)
			},
		},
		{
			name: "getblock_byhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHash(util.Uint256{})
			},
		},
		{
			name: "getblock_byhash_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHashVerbose(util.Uint256{})
			},
		},
		{
			name: "getblockhash_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(0)
			},
		},
		{
			name: "getblockheader_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeader(util.Uint256{})
			},
		},
		{
			name: "getblockheader_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeaderVerbose(util.Uint256{})
			},
		},
		{
			name: "getblocksysfee_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
		},
		{
			name: "getconnectioncount_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
		},
		{
			name: "getcontractstate_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractState(util.Uint160{})
			},
		},
		{
			name: "getnep5balances_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Balances(util.Uint160{})
			},
		},
		{
			name: "getnep5transfers_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Transfers("")
			},
		},
		{
			name: "getrawtransaction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransaction(util.Uint256{})
			},
		},
		{
			name: "getrawtransaction_verbose_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransactionVerbose(util.Uint256{})
			},
		},
		{
			name: "getstorage_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorage(util.Uint160{}, []byte{})
			},
		},
		{
			name: "gettransactionheight_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTransactionHeight(util.Uint256{})
			},
		},
		{
			name: "gettxoutput_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTxOut(util.Uint256{}, 0)
			},
		},
		{
			name: "getunclaimed_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimed("")
			},
		},
		{
			name: "invokefunction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeFunction("", "", []smartcontract.Parameter{})
			},
		},
		{
			name: "invokescript_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript("")
			},
		},
		{
			name: "sendrawtransaction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SendRawTransaction(&transaction.Transaction{})
			},
		},
		{
			name: "submitblock_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SubmitBlock(block.Block{})
			},
		},
		{
			name: "validateaddress_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("")
			},
		},
	},
	`{}`: {
		{
			name: "getaccountstate_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetAccountState("")
			},
		},
		{
			name: "getapplicationlog_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetApplicationLog(util.Uint256{})
			},
		},
		{
			name: "getbestblockhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBestBlockHash()
			},
		},
		{
			name: "getblock_byindex_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndex(1)
			},
		},
		{
			name: "getblock_byindex_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByIndexVerbose(1)
			},
		},
		{
			name: "getblock_byhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHash(util.Uint256{})
			},
		},
		{
			name: "getblock_byhash_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockByHashVerbose(util.Uint256{})
			},
		},
		{
			name: "getblockcount_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockCount()
			},
		},
		{
			name: "getblockhash_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHash(1)
			},
		},
		{
			name: "getblockheader_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeader(util.Uint256{})
			},
		},
		{
			name: "getblockheader_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockHeaderVerbose(util.Uint256{})
			},
		},
		{
			name: "getblocksysfee_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetBlockSysFee(1)
			},
		},
		{
			name: "getconnectioncount_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetConnectionCount()
			},
		},
		{
			name: "getcontractstate_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetContractState(util.Uint160{})
			},
		},
		{
			name: "getnep5balances_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Balances(util.Uint160{})
			},
		},
		{
			name: "getnep5transfers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetNEP5Transfers("")
			},
		},
		{
			name: "getpeers_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetPeers()
			},
		},
		{
			name: "getrawmempool_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawMemPool()
			},
		},
		{
			name: "getrawtransaction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransaction(util.Uint256{})
			},
		},
		{
			name: "getrawtransaction_verbose_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetRawTransactionVerbose(util.Uint256{})
			},
		},
		{
			name: "getstorage_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStorage(util.Uint160{}, []byte{})
			},
		},
		{
			name: "gettransactionheight_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTransactionHeight(util.Uint256{})
			},
		},
		{
			name: "getxoutput_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetTxOut(util.Uint256{}, 0)
			},
		},
		{
			name: "getunclaimed_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnclaimed("")
			},
		},
		{
			name: "getvalidators_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetValidators()
			},
		},
		{
			name: "getversion_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetVersion()
			},
		},
		{
			name: "invokefunction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeFunction("", "", []smartcontract.Parameter{})
			},
		},
		{
			name: "invokescript_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript("")
			},
		},
		{
			name: "sendrawtransaction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SendRawTransaction(transaction.NewContractTX())
			},
		},
		{
			name: "submitblock_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SubmitBlock(block.Block{
					Base:         block.Base{},
					Transactions: nil,
					Trimmed:      false,
				})
			},
		},
		{
			name: "validateaddress_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.ValidateAddress("")
			},
		},
	},
}

func TestRPCClients(t *testing.T) {
	t.Run("Client", func(t *testing.T) {
		testRPCClient(t, func(ctx context.Context, endpoint string, opts Options) (*Client, error) {
			return New(ctx, endpoint, opts)
		})
	})
	t.Run("WSClient", func(t *testing.T) {
		testRPCClient(t, func(ctx context.Context, endpoint string, opts Options) (*Client, error) {
			wsc, err := NewWS(ctx, httpURLtoWS(endpoint), opts)
			require.NoError(t, err)
			return &wsc.Client, nil
		})
	})
}

func testRPCClient(t *testing.T, newClient func(context.Context, string, Options) (*Client, error)) {
	for method, testBatch := range rpcClientTestCases {
		t.Run(method, func(t *testing.T) {
			for _, testCase := range testBatch {
				t.Run(testCase.name, func(t *testing.T) {
					srv := initTestServer(t, testCase.serverResponse)
					defer srv.Close()

					endpoint := srv.URL
					opts := Options{}
					c, err := newClient(context.TODO(), endpoint, opts)
					if err != nil {
						t.Fatal(err)
					}

					actual, err := testCase.invoke(c)
					assert.NoError(t, err)

					expected := testCase.result(c)
					if testCase.check == nil {
						assert.Equal(t, expected, actual)
					} else {
						testCase.check(t, c, actual)
					}
				})
			}
		})
	}
	for serverResponse, testBatch := range rpcClientErrorCases {
		srv := initTestServer(t, serverResponse)
		defer srv.Close()

		endpoint := srv.URL
		opts := Options{}
		c, err := newClient(context.TODO(), endpoint, opts)
		if err != nil {
			t.Fatal(err)
		}

		for _, testCase := range testBatch {
			t.Run(testCase.name, func(t *testing.T) {
				_, err := testCase.invoke(c)
				assert.Error(t, err)
			})
		}
	}
}

func httpURLtoWS(url string) string {
	return "ws" + strings.TrimPrefix(url, "http") + "/ws"
}

func initTestServer(t *testing.T, resp string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ws" && req.Method == "GET" {
			var upgrader = websocket.Upgrader{}
			ws, err := upgrader.Upgrade(w, req, nil)
			require.NoError(t, err)
			for {
				ws.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, _, err = ws.ReadMessage()
				if err != nil {
					break
				}
				ws.SetWriteDeadline(time.Now().Add(2 * time.Second))
				err = ws.WriteMessage(1, []byte(resp))
				if err != nil {
					break
				}
			}
			ws.Close()
			return
		}
		requestHandler(t, w, resp)
	}))

	return srv
}

func requestHandler(t *testing.T, w http.ResponseWriter, resp string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err := w.Write([]byte(resp))

	if err != nil {
		t.Fatalf("Error writing response: %s", err.Error())
	}
}

func TestCalculateValidUntilBlock(t *testing.T) {
	var (
		getBlockCountCalled int
		getValidatorsCalled int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r := request.NewIn()
		err := r.DecodeData(req.Body)
		if err != nil {
			t.Fatalf("Cannot decode request body: %s", req.Body)
		}
		var response string
		switch r.Method {
		case "getblockcount":
			getBlockCountCalled++
			response = `{"jsonrpc":"2.0","id":1,"result":50}`
		case "getvalidators":
			getValidatorsCalled++
			response = `{"id":1,"jsonrpc":"2.0","result":[{"publickey":"02b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2","votes":"0","active":true},{"publickey":"02103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e","votes":"0","active":true},{"publickey":"03d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee699","votes":"0","active":true},{"publickey":"02a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd62","votes":"0","active":true}]}`
		default:
			t.Fatalf("Bad request method: %s", r.Method)
		}
		requestHandler(t, w, response)
	}))
	defer srv.Close()

	endpoint := srv.URL
	opts := Options{}
	c, err := New(context.TODO(), endpoint, opts)
	if err != nil {
		t.Fatal(err)
	}

	validUntilBlock, err := c.CalculateValidUntilBlock()
	assert.NoError(t, err)
	assert.Equal(t, uint32(54), validUntilBlock)
	assert.Equal(t, 1, getBlockCountCalled)
	assert.Equal(t, 1, getValidatorsCalled)

	// check, whether caching is working
	validUntilBlock, err = c.CalculateValidUntilBlock()
	assert.NoError(t, err)
	assert.Equal(t, uint32(54), validUntilBlock)
	assert.Equal(t, 2, getBlockCountCalled)
	assert.Equal(t, 1, getValidatorsCalled)
}
