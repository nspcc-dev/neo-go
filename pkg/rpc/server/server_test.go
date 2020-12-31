package server

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
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

const testContractHashOld = "b0769b0fe5b510572650d2fc2025ca3f1f0361f6"

const testContractHash = "80f4f684f9f26a1241abf787331f9c8efeb517bb"

var rpcTestCases = map[string][]rpcTestCase{
	"getapplicationlog": {
		{
			name:   "positive",
			params: `["5d7e5b311e7a44fc9a5091c0dcbe15b5bc7c3734caa95d22dbcf07fc55843f66"]`,
			result: func(e *executor) interface{} { return &result.ApplicationLog{} },
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*result.ApplicationLog)

				require.True(t, ok)

				expectedTxHash, err := util.Uint256DecodeStringLE("5d7e5b311e7a44fc9a5091c0dcbe15b5bc7c3734caa95d22dbcf07fc55843f66")
				require.NoError(t, err)
				assert.Equal(t, expectedTxHash, res.TxHash)
				assert.Equal(t, 1, len(res.Executions))
				assert.Equal(t, "Application", res.Executions[0].Trigger)
				assert.Equal(t, "HALT", res.Executions[0].VMState)
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
		{
			name:   "invalid tx type",
			params: `["f9adfde059810f37b3d0686d67f6b29034e0c669537df7e59b40c14a0508b9ed"]`,
			fail:   true,
		},
	},
	"getaccountstate": {
		{
			name:   "positive",
			params: `["AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU"]`,
			result: func(e *executor) interface{} { return &result.AccountState{} },
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*result.AccountState)
				require.True(t, ok)
				assert.Equal(t, 1, len(res.Balances))
				assert.Equal(t, false, res.IsFrozen)
			},
		},
		{
			name:   "positive null",
			params: `["AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"]`,
			result: func(e *executor) interface{} { return &result.AccountState{} },
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*result.AccountState)
				require.True(t, ok)
				assert.Equal(t, 0, len(res.Balances))
				assert.Equal(t, false, res.IsFrozen)
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
			params: fmt.Sprintf(`["%s"]`, testContractHash),
			result: func(e *executor) interface{} { return &result.ContractState{} },
			check: func(t *testing.T, e *executor, cs interface{}) {
				res, ok := cs.(*result.ContractState)
				require.True(t, ok)
				assert.Equal(t, byte(0), res.Version)
				assert.Equal(t, testContractHash, res.ScriptHash.StringLE())
				assert.Equal(t, "0.99", res.CodeVersion)
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
			params: `["a90f00d94349a320376b7cb86c884b53ad76aa2b"]`,
			result: func(e *executor) interface{} { return &result.NEP5Balances{} },
			check:  checkNep5Balances,
		},
		{
			name:   "positive_address",
			params: `["AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs"]`,
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
			params: `["AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs", "notanumber"]`,
			fail:   true,
		},
		{
			name:   "positive",
			params: `["AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs", 0]`,
			result: func(e *executor) interface{} { return &result.NEP5Transfers{} },
			check:  checkNep5Transfers,
		},
		{
			name:   "positive_hash",
			params: `["a90f00d94349a320376b7cb86c884b53ad76aa2b", 0]`,
			result: func(e *executor) interface{} { return &result.NEP5Transfers{} },
			check:  checkNep5Transfers,
		},
	},
	"getproof": {
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid root",
			params: `["0xabcdef"]`,
			fail:   true,
		},
		{
			name:   "invalid contract",
			params: `["0000000000000000000000000000000000000000000000000000000000000000", "0xabcdef"]`,
			fail:   true,
		},
		{
			name:   "invalid key",
			params: `["0000000000000000000000000000000000000000000000000000000000000000", "` + testContractHash + `", "notahex"]`,
			fail:   true,
		},
	},
	"getstateheight": {
		{
			name:   "positive",
			params: `[]`,
			result: func(_ *executor) interface{} { return new(result.StateHeight) },
			check: func(t *testing.T, e *executor, res interface{}) {
				sh, ok := res.(*result.StateHeight)
				require.True(t, ok)

				require.Equal(t, e.chain.BlockHeight(), sh.BlockHeight)
				require.Equal(t, e.chain.StateHeight(), sh.StateHeight)
			},
		},
	},
	"getstateroot": {
		{
			name:   "no params",
			params: `[]`,
			fail:   true,
		},
		{
			name:   "invalid hash",
			params: `["0x1234567890"]`,
			fail:   true,
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
	"getutxotransfers": {
		{
			name:   "invalid address",
			params: `["notanaddress"]`,
			fail:   true,
		},
		{
			name:   "invalid asset",
			params: `["AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs", "notanasset"]`,
			fail:   true,
		},
		{
			name:   "invalid start timestamp",
			params: `["AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs", "neo", "notanumber"]`,
			fail:   true,
		},
		{
			name:   "invalid end timestamp",
			params: `["AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs", "neo", 123, "notanumber"]`,
			fail:   true,
		},
	},
	"getassetstate": {
		{
			name:   "positive",
			params: `["602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"]`,
			result: func(e *executor) interface{} { return &result.AssetState{} },
			check: func(t *testing.T, e *executor, as interface{}) {
				res, ok := as.(*result.AssetState)
				require.True(t, ok)
				assert.Equal(t, "00", res.Owner)
				assert.Equal(t, "AWKECj9RD8rS8RPcpCgYVjk1DeYyHwxZm3", res.Admin)
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
				v := "0x" + e.chain.CurrentBlockHash().StringLE()
				return &v
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
			params: "[2, 1]",
			result: func(e *executor) interface{} { return &result.Block{} },
			check: func(t *testing.T, e *executor, blockRes interface{}) {
				res, ok := blockRes.(*result.Block)
				require.True(t, ok)

				block, err := e.chain.GetBlock(e.chain.GetHeaderHash(2))
				require.NoErrorf(t, err, "could not get block")

				assert.Equal(t, block.Hash(), res.Hash())
				for i := range res.Tx {
					tx := res.Tx[i]
					require.Equal(t, transaction.MinerType, tx.Transaction.Type)

					miner, ok := block.Transactions[i].Data.(*transaction.MinerTX)
					require.True(t, ok)
					require.Equal(t, miner.Nonce, tx.Transaction.Data.(*transaction.MinerTX).Nonce)
					require.Equal(t, block.Transactions[i].Hash(), tx.Transaction.Hash())
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
			name:   "positive, no verbose",
			params: `["ac8239ca86a56ea02c66bc22c17bad194dd35d4e6391359435ab0a04af115fdf"]`,
			result: func(e *executor) interface{} {
				expected := "00000000999086db552ba8f84734bddca55b25a8d3d8c5f866f941209169c38d35376e995f2f29c4685140d85073fec705089706553eae4de3c95d9d8d425af36e597ee6bdc2275f010000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd040140e2ce9b2936c858ec8d8e2ae0dc3e1d924700efb44eecfccc6a7de134ff9b7505edf3e536fe6810f5ce585f6a2976b6bcf9b52addd2234ab73a063a7f52c07988401f91f717a3a386441d05a5c64a4e834299fd56d4f86c8e0eafc4d91e7e013d14a4eaff4d113aef76dedd93c326e9fe9f81e2ee0995ac20f87d56ef7c23bbea39400cf601281f67a69119fad0ef04e5c600b602ba36a490b1e32f006a9135bc5cb4359b855202680a7a07c03d16404738bbf7253c22013f67b2cc2a2a47640b3a63404cd3181b886fe64cf01caad4b4aa0c70587bd0fbcbc2d9cdd15bbc11878159a0da7341be6d06627e614d9e48529c7c96d4b9a4a56aca4fa118f1b49c2789e82b8b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae00"
				return &expected
			},
		},
		{
			name:   "positive, verbose 0",
			params: `["ac8239ca86a56ea02c66bc22c17bad194dd35d4e6391359435ab0a04af115fdf", 0]`,
			result: func(e *executor) interface{} {
				expected := "00000000999086db552ba8f84734bddca55b25a8d3d8c5f866f941209169c38d35376e995f2f29c4685140d85073fec705089706553eae4de3c95d9d8d425af36e597ee6bdc2275f010000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd040140e2ce9b2936c858ec8d8e2ae0dc3e1d924700efb44eecfccc6a7de134ff9b7505edf3e536fe6810f5ce585f6a2976b6bcf9b52addd2234ab73a063a7f52c07988401f91f717a3a386441d05a5c64a4e834299fd56d4f86c8e0eafc4d91e7e013d14a4eaff4d113aef76dedd93c326e9fe9f81e2ee0995ac20f87d56ef7c23bbea39400cf601281f67a69119fad0ef04e5c600b602ba36a490b1e32f006a9135bc5cb4359b855202680a7a07c03d16404738bbf7253c22013f67b2cc2a2a47640b3a63404cd3181b886fe64cf01caad4b4aa0c70587bd0fbcbc2d9cdd15bbc11878159a0da7341be6d06627e614d9e48529c7c96d4b9a4a56aca4fa118f1b49c2789e82b8b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae00"
				return &expected
			},
		},
		{
			name:   "positive, verbose !=0",
			params: `["ac8239ca86a56ea02c66bc22c17bad194dd35d4e6391359435ab0a04af115fdf", 2]`,
			result: func(e *executor) interface{} {
				hash, err := util.Uint256DecodeStringLE("ac8239ca86a56ea02c66bc22c17bad194dd35d4e6391359435ab0a04af115fdf")
				if err != nil {
					panic("can not decode hash parameter")
				}
				block, err := e.chain.GetBlock(hash)
				if err != nil {
					panic("unknown block (update block hash)")
				}
				header := block.Header()
				expected := result.Header{
					Hash:          header.Hash(),
					Size:          io.GetVarSize(header),
					Version:       header.Version,
					PrevBlockHash: header.PrevHash,
					MerkleRoot:    header.MerkleRoot,
					Timestamp:     header.Timestamp,
					Index:         header.Index,
					Nonce:         strconv.FormatUint(header.ConsensusData, 16),
					NextConsensus: address.Uint160ToString(header.NextConsensus),
					Script:        header.Script,
					Confirmations: e.chain.BlockHeight() - header.Index + 1,
				}

				nextHash := e.chain.GetHeaderHash(int(header.Index) + 1)
				if !hash.Equals(util.Uint256{}) {
					expected.NextBlockHash = &nextHash
				}
				return &expected
			},
		},
		{
			name:   "invalid verbose type",
			params: `["ac8239ca86a56ea02c66bc22c17bad194dd35d4e6391359435ab0a04af115fdf", true]`,
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

				var expectedBlockSysFee util.Fixed8
				for _, tx := range block.Transactions {
					expectedBlockSysFee += e.chain.SystemFee(tx)
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
	"getclaimable": {
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
			name:   "normal address",
			params: `["AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU"]`,
			result: func(*executor) interface{} {
				// hash of the issueTx
				h, _ := util.Uint256DecodeStringBE("6da730b566db183bfceb863b780cd92dee2b497e5a023c322c1eaca81cf9ad7a")
				amount := util.Fixed8FromInt64(1 * 8) // (endHeight - startHeight) * genAmount[0]
				return &result.ClaimableInfo{
					Spents: []result.Claimable{
						{
							Tx:        h,
							Value:     util.Fixed8FromInt64(100000000),
							EndHeight: 1,
							Generated: amount,
							Unclaimed: amount,
						},
					},
					Address:   "AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU",
					Unclaimed: amount,
				}
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
			name:   "poositive",
			params: `["3fee783413c27849c8ee2656fd757a7483de64f4e78bd7897f30ecdf42ce788b"]`,
			result: func(e *executor) interface{} {
				h := 202
				return &h
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
	"getunclaimed": {
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
			params: `["AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU"]`,
			result: func(*executor) interface{} {
				return &result.Unclaimed{}
			},
			check: func(t *testing.T, e *executor, uncl interface{}) {
				res, ok := uncl.(*result.Unclaimed)
				require.True(t, ok)
				assert.Equal(t, res.Available, util.Fixed8FromInt64(8))
				assert.True(t, res.Unavailable > 0)
				assert.Equal(t, res.Available+res.Unavailable, res.Unclaimed)
			},
		},
	},
	"getunspents": {
		{
			name:   "positive",
			params: `["AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU"]`,
			result: func(e *executor) interface{} { return &result.Unspents{} },
			check: func(t *testing.T, e *executor, unsp interface{}) {
				res, ok := unsp.(*result.Unspents)
				require.True(t, ok)
				require.Equal(t, 1, len(res.Balance))
				assert.Equal(t, 1, len(res.Balance[0].Unspents))
			},
		},
		{
			name:   "positive null",
			params: `["AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"]`,
			result: func(e *executor) interface{} { return &result.Unspents{} },
			check: func(t *testing.T, e *executor, unsp interface{}) {
				res, ok := unsp.(*result.Unspents)
				require.True(t, ok)
				require.Equal(t, 0, len(res.Balance))
			},
		},
	},
	"getvalidators": {
		{
			params: "[]",
			result: func(*executor) interface{} {
				return &[]result.Validator{}
			},
			check: func(t *testing.T, e *executor, validators interface{}) {
				var expected []result.Validator
				sBValidators, err := e.chain.GetStandByValidators()
				require.NoError(t, err)
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
	"invoke": {
		{
			name:   "positive",
			params: `["50befd26fdf6e4d957c11e078b24ebce6291456f", [{"type": "String", "value": "qwerty"}]]`,
			result: func(e *executor) interface{} { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv interface{}) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "06717765727479676f459162ceeb248b071ec157d9e4f6fd26fdbe50", res.Script)
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
			result: func(e *executor) interface{} {
				v := true
				return &v
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
	"submitblock": {
		{
			name:   "empty block",
			params: `["00000000c09fa5d15c48fc4d444a5cd103cbcfc17d7f336a9470342050a0d543c50e4603edb908054ac1409be5f77d5369c6e03490b2f6676d68d0b3370f8159e0fdadf90500175fd30000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd04014020f6f25257442110f3768ccff694b6f35c46a92859445f80cc57938a69b91bb7c23d1b87eab2e1a2c1fff14d264f2017baefc0d5c0ae586b203782fdd0f5a43d40b885005b735818d30294c5a477c1b736eac396be90a0f741511911efdd76fa47671bd02fa4161a443f84e02639d5ffbc20a341836cbc224a5471afbd89b4e82b40f089b04230dd5f0e872b2b1bfa12120481f0a79b110221a325d00101d6558c38888dcc0cc78f911d6037dfe7a41d1befeb9e055adf80597a0c4fca8fe8f0bb2440d6eb1075244c49cf785e093a60db3532acc7387e723d70562fd9ac74477b149047c6a09909c146906d2ac6c430fbfaead3c151ffa8a31b20db18e7ee43b1e22f8b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae00"]`,
			fail:   true,
		},
		{
			name:   "invalid block height",
			params: `["000000005fb86f62eafe8e9246bc0d1648e4e5c8389dee9fb7fe03fcc6772ec8c5e4ec2aedb908054ac1409be5f77d5369c6e03490b2f6676d68d0b3370f8159e0fdadf99bc05f5e030000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd0401406f299c82b513f59f5bd120317974852c9694c6e10db1ef2f1bb848b1a33e47a08f8dc03ee784166b2060a94cd4e7af88899b39787938f7f2763ea4d2182776ed40f3bafd85214fef38a4836ca97793001ea411f553c51e88781f7b916c59c145bff28314b6e7ea246789422a996fc4937e290a1b40f6b97c5222540f65b0d47aca40d2b3d19203d456428bfdb529e846285052105957385b65388b9a617f6e2d56a64ec41aa73439eafccb52987bb1975c9b67518b053d9e61b445e4a3377dbc206640bd688489bd62adf6bed9d61a73905b9591eb87053c6f0f4dd70f3bee7295541b490caef044b55b6f9f01dc4a05a756a3f2edd06f5adcbe4e984c1e552f9023f08b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae0100000000000000000000"]`,
			fail:   true,
		},
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
		{
			name: "positive",
			// If you are planning to modify test chain from `testblocks.acc`, please, update param value
			params: `["00000000cd1bab8dadfbe297367aa85d77b9b31ea3e4c4a09285eca299fd124dec280dc9edb908054ac1409be5f77d5369c6e03490b2f6676d68d0b3370f8159e0fdadf98fc3275fd30000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd0401401dfd7f28a8c86acc58691d48db403ec29dcc342a0caf0f0ac4ee0329a94772ef05065c8c60921d8931d249b96b19a6059836fd489c8f55bce5e0ff94bc333e19400200f5f0b1c0eafdf41d30277004d987413cac71cc8fc5c73ba23274a18265d63e58fbc94f0118b9fa40411a0f75e478bccf52a8ab885881c443a69d56797f82409ee263f7699cb7b2c73955345a3354410322cad43a197de7138ef37e0fb1061e43cd907d1da725ca5a658babcfb23ed87064b639979b97b95686b0cc58d6d729403fa7fbe626bd60f452fd812ea2cefe81ee4bb374103a28dfe81d385c2d2c9925af3fd32d2868d7321db5b025e7ddf6ab27ac6d37fdfbbe3bae858fd4129128498b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae0100000000000000000000"]`,
			result: func(e *executor) interface{} {
				v := true
				return &v
			},
		},
	},
	"validateaddress": {
		{
			name:   "positive",
			params: `["AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i"]`,
			result: func(*executor) interface{} { return &result.ValidateAddress{} },
			check: func(t *testing.T, e *executor, va interface{}) {
				res, ok := va.(*result.ValidateAddress)
				require.True(t, ok)
				assert.Equal(t, "AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i", res.Address)
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
	t.Run("single request", func(t *testing.T) {
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
	})
	t.Run("batch with single request", func(t *testing.T) {
		for method, cases := range rpcTestCases {
			if method == "sendrawtransaction" || method == "submitblock" {
				continue // cannot send the same transaction twice
			}
			t.Run(method, func(t *testing.T) {
				rpc := `[{"jsonrpc": "2.0", "id": 1, "method": "%s", "params": %s}]`

				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						body := doRPCCall(fmt.Sprintf(rpc, method, tc.params), httpSrv.URL, t)
						result := checkErrGetBatchResult(t, body, tc.fail)
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
	})

	t.Run("batch with multiple requests", func(t *testing.T) {
		for method, cases := range rpcTestCases {
			if method == "sendrawtransaction" || method == "submitblock" {
				continue // cannot send the same transaction twice
			}
			t.Run(method, func(t *testing.T) {
				rpc := `{"jsonrpc": "2.0", "id": %d, "method": "%s", "params": %s},`
				var resultRPC string
				for i, tc := range cases {
					resultRPC += fmt.Sprintf(rpc, i, method, tc.params)
				}
				resultRPC = `[` + resultRPC[:len(resultRPC)-1] + `]`
				body := doRPCCall(resultRPC, httpSrv.URL, t)
				var responses []response.Raw
				err := json.Unmarshal(body, &responses)
				require.Nil(t, err)
				for i, tc := range cases {
					var resp response.Raw
					for _, r := range responses {
						if bytes.Equal(r.ID, []byte(strconv.Itoa(i))) {
							resp = r
							break
						}
					}
					if tc.fail {
						require.NotNil(t, resp.Error)
						assert.NotEqual(t, 0, resp.Error.Code)
						assert.NotEqual(t, "", resp.Error.Message)
					} else {
						assert.Nil(t, resp.Error)
					}
					if tc.fail {
						return
					}
					expected, res := tc.getResultPair(e)
					err := json.Unmarshal(resp.Result, res)
					require.NoErrorf(t, err, "could not parse response: %s", resp.Result)

					if tc.check == nil {
						assert.Equal(t, expected, res)
					} else {
						tc.check(t, e, res)
					}
				}
			})
		}
	})

	t.Run("getproof", func(t *testing.T) {
		r, err := chain.GetStateRoot(210)
		require.NoError(t, err)

		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getproof", "params": ["%s", "%s", "%x"]}`,
			r.Root.StringLE(), testContractHash, []byte("testkey"))
		body := doRPCCall(rpc, httpSrv.URL, t)
		rawRes := checkErrGetResult(t, body, false)
		res := new(result.GetProof)
		require.NoError(t, json.Unmarshal(rawRes, res))
		require.True(t, res.Success)
		h, _ := hex.DecodeString(testContractHash)
		skey := append(h, []byte("testkey")...)
		require.Equal(t, mpt.ToNeoStorageKey(skey), res.Result.Key)
		require.True(t, len(res.Result.Proof) > 0)

		rpc = fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "verifyproof", "params": ["%s", "%s"]}`,
			r.Root.StringLE(), res.Result.String())
		body = doRPCCall(rpc, httpSrv.URL, t)
		rawRes = checkErrGetResult(t, body, false)
		vp := new(result.VerifyProof)
		require.NoError(t, json.Unmarshal(rawRes, vp))
		require.Equal(t, []byte("testvalue"), vp.Value)
	})

	t.Run("getstateroot", func(t *testing.T) {
		testRoot := func(t *testing.T, p string) {
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getstateroot", "params": [%s]}`, p)
			body := doRPCCall(rpc, httpSrv.URL, t)
			rawRes := checkErrGetResult(t, body, false)

			res := new(state.MPTRootState)
			require.NoError(t, json.Unmarshal(rawRes, res))
			require.NotEqual(t, util.Uint256{}, res.Root) // be sure this test uses valid height

			expected, err := e.chain.GetStateRoot(205)
			require.NoError(t, err)
			require.Equal(t, expected, res)
		}
		t.Run("ByHeight", func(t *testing.T) { testRoot(t, strconv.FormatInt(205, 10)) })
		t.Run("ByHash", func(t *testing.T) { testRoot(t, `"`+chain.GetHeaderHash(205).StringLE()+`"`) })
	})

	t.Run("getrawtransaction", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s"]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		result := checkErrGetResult(t, body, false)
		var res string
		err := json.Unmarshal(result, &res)
		require.NoErrorf(t, err, "could not parse response: %s", result)
		assert.Equal(t, "400000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000", res)
	})

	t.Run("getrawtransaction 2 arguments", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 0]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		result := checkErrGetResult(t, body, false)
		var res string
		err := json.Unmarshal(result, &res)
		require.NoErrorf(t, err, "could not parse response: %s", result)
		assert.Equal(t, "400000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000", res)
	})

	t.Run("getrawtransaction 2 arguments, verbose", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 1]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		txOut := checkErrGetResult(t, body, false)
		actual := result.TransactionOutputRaw{}
		err := json.Unmarshal(txOut, &actual)
		require.NoErrorf(t, err, "could not parse response: %s", txOut)
		admin, err := util.Uint160DecodeStringBE("da1745e9b549bd0bfa1a569971c77eba30cd5a4b")
		require.NoError(t, err)

		assert.Equal(t, transaction.RegisterType, actual.Transaction.Type)
		assert.Equal(t, &transaction.RegisterTX{
			AssetType: 0,
			Name:      `[{"lang":"zh-CN","name":"小蚁股"},{"lang":"en","name":"AntShare"}]`,
			Amount:    util.Fixed8FromInt64(100000000),
			Precision: 0,
			Owner:     keys.PublicKey{},
			Admin:     admin,
		}, actual.Transaction.Data.(*transaction.RegisterTX))
		assert.Equal(t, 212, actual.Confirmations)
		assert.Equal(t, TXHash, actual.Transaction.Hash())
	})

	t.Run("getrawtransaction verbose, check outputs", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(1))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 1]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		txOut := checkErrGetResult(t, body, false)
		actual := result.TransactionOutputRaw{}
		err := json.Unmarshal(txOut, &actual)
		require.NoErrorf(t, err, "could not parse response: %s", txOut)
		neoID := core.GoverningTokenID()
		multiAddr, err := util.Uint160DecodeStringBE("be48d3a3f5d10013ab9ffee489706078714f1ea2")
		require.NoError(t, err)
		singleAddr, err := keys.NewPrivateKeyFromWIF("KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY")
		require.NoError(t, err)
		assert.Equal(t, transaction.ContractType, actual.Transaction.Type)
		assert.Equal(t, []transaction.Output{
			{
				AssetID:    neoID,
				Amount:     util.Fixed8FromInt64(99999000),
				ScriptHash: singleAddr.GetScriptHash(),
				Position:   0,
			},
			{
				AssetID:    neoID,
				Amount:     util.Fixed8FromInt64(1000),
				ScriptHash: multiAddr,
				Position:   1,
			},
		}, actual.Transaction.Outputs)
	})

	t.Run("gettxout", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		tx := block.Transactions[3]
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "gettxout", "params": [%s, %d]}"`,
			`"`+tx.Hash().StringLE()+`"`, 0)
		body := doRPCCall(rpc, httpSrv.URL, t)
		res := checkErrGetResult(t, body, false)
		assert.Equal(t, "null", string(res)) // already spent

		block, _ = chain.GetBlock(e.chain.GetHeaderHash(1))
		tx = block.Transactions[1]
		rpc = fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "gettxout", "params": [%s, %d]}"`,
			`"`+tx.Hash().StringLE()+`"`, 1) // Neo remainder in txMoveNeo
		body = doRPCCall(rpc, httpSrv.URL, t)
		res = checkErrGetResult(t, body, false)

		var txOut result.TransactionOutput
		err := json.Unmarshal(res, &txOut)
		require.NoErrorf(t, err, "could not parse response: %s", res)
		assert.Equal(t, 1, txOut.N)
		assert.Equal(t, "0x9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc5", txOut.Asset)
		assert.Equal(t, util.Fixed8FromInt64(1000), txOut.Value)
		assert.Equal(t, "AZ81H31DMWzbSnFDLFkzh9vHwaDLayV7fU", txOut.Address)
	})

	t.Run("getrawmempool", func(t *testing.T) {
		mp := chain.GetMemPool()
		// `expected` stores hashes of previously added txs
		expected := make([]util.Uint256, 0)
		for _, tx := range mp.GetVerifiedTransactions() {
			expected = append(expected, tx.Tx.Hash())
		}
		for i := 0; i < 5; i++ {
			tx := &transaction.Transaction{
				Type: transaction.MinerType,
				Data: &transaction.MinerTX{
					Nonce: uint32(random.Int(0, 1000000000)),
				},
			}
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

	t.Run("getutxotransfers", func(t *testing.T) {
		testGetUTXO := func(t *testing.T, asset string, start, stop, limit, page int, present []int) {
			ps := []string{`"AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs"`}
			if asset != "" {
				ps = append(ps, fmt.Sprintf("%q", asset))
			}
			if start > int(e.chain.HeaderHeight()) {
				ps = append(ps, strconv.Itoa(int(time.Now().Unix())))
			} else {
				b, err := e.chain.GetHeader(e.chain.GetHeaderHash(start))
				require.NoError(t, err)
				ps = append(ps, strconv.Itoa(int(b.Timestamp)))
			}
			if stop != 0 {
				b, err := e.chain.GetHeader(e.chain.GetHeaderHash(stop))
				require.NoError(t, err)
				ps = append(ps, strconv.Itoa(int(b.Timestamp)))
			}
			if limit != 0 {
				ps = append(ps, strconv.Itoa(limit))
			}
			if page != 0 {
				ps = append(ps, strconv.Itoa(page))
			}
			p := strings.Join(ps, ", ")
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getutxotransfers", "params": [%s]}`, p)
			body := doRPCCall(rpc, httpSrv.URL, t)
			res := checkErrGetResult(t, body, false)
			actual := new(result.GetUTXO)
			require.NoError(t, json.Unmarshal(res, actual))
			checkTransfers(t, e, actual, present)
		}
		// See `checkTransfers` for the last parameter values.
		t.Run("All", func(t *testing.T) { testGetUTXO(t, "", 0, 207, 0, 0, []int{0, 1, 2, 3, 4, 5, 6, 7}) })
		t.Run("RestrictByAsset", func(t *testing.T) { testGetUTXO(t, "neo", 0, 0, 0, 0, []int{0, 1, 2, 6, 7}) })
		t.Run("TooBigStart", func(t *testing.T) { testGetUTXO(t, "", 300, 0, 0, 0, []int{}) })
		t.Run("RestrictAll", func(t *testing.T) { testGetUTXO(t, "", 202, 203, 0, 0, []int{1, 2, 3}) })
		t.Run("Limit", func(t *testing.T) { testGetUTXO(t, "neo", 0, 207, 2, 0, []int{7, 6}) })
		t.Run("Limit 2", func(t *testing.T) { testGetUTXO(t, "", 0, 204, 1, 0, []int{5}) })
		t.Run("Limit with page", func(t *testing.T) { testGetUTXO(t, "", 0, 204, 1, 1, []int{4}) })
		t.Run("Limit with page 2", func(t *testing.T) { testGetUTXO(t, "", 0, 204, 2, 2, []int{1, 0}) })
	})

	t.Run("getnep5transfers", func(t *testing.T) {
		ps := []string{`"AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs"`}
		h, err := e.chain.GetHeader(e.chain.GetHeaderHash(207))
		require.NoError(t, err)
		ps = append(ps, strconv.Itoa(int(h.Timestamp)))
		h, err = e.chain.GetHeader(e.chain.GetHeaderHash(208))
		require.NoError(t, err)
		ps = append(ps, strconv.Itoa(int(h.Timestamp)))

		p := strings.Join(ps, ", ")
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getnep5transfers", "params": [%s]}`, p)
		body := doRPCCall(rpc, httpSrv.URL, t)
		res := checkErrGetResult(t, body, false)
		actual := new(result.NEP5Transfers)
		require.NoError(t, json.Unmarshal(res, actual))
		checkNep5TransfersAux(t, e, actual, true)
	})

	t.Run("getalltransfertx", func(t *testing.T) {
		testGetTxs := func(t *testing.T, asset string, start, stop, limit, page int, present []util.Uint256) {
			ps := []string{`"AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs"`}
			ps = append(ps, strconv.Itoa(start))
			ps = append(ps, strconv.Itoa(stop))
			if limit != 0 {
				ps = append(ps, strconv.Itoa(limit))
			}
			if page != 0 {
				ps = append(ps, strconv.Itoa(page))
			}
			p := strings.Join(ps, ", ")
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getalltransfertx", "params": [%s]}`, p)
			body := doRPCCall(rpc, httpSrv.URL, t)
			res := checkErrGetResult(t, body, false)
			actualp := new([]result.TransferTx)
			require.NoError(t, json.Unmarshal(res, actualp))
			actual := *actualp
			require.Equal(t, len(present), len(actual))
			for _, id := range present {
				var isThere bool
				var ttx result.TransferTx
				for i := range actual {
					if id.Equals(actual[i].TxID) {
						ttx = actual[i]
						isThere = true
						break
					}
				}
				require.True(t, isThere)
				tx, h, err := e.chain.GetTransaction(id)
				require.NoError(t, err)
				require.Equal(t, h, ttx.Index)
				require.Equal(t, e.chain.SystemFee(tx).String(), ttx.SystemFee)
				require.Equal(t, e.chain.NetworkFee(tx).String(), ttx.NetworkFee)
				require.Equal(t, len(tx.Inputs)+len(tx.Outputs), len(ttx.Elements))
			}
		}
		b, err := e.chain.GetBlock(e.chain.GetHeaderHash(1))
		require.NoError(t, err)
		txMoveNeo := b.Transactions[1].Hash()
		b, err = e.chain.GetBlock(e.chain.GetHeaderHash(202))
		require.NoError(t, err)
		txNeoRT := b.Transactions[1].Hash()
		b, err = e.chain.GetBlock(e.chain.GetHeaderHash(203))
		require.NoError(t, err)
		txGasClaim := b.Transactions[1].Hash()
		b, err = e.chain.GetBlock(e.chain.GetHeaderHash(204))
		require.NoError(t, err)
		txDeploy := b.Transactions[1].Hash()
		ts204 := int(b.Timestamp)
		b, err = e.chain.GetBlock(e.chain.GetHeaderHash(206))
		require.NoError(t, err)
		txNeoTo1 := b.Transactions[1].Hash()
		b, err = e.chain.GetBlock(e.chain.GetHeaderHash(207))
		require.NoError(t, err)
		txNep5Tr := b.Transactions[2].Hash()
		b, err = e.chain.GetBlock(e.chain.GetHeaderHash(208))
		require.NoError(t, err)
		txNep5To1 := b.Transactions[1].Hash()
		ts208 := int(b.Timestamp)
		b, err = e.chain.GetBlock(e.chain.GetHeaderHash(209))
		require.NoError(t, err)
		txMigrate := b.Transactions[1].Hash()
		b, err = e.chain.GetBlock(e.chain.GetHeaderHash(210))
		require.NoError(t, err)
		txNep5To0 := b.Transactions[1].Hash()
		lastTs := int(b.Timestamp)
		t.Run("All", func(t *testing.T) {
			testGetTxs(t, "", 0, lastTs, 0, 0, []util.Uint256{
				txMoveNeo, txNeoRT, txGasClaim, txDeploy, txNeoTo1, txNep5Tr,
				txNep5To1, txMigrate, txNep5To0,
			})
		})
		t.Run("last 3", func(t *testing.T) {
			testGetTxs(t, "", 0, lastTs, 3, 0, []util.Uint256{
				txNep5To1, txMigrate, txNep5To0,
			})
		})
		t.Run("3, page 1", func(t *testing.T) {
			testGetTxs(t, "", 0, lastTs, 3, 1, []util.Uint256{
				txDeploy, txNeoTo1, txNep5Tr,
			})
		})
		t.Run("3, page 2", func(t *testing.T) {
			testGetTxs(t, "", 0, lastTs, 3, 2, []util.Uint256{
				txMoveNeo, txNeoRT, txGasClaim,
			})
		})
		t.Run("3, page 3", func(t *testing.T) {
			testGetTxs(t, "", 0, lastTs, 3, 3, []util.Uint256{})
		})
		t.Run("no dates", func(t *testing.T) {
			testGetTxs(t, "", 0, 1000000, 0, 0, []util.Uint256{})
		})
		t.Run("204-208", func(t *testing.T) {
			testGetTxs(t, "", ts204, ts208, 0, 0, []util.Uint256{
				txDeploy, txNeoTo1, txNep5Tr, txNep5To1,
			})
		})
	})
	t.Run("getblocktransfertx", func(t *testing.T) {
		bNeo, err := e.chain.GetBlock(e.chain.GetHeaderHash(206))
		require.NoError(t, err)
		txNeoTo1 := bNeo.Transactions[1].Hash()

		body := doRPCCall(`{"jsonrpc": "2.0", "id": 1, "method": "getblocktransfertx", "params": [206]}`, httpSrv.URL, t)
		res := checkErrGetResult(t, body, false)
		actualp := new([]result.TransferTx)
		require.NoError(t, json.Unmarshal(res, actualp))
		expected := []result.TransferTx{
			{
				TxID:       txNeoTo1,
				Timestamp:  bNeo.Timestamp,
				Index:      bNeo.Index,
				SystemFee:  "0",
				NetworkFee: "0",
				Elements: []result.TransferTxEvent{
					{
						Address: "AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs",
						Type:    "input",
						Value:   "99999000",
						Asset:   "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
					},
					{
						Address: "AWLYWXB8C9Lt1nHdDZJnC5cpYJjgRDLk17",
						Type:    "output",
						Value:   "1000",
						Asset:   "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
					},
					{
						Address: "AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs",
						Type:    "output",
						Value:   "99998000",
						Asset:   "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b",
					},
				},
			},
		}
		require.Equal(t, expected, *actualp)

		bNep5, err := e.chain.GetBlock(e.chain.GetHeaderHash(207))
		require.NoError(t, err)
		txNep5Init := bNep5.Transactions[1].Hash()
		txNep5Transfer := bNep5.Transactions[2].Hash()

		body = doRPCCall(`{"jsonrpc": "2.0", "id": 1, "method": "getblocktransfertx", "params": [207]}`, httpSrv.URL, t)
		res = checkErrGetResult(t, body, false)
		actualp = new([]result.TransferTx)
		require.NoError(t, json.Unmarshal(res, actualp))
		expected = []result.TransferTx{
			{
				TxID:       txNep5Init,
				Timestamp:  bNep5.Timestamp,
				Index:      bNep5.Index,
				SystemFee:  "0",
				NetworkFee: "0",
				Events: []result.TransferTxEvent{
					{
						To:    "AeEc6DNaiVZSNJfTJ72rAFFqVKAMR5B7i3",
						Value: "1000000",
						Asset: testContractHashOld,
					},
				},
			},
			{
				TxID:       txNep5Transfer,
				Timestamp:  bNep5.Timestamp,
				Index:      bNep5.Index,
				SystemFee:  "0",
				NetworkFee: "0",
				Events: []result.TransferTxEvent{
					{
						From:  "AeEc6DNaiVZSNJfTJ72rAFFqVKAMR5B7i3",
						To:    "AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs",
						Value: "1000",
						Asset: testContractHashOld,
					},
				},
			},
		}
		require.Equal(t, expected, *actualp)
	})
}

func (tc rpcTestCase) getResultPair(e *executor) (expected interface{}, res interface{}) {
	expected = tc.result(e)
	resVal := reflect.New(reflect.TypeOf(expected).Elem())
	return expected, resVal.Interface()
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

func checkErrGetBatchResult(t *testing.T, body []byte, expectingFail bool) json.RawMessage {
	var resp []response.Raw
	err := json.Unmarshal(body, &resp)
	require.Nil(t, err)
	require.Equal(t, 1, len(resp))
	if expectingFail {
		require.NotNil(t, resp[0].Error)
		assert.NotEqual(t, 0, resp[0].Error.Code)
		assert.NotEqual(t, "", resp[0].Error.Message)
	} else {
		assert.Nil(t, resp[0].Error)
	}
	return resp[0].Result
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
	require.Equal(t, "AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs", res.Address)
	require.Equal(t, 1, len(res.Balances))
	require.Equal(t, "880", res.Balances[0].Amount)
	require.Equal(t, testContractHash, res.Balances[0].Asset.StringLE())
	require.Equal(t, uint32(210), res.Balances[0].LastUpdated)
}

func checkNep5Transfers(t *testing.T, e *executor, acc interface{}) {
	checkNep5TransfersAux(t, e, acc, false)
}

func checkNep5TransfersAux(t *testing.T, e *executor, acc interface{}, onlyFirst bool) {
	res, ok := acc.(*result.NEP5Transfers)
	require.True(t, ok)
	require.Equal(t, "AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs", res.Address)

	assetHash, err := util.Uint160DecodeStringLE(testContractHash)
	require.NoError(t, err)

	assetHashOld, err := util.Uint160DecodeStringLE(testContractHashOld)
	require.NoError(t, err)

	if onlyFirst {
		require.Equal(t, 1, len(res.Received))
		require.Equal(t, "1000", res.Received[0].Amount)
		require.Equal(t, assetHashOld, res.Received[0].Asset)
		require.Equal(t, address.Uint160ToString(assetHashOld), res.Received[0].Address)
	} else {
		require.Equal(t, 3, len(res.Received))

		require.Equal(t, "1000", res.Received[2].Amount)
		require.Equal(t, assetHashOld, res.Received[2].Asset)
		require.Equal(t, address.Uint160ToString(assetHashOld), res.Received[2].Address)

		require.Equal(t, "2", res.Received[1].Amount)
		require.Equal(t, assetHash, res.Received[1].Asset)
		require.Equal(t, "AWLYWXB8C9Lt1nHdDZJnC5cpYJjgRDLk17", res.Received[1].Address)
		require.Equal(t, uint32(0), res.Received[1].NotifyIndex)

		require.Equal(t, "1", res.Received[0].Amount)
		require.Equal(t, assetHash, res.Received[0].Asset)
		require.Equal(t, "AWLYWXB8C9Lt1nHdDZJnC5cpYJjgRDLk17", res.Received[0].Address)
		require.Equal(t, uint32(1), res.Received[0].NotifyIndex)
	}

	require.Equal(t, 1, len(res.Sent))
	require.Equal(t, "123", res.Sent[0].Amount)
	require.Equal(t, assetHashOld, res.Sent[0].Asset)
	require.Equal(t, "AWLYWXB8C9Lt1nHdDZJnC5cpYJjgRDLk17", res.Sent[0].Address)
	require.Equal(t, uint32(0), res.Sent[0].NotifyIndex)
}

func checkTransfers(t *testing.T, e *executor, acc interface{}, checked []int) {
	type transfer struct {
		sent   bool
		asset  string
		index  uint32
		amount int64
	}

	var transfers = []transfer{
		{false, "neo", 1, 99999000},       // NEO to us.
		{false, "neo", 202, 99999000},     // NEO roundtrip for GAS claim.
		{true, "neo", 202, 99999000},      // NEO roundtrip for GAS claim.
		{false, "gas", 203, 160798392000}, // GAS claim.
		{false, "gas", 204, 150798392000}, // Remainder from contract deployment.
		{true, "gas", 204, 160798392000},  // Contract deployment.
		{false, "neo", 206, 99998000},     // Remainder of NEO sent.
		{true, "neo", 206, 99999000},      // NEO to another validator.
	}
	res := acc.(*result.GetUTXO)
	require.Equal(t, res.Address, "AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs")

	for i, tr := range transfers {
		var present bool

		u := getUTXOForBlock(res, tr.sent, tr.asset, tr.index)
		for j := range checked {
			if checked[j] == i {
				present = true
				break
			}
		}
		if present {
			require.NotNil(t, u)
			require.EqualValues(t, tr.amount, u.Amount)
		} else {
			require.Nil(t, u)
		}
	}
}

func getUTXOForBlock(res *result.GetUTXO, sent bool, asset string, b uint32) *result.UTXO {
	arr := res.Received
	if sent {
		arr = res.Sent
	}
	for i := range arr {
		if arr[i].AssetName == strings.ToUpper(asset) {
			for j := range arr[i].Transactions {
				if b == arr[i].Transactions[j].Index {
					return &arr[i].Transactions[j]
				}
			}
		}
	}
	return nil
}
