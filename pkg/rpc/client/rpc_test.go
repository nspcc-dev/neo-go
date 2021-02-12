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
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
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

// getResultBlock1 returns data for block number 1 which is used by several tests.
func getResultBlock1() *result.Block {
	nextBlockHash, err := util.Uint256DecodeStringLE("cc37d5bc460e72c9423015cb8d579c13e7b03b93bfaa1a23cf4fa777988e035f")
	if err != nil {
		panic(err)
	}
	prevBlockHash, err := util.Uint256DecodeStringLE("996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099")
	if err != nil {
		panic(err)
	}
	merkleRoot, err := util.Uint256DecodeStringLE("cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2")
	if err != nil {
		panic(err)
	}
	invScript, err := hex.DecodeString("40356a91d94e398170e47447d6a0f60aa5470e209782a5452403115a49166db3e1c4a3898122db19f779c30f8ccd0b7d401acdf71eda340655e4ae5237a64961bf4034dd47955e5a71627dafc39dd92999140e9eaeec6b11dbb2b313efa3f1093ed915b4455e199c69ec53778f94ffc236b92f8b97fff97a1f6bbb3770c0c0b3844a40fbe743bd5c90b2f5255e0b073281d7aeb2fb516572f36bec8446bcc37ac755cbf10d08b16c95644db1b2dddc2df5daa377880b20198fc7b967ac6e76474b22df")
	if err != nil {
		panic(err)
	}
	verifScript, err := hex.DecodeString("532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae")
	if err != nil {
		panic(err)
	}
	var nonce uint64
	i, err := fmt.Sscanf("51b484a2fe49ed4d", "%016x", &nonce)
	if i != 1 {
		panic("can't decode nonce")
	}
	if err != nil {
		panic(err)
	}
	nextCon, err := address.StringToUint160("AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU")
	if err != nil {
		panic(err)
	}
	tx := &transaction.Transaction{
		Type:       transaction.MinerType,
		Version:    0,
		Data:       &transaction.MinerTX{Nonce: 4266257741},
		Attributes: []transaction.Attribute{},
		Inputs:     []transaction.Input{},
		Outputs:    []transaction.Output{},
		Scripts:    []transaction.Witness{},
		Trimmed:    false,
	}
	base := &block.Base{
		Version:       0,
		PrevHash:      prevBlockHash,
		MerkleRoot:    merkleRoot,
		Timestamp:     1541215200,
		Index:         1,
		ConsensusData: nonce,
		NextConsensus: nextCon,
		Script: transaction.Witness{
			InvocationScript:   invScript,
			VerificationScript: verifScript,
		},
	}
	// Update hashes for correct result comparison.
	_ = tx.Hash()
	_ = base.Hash()
	return &result.Block{
		Base: base,
		BlockMetadataAndTx: result.BlockMetadataAndTx{
			Size:          452,
			Confirmations: 10534,
			NextBlockHash: &nextBlockHash,
			Tx: []result.Tx{{
				Transaction: tx,
				Fees: result.Fees{
					SysFee: 0,
					NetFee: 0,
				},
			}},
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
			serverResponse: `{"jsonrpc":"2.0","id": 1,"result":{"version":0,"script_hash":"0x1179716da2e9523d153a35fb3ad10c561b1e5b1a","frozen":false,"votes":[],"balances":[{"asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","value":"94"}]}}`,
			result: func(c *Client) interface{} {
				scriptHash, err := util.Uint160DecodeStringLE("1179716da2e9523d153a35fb3ad10c561b1e5b1a")
				if err != nil {
					panic(err)
				}
				return &result.AccountState{
					Version:    0,
					ScriptHash: scriptHash,
					IsFrozen:   false,
					Votes:      []*keys.PublicKey{},
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
	"getassetstate": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetAssetState(util.Uint256{})
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"id":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","type":"GoverningToken","name":"NEO","amount":"100000000","available":"100000000","precision":0,"owner":"00","admin":"Abf2qMs1pzQb8kYk9RuxtUb9jtRKJVuBJt","issuer":"AFmseVrdL9f9oyCzZefL9tG6UbvhPbdYzM","expiration":4000000,"frozen":false}}`,
			result: func(c *Client) interface{} {
				return &result.AssetState{
					ID:         core.GoverningTokenID(),
					AssetType:  0,
					Name:       "NEO",
					Amount:     util.Fixed8FromInt64(100000000),
					Available:  util.Fixed8FromInt64(100000000),
					Precision:  0,
					Owner:      "00",
					Admin:      "Abf2qMs1pzQb8kYk9RuxtUb9jtRKJVuBJt",
					Issuer:     "AFmseVrdL9f9oyCzZefL9tG6UbvhPbdYzM",
					Expiration: 4000000,
					IsFrozen:   false,
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
				return c.GetBlockByIndex(1)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"00000000999086db552ba8f84734bddca55b25a8d3d8c5f866f941209169c38d35376e99b29ffa96224227f5e033c9a291bceef2724429d596c3a6944cafd6995fdb6dcbe013dd5b010000004ded49fea284b451be48d3a3f5d10013ab9ffee489706078714f1ea201c340356a91d94e398170e47447d6a0f60aa5470e209782a5452403115a49166db3e1c4a3898122db19f779c30f8ccd0b7d401acdf71eda340655e4ae5237a64961bf4034dd47955e5a71627dafc39dd92999140e9eaeec6b11dbb2b313efa3f1093ed915b4455e199c69ec53778f94ffc236b92f8b97fff97a1f6bbb3770c0c0b3844a40fbe743bd5c90b2f5255e0b073281d7aeb2fb516572f36bec8446bcc37ac755cbf10d08b16c95644db1b2dddc2df5daa377880b20198fc7b967ac6e76474b22df8b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae0100004ded49fe00000000"}`,
			result:         func(c *Client) interface{} { return &block.Block{} },
			check: func(t *testing.T, c *Client, result interface{}) {
				res, ok := result.(*block.Block)
				require.True(t, ok)
				assert.Equal(t, uint32(0), res.Version)
				assert.Equal(t, "e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c", res.Hash().StringLE())
				assert.Equal(t, "996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099", res.PrevHash.StringLE())
				assert.Equal(t, "cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2", res.MerkleRoot.StringLE())
				assert.Equal(t, 1, len(res.Transactions))
				assert.Equal(t, "cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2", res.Transactions[0].Hash().StringLE())
			},
		},
		{
			name: "byIndex_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				return c.GetBlockByIndexVerbose(1)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"hash":"0xe93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c","size":452,"version":0,"nextblockhash":"0xcc37d5bc460e72c9423015cb8d579c13e7b03b93bfaa1a23cf4fa777988e035f","previousblockhash":"0x996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099","merkleroot":"0xcb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2","time":1541215200,"index":1,"nonce":"51b484a2fe49ed4d","nextconsensus":"AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU","confirmations":10534,"script":{"invocation":"40356a91d94e398170e47447d6a0f60aa5470e209782a5452403115a49166db3e1c4a3898122db19f779c30f8ccd0b7d401acdf71eda340655e4ae5237a64961bf4034dd47955e5a71627dafc39dd92999140e9eaeec6b11dbb2b313efa3f1093ed915b4455e199c69ec53778f94ffc236b92f8b97fff97a1f6bbb3770c0c0b3844a40fbe743bd5c90b2f5255e0b073281d7aeb2fb516572f36bec8446bcc37ac755cbf10d08b16c95644db1b2dddc2df5daa377880b20198fc7b967ac6e76474b22df","verification":"532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae"},"tx":[{"txid":"0xcb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2","size":10,"type":"MinerTransaction","version":0,"attributes":[],"vin":[],"vout":[],"scripts":[],"sys_fee":"0","net_fee":"0","nonce":4266257741}]}}`,
			result: func(c *Client) interface{} {
				return getResultBlock1()
			},
		},
		{
			name: "byHash_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHash(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"00000000999086db552ba8f84734bddca55b25a8d3d8c5f866f941209169c38d35376e99b29ffa96224227f5e033c9a291bceef2724429d596c3a6944cafd6995fdb6dcbe013dd5b010000004ded49fea284b451be48d3a3f5d10013ab9ffee489706078714f1ea201c340356a91d94e398170e47447d6a0f60aa5470e209782a5452403115a49166db3e1c4a3898122db19f779c30f8ccd0b7d401acdf71eda340655e4ae5237a64961bf4034dd47955e5a71627dafc39dd92999140e9eaeec6b11dbb2b313efa3f1093ed915b4455e199c69ec53778f94ffc236b92f8b97fff97a1f6bbb3770c0c0b3844a40fbe743bd5c90b2f5255e0b073281d7aeb2fb516572f36bec8446bcc37ac755cbf10d08b16c95644db1b2dddc2df5daa377880b20198fc7b967ac6e76474b22df8b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae0100004ded49fe00000000"}`,
			result:         func(c *Client) interface{} { return &block.Block{} },
			check: func(t *testing.T, c *Client, result interface{}) {
				res, ok := result.(*block.Block)
				require.True(t, ok)
				assert.Equal(t, uint32(0), res.Version)
				assert.Equal(t, "e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c", res.Hash().StringLE())
				assert.Equal(t, "996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099", res.PrevHash.StringLE())
				assert.Equal(t, "cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2", res.MerkleRoot.StringLE())
				assert.Equal(t, 1, len(res.Transactions))
				assert.Equal(t, "cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2", res.Transactions[0].Hash().StringLE())
			},
		},
		{
			name: "byHash_verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockByHashVerbose(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"hash":"0xe93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c","size":452,"version":0,"nextblockhash":"0xcc37d5bc460e72c9423015cb8d579c13e7b03b93bfaa1a23cf4fa777988e035f","previousblockhash":"0x996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099","merkleroot":"0xcb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2","time":1541215200,"index":1,"nonce":"51b484a2fe49ed4d","nextconsensus":"AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU","confirmations":10534,"script":{"invocation":"40356a91d94e398170e47447d6a0f60aa5470e209782a5452403115a49166db3e1c4a3898122db19f779c30f8ccd0b7d401acdf71eda340655e4ae5237a64961bf4034dd47955e5a71627dafc39dd92999140e9eaeec6b11dbb2b313efa3f1093ed915b4455e199c69ec53778f94ffc236b92f8b97fff97a1f6bbb3770c0c0b3844a40fbe743bd5c90b2f5255e0b073281d7aeb2fb516572f36bec8446bcc37ac755cbf10d08b16c95644db1b2dddc2df5daa377880b20198fc7b967ac6e76474b22df","verification":"532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae"},"tx":[{"txid":"0xcb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2","size":10,"type":"MinerTransaction","version":0,"attributes":[],"vin":[],"vout":[],"scripts":[],"sys_fee":"0","net_fee":"0","nonce":4266257741}]}}`,
			result: func(c *Client) interface{} {
				return getResultBlock1()
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
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeader(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"00000000999086db552ba8f84734bddca55b25a8d3d8c5f866f941209169c38d35376e99b29ffa96224227f5e033c9a291bceef2724429d596c3a6944cafd6995fdb6dcbe013dd5b010000004ded49fea284b451be48d3a3f5d10013ab9ffee489706078714f1ea201c340356a91d94e398170e47447d6a0f60aa5470e209782a5452403115a49166db3e1c4a3898122db19f779c30f8ccd0b7d401acdf71eda340655e4ae5237a64961bf4034dd47955e5a71627dafc39dd92999140e9eaeec6b11dbb2b313efa3f1093ed915b4455e199c69ec53778f94ffc236b92f8b97fff97a1f6bbb3770c0c0b3844a40fbe743bd5c90b2f5255e0b073281d7aeb2fb516572f36bec8446bcc37ac755cbf10d08b16c95644db1b2dddc2df5daa377880b20198fc7b967ac6e76474b22df8b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae00"}`,
			result:         func(c *Client) interface{} { return &block.Header{} },
			check: func(t *testing.T, c *Client, result interface{}) {
				res, ok := result.(*block.Header)
				require.True(t, ok)
				assert.Equal(t, uint32(0), res.Version)
				assert.Equal(t, "e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c", res.Hash().StringLE())
				assert.Equal(t, "996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099", res.PrevHash.StringLE())
				assert.Equal(t, "cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2", res.MerkleRoot.StringLE())
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (i interface{}, err error) {
				hash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				return c.GetBlockHeaderVerbose(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"hash":"0xe93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c","size":442,"version":0,"previousblockhash":"0x996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099","merkleroot":"0xcb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2","time":1541215200,"index":1,"nonce":"51b484a2fe49ed4d","nextconsensus":"AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU","script":{"invocation":"40356a91d94e398170e47447d6a0f60aa5470e209782a5452403115a49166db3e1c4a3898122db19f779c30f8ccd0b7d401acdf71eda340655e4ae5237a64961bf4034dd47955e5a71627dafc39dd92999140e9eaeec6b11dbb2b313efa3f1093ed915b4455e199c69ec53778f94ffc236b92f8b97fff97a1f6bbb3770c0c0b3844a40fbe743bd5c90b2f5255e0b073281d7aeb2fb516572f36bec8446bcc37ac755cbf10d08b16c95644db1b2dddc2df5daa377880b20198fc7b967ac6e76474b22df","verification":"532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae"},"confirmations":20061,"nextblockhash":"0xcc37d5bc460e72c9423015cb8d579c13e7b03b93bfaa1a23cf4fa777988e035f"}}`,
			result: func(c *Client) interface{} {
				b := getResultBlock1()
				return &result.Header{
					Hash:          b.Hash(),
					Size:          442,
					Version:       b.Version,
					NextBlockHash: b.NextBlockHash,
					PrevBlockHash: b.PrevHash,
					MerkleRoot:    b.MerkleRoot,
					Timestamp:     b.Timestamp,
					Index:         b.Index,
					Nonce:         "51b484a2fe49ed4d",
					NextConsensus: address.Uint160ToString(b.NextConsensus),
					Confirmations: 20061,
					Script:        b.Script,
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
	"getclaimable": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetClaimable("AGofsxAUDwt52KjaB664GYsqVAkULYvKNt")
			},
			serverResponse: `{"jsonrpc":"2.0","id":1,"result":{"claimable":[{"txid":"52ba70ef18e879785572c917795cd81422c3820b8cf44c24846a30ee7376fd77","n":1,"value":800000,"start_height":476496,"end_height":488154,"generated":746.112,"sys_fee": 3.92,"unclaimed":750.032}],"address":"AGofsxAUDwt52KjaB664GYsqVAkULYvKNt","unclaimed": 750.032}}`,
			result: func(c *Client) interface{} {
				txID, err := util.Uint256DecodeStringLE("52ba70ef18e879785572c917795cd81422c3820b8cf44c24846a30ee7376fd77")
				if err != nil {
					panic(err)
				}
				return &result.ClaimableInfo{
					Spents: []result.Claimable{
						{
							Tx:          txID,
							N:           1,
							Value:       util.Fixed8FromInt64(800000),
							StartHeight: 476496,
							EndHeight:   488154,
							Generated:   util.Fixed8FromFloat(746.112),
							SysFee:      util.Fixed8FromFloat(3.92),
							Unclaimed:   util.Fixed8FromFloat(750.032),
						}},
					Address:   "AGofsxAUDwt52KjaB664GYsqVAkULYvKNt",
					Unclaimed: util.Fixed8FromFloat(750.032),
				}
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
				return c.GetNEP5Transfers("AbHgdBaWEnHkCiLtDZXjhvhaAK2cwFh5pF", nil, nil, nil, nil)
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
				hash, err := util.Uint256DecodeStringLE("cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransaction(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":"00004ded49fe00000000"}`,
			result:         func(c *Client) interface{} { return &transaction.Transaction{} },
			check: func(t *testing.T, c *Client, result interface{}) {
				res, ok := result.(*transaction.Transaction)
				require.True(t, ok)
				assert.Equal(t, uint8(0), res.Version)
				assert.Equal(t, "cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2", res.Hash().StringLE())
				assert.Equal(t, transaction.MinerType, res.Type)
				assert.Equal(t, false, res.Trimmed)
			},
		},
		{
			name: "verbose_positive",
			invoke: func(c *Client) (interface{}, error) {
				hash, err := util.Uint256DecodeStringLE("cb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2")
				if err != nil {
					panic(err)
				}
				return c.GetRawTransactionVerbose(hash)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"nonce":4266257741,"type":"MinerTransaction","version":0,"attributes":[],"vin":[],"vout":[],"scripts":[],"txid":"0xcb6ddb5f99d6af4c94a6c396d5294472f2eebc91a2c933e0f527422296fa9fb2","size":10,"sys_fee":"0","net_fee":"0","blockhash":"0xe93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c","confirmations":20875,"blocktime":1541215200}}`,
			result: func(c *Client) interface{} {
				blockHash, err := util.Uint256DecodeStringLE("e93d17a52967f9e69314385482bf86f85260e811b46bf4d4b261a7f4135a623c")
				if err != nil {
					panic(err)
				}
				tx := &transaction.Transaction{
					Type:       transaction.MinerType,
					Version:    0,
					Data:       &transaction.MinerTX{Nonce: 4266257741},
					Attributes: []transaction.Attribute{},
					Inputs:     []transaction.Input{},
					Outputs:    []transaction.Output{},
					Scripts:    []transaction.Witness{},
					Trimmed:    false,
				}
				// Update hashes for correct result comparison.
				_ = tx.Hash()

				return &result.TransactionOutputRaw{
					Transaction: tx,
					TransactionMetadata: result.TransactionMetadata{
						SysFee:        0,
						NetFee:        0,
						Blockhash:     blockHash,
						Confirmations: 20875,
						Timestamp:     uint32(1541215200),
					},
				}
			},
		},
	},
	"getstateheight": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStateHeight()
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"blockheight":208,"stateheight":200}}`,
			result: func(c *Client) interface{} {
				return &result.StateHeight{
					BlockHeight: 208,
					StateHeight: 200,
				}
			},
		},
	},
	"getproof": {
		{
			name:           "positive",
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"proof":"256f20ccfbd5f01d5b9633387428b8bab95a9e78c2746573746b657900000000000000000009026d014a060f02000c0c0f0b0d050f00010d050b090603030308070402080b080b0a0b09050a090e07080c020704060507030704060b0605070900000000000000000000000000000000000000092068ae9a1a2c6b6b64216df47b4c44086141569d5d20dc9bd65fdfabed54d2c3250e030c00097465737476616c756500","success":true}}`,
			invoke: func(c *Client) (interface{}, error) {
				return c.GetProof(util.Uint256{}, util.Uint160{}, []byte{})
			},
			result: func(c *Client) interface{} {
				var p result.ProofWithKey
				err := p.FromString("256f20ccfbd5f01d5b9633387428b8bab95a9e78c2746573746b657900000000000000000009026d014a060f02000c0c0f0b0d050f00010d050b090603030308070402080b080b0a0b09050a090e07080c020704060507030704060b0605070900000000000000000000000000000000000000092068ae9a1a2c6b6b64216df47b4c44086141569d5d20dc9bd65fdfabed54d2c3250e030c00097465737476616c756500")
				if err != nil {
					panic(err)
				}
				return &result.GetProof{
					Result:  p,
					Success: true,
				}
			},
		},
	},
	"getstateroot": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetStateRootByHeight(205)
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"stateroot":{"version":0,"index":205,"prehash":"0x3959779e0a406789a96dd86277d444e66eafbaa609b3c60ce18bea202bc30eeb","stateroot":"0x2ae87960aa8e373826fd6d190cc150ceb516e7a48523b0edad58beb54210de7c"},"flag":"Unverified"}}`,
			result: func(c *Client) interface{} {
				return &state.MPTRootState{
					MPTRoot: state.MPTRoot{
						MPTRootBase: state.MPTRootBase{
							Index:    205,
							PrevHash: util.Uint256{0xeb, 0xe, 0xc3, 0x2b, 0x20, 0xea, 0x8b, 0xe1, 0xc, 0xc6, 0xb3, 0x9, 0xa6, 0xba, 0xaf, 0x6e, 0xe6, 0x44, 0xd4, 0x77, 0x62, 0xd8, 0x6d, 0xa9, 0x89, 0x67, 0x40, 0xa, 0x9e, 0x77, 0x59, 0x39},
							Root:     util.Uint256{0x7c, 0xde, 0x10, 0x42, 0xb5, 0xbe, 0x58, 0xad, 0xed, 0xb0, 0x23, 0x85, 0xa4, 0xe7, 0x16, 0xb5, 0xce, 0x50, 0xc1, 0xc, 0x19, 0x6d, 0xfd, 0x26, 0x38, 0x37, 0x8e, 0xaa, 0x60, 0x79, 0xe8, 0x2a},
						},
					},
					Flag: state.Unverified,
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
	"getunspents": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnspents("AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y")
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"balance":[{"unspent":[{"txid":"0x83df8bd085fcb60b2789f7d0a9f876e5f3908567f7877fcba835e899b9dea0b5","n":0,"value":"100000000"}],"asset_hash":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","asset":"NEO","asset_symbol":"NEO","amount":"100000000"},{"unspent":[{"txid":"0x2ab085fa700dd0df4b73a94dc17a092ac3a85cbd965575ea1585d1668553b2f9","n":0,"value":"19351.99993"}],"asset_hash":"0x602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7","asset":"GAS","asset_symbol":"GAS","amount":"19351.99993"}],"address":"AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"}}`,
			result:         func(c *Client) interface{} { return &result.Unspents{} },
			check: func(t *testing.T, c *Client, uns interface{}) {
				res, ok := uns.(*result.Unspents)
				require.True(t, ok)
				assert.Equal(t, "AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y", res.Address)
				assert.Equal(t, 2, len(res.Balance))
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
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"port":20332,"nonce":2153672787,"useragent":"/NEO-GO:0.73.1-pre-273-ge381358/"}}`,
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
				}, nil)
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
				return c.InvokeScript("00046e616d656724058e5e1b6008847cd662728549088a9ee82191", nil)
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
				return nil, c.SendRawTransaction(&transaction.Transaction{
					Type:       transaction.MinerType,
					Version:    0,
					Data:       nil,
					Attributes: []transaction.Attribute{},
					Inputs:     []transaction.Input{},
					Outputs:    []transaction.Output{},
					Scripts:    []transaction.Witness{},
					Trimmed:    false,
				})
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
	"verifyproof": {
		{
			name: "positive",
			invoke: func(c *Client) (interface{}, error) {
				return c.VerifyProof(util.Uint256{}, new(result.ProofWithKey))
			},
			serverResponse: `{"id":1,"jsonrpc":"2.0","result":{"value":"7465737476616c7565"}}`,
			result: func(c *Client) interface{} {
				return &result.VerifyProof{
					Value: []byte("testvalue"),
				}
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
				return nil, c.SendRawTransaction(&transaction.Transaction{
					Type:       transaction.MinerType,
					Version:    0,
					Data:       nil,
					Attributes: []transaction.Attribute{},
					Inputs:     []transaction.Input{},
					Outputs:    []transaction.Output{},
					Scripts:    []transaction.Witness{},
					Trimmed:    false,
				})
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
			name: "getassetstate_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetAssetState(core.GoverningTokenID())
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
			name: "getclaimable_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetClaimable("")
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
				return c.GetNEP5Transfers("", nil, nil, nil, nil)
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
			name: "getunspents_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnspents("")
			},
		},
		{
			name: "invokefunction_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeFunction("", "", []smartcontract.Parameter{}, nil)
			},
		},
		{
			name: "invokescript_invalid_params_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript("", nil)
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
			name: "getassetstate_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetAssetState(core.GoverningTokenID())
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
			name: "getclaimable_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetClaimable("")
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
				return c.GetNEP5Transfers("", nil, nil, nil, nil)
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
			name: "getunspents_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.GetUnspents("")
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
				return c.InvokeFunction("", "", []smartcontract.Parameter{}, nil)
			},
		},
		{
			name: "invokescript_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return c.InvokeScript("", nil)
			},
		},
		{
			name: "sendrawtransaction_unmarshalling_error",
			invoke: func(c *Client) (interface{}, error) {
				return nil, c.SendRawTransaction(&transaction.Transaction{
					Type:       0,
					Version:    0,
					Data:       nil,
					Attributes: nil,
					Inputs:     nil,
					Outputs:    nil,
					Scripts:    nil,
					Trimmed:    false,
				})
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
