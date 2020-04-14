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

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
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

const testContractHash = "c2789e5ab9bab828743833965b1df0d5fbcc206f"

var rpcTestCases = map[string][]rpcTestCase{
	"getapplicationlog": {
		{
			name:   "positive",
			params: `["0d77a6c348097c4b29fd9d0fe8657c621bc7d804034c71272e3c4f13775138f2"]`,
			result: func(e *executor) interface{} { return &result.ApplicationLog{} },
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*result.ApplicationLog)

				require.True(t, ok)

				expectedTxHash, err := util.Uint256DecodeStringLE("0d77a6c348097c4b29fd9d0fe8657c621bc7d804034c71272e3c4f13775138f2")
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
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*result.NEP5Balances)
				require.True(t, ok)
				require.Equal(t, "AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs", res.Address)
				require.Equal(t, 1, len(res.Balances))
				require.Equal(t, "8.77", res.Balances[0].Amount)
				require.Equal(t, testContractHash, res.Balances[0].Asset.StringLE())
				require.Equal(t, uint32(208), res.Balances[0].LastUpdated)
			},
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
			name:   "positive",
			params: `["AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs"]`,
			result: func(e *executor) interface{} { return &result.NEP5Transfers{} },
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*result.NEP5Transfers)
				require.True(t, ok)
				require.Equal(t, "AKkkumHbBipZ46UMZJoFynJMXzSRnBvKcs", res.Address)

				assetHash, err := util.Uint160DecodeStringLE(testContractHash)
				require.NoError(t, err)

				require.Equal(t, 1, len(res.Received))
				require.Equal(t, "10", res.Received[0].Amount)
				require.Equal(t, assetHash, res.Received[0].Asset)
				require.Equal(t, address.Uint160ToString(assetHash), res.Received[0].Address)

				require.Equal(t, 1, len(res.Sent))
				require.Equal(t, "1.23", res.Sent[0].Amount)
				require.Equal(t, assetHash, res.Sent[0].Asset)
				require.Equal(t, "AWLYWXB8C9Lt1nHdDZJnC5cpYJjgRDLk17", res.Sent[0].Address)
			},
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
	"getassetstate": {
		{
			name:   "positive",
			params: `["057ea06225860f0f7e69cca1e0052652918629929591b8138a516431be144ba8"]`,
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

				assert.Equal(t, block.Hash(), res.Hash)
				for i := range res.Tx {
					tx := res.Tx[i]
					require.Equal(t, transaction.MinerType, tx.Transaction.Type)

					miner := block.Transactions[i]
					require.True(t, ok)
					require.Equal(t, miner.Nonce, tx.Transaction.Nonce)
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
			params: `["7c32645dab0d87cfaddd5db053e2430c669f807e9efc2b91d1f50a824893352f"]`,
			result: func(e *executor) interface{} {
				expected := "000000002534685cc322d339f957ea368a654bf30b834be4cac0b8441375cce2a37630625507db78d2a2f3130c6073bd68f57202248fdc8966fa8706e88eb0fc61df74085384945e010000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd040140b622b3bdddb8c5e52ae8b4bce5665eb6e2aca950ba72386ea0d37e364af2c7fc40f009c47e1a2140bee90d1e2f81c705d6f2499fe6230b27d1f225c45f38d3c440ec01e743024c2c63dccb2ca4a555eeadb2e6115fe6bc42a8d5c17093828e4ea779b36c6d1535c70c047c709183fc1c93139153291dd0edb614dd3e17d67038dd400f7635816214254b7fba0bb480e2a0ab149c84854adcb542124e67f4cedbf551690f8048271c3f05bbe50f576dcbaf3607afffa2551480cdc24908df0fd47c60408d724d6cfb9bf99e38dc42974f084d110057d3f23a04f5bbf99f5712dad17ae842416bb3e30fee034a317280565cb772f8a61fd09cb05e76db27c5fc7722819f8b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae00"
				return &expected
			},
		},
		{
			name:   "positive, verbose 0",
			params: `["7c32645dab0d87cfaddd5db053e2430c669f807e9efc2b91d1f50a824893352f", 0]`,
			result: func(e *executor) interface{} {
				expected := "000000002534685cc322d339f957ea368a654bf30b834be4cac0b8441375cce2a37630625507db78d2a2f3130c6073bd68f57202248fdc8966fa8706e88eb0fc61df74085384945e010000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd040140b622b3bdddb8c5e52ae8b4bce5665eb6e2aca950ba72386ea0d37e364af2c7fc40f009c47e1a2140bee90d1e2f81c705d6f2499fe6230b27d1f225c45f38d3c440ec01e743024c2c63dccb2ca4a555eeadb2e6115fe6bc42a8d5c17093828e4ea779b36c6d1535c70c047c709183fc1c93139153291dd0edb614dd3e17d67038dd400f7635816214254b7fba0bb480e2a0ab149c84854adcb542124e67f4cedbf551690f8048271c3f05bbe50f576dcbaf3607afffa2551480cdc24908df0fd47c60408d724d6cfb9bf99e38dc42974f084d110057d3f23a04f5bbf99f5712dad17ae842416bb3e30fee034a317280565cb772f8a61fd09cb05e76db27c5fc7722819f8b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae00"
				return &expected
			},
		},
		{
			name:   "positive, verbose !=0",
			params: `["7c32645dab0d87cfaddd5db053e2430c669f807e9efc2b91d1f50a824893352f", 2]`,
			result: func(e *executor) interface{} {
				hash, err := util.Uint256DecodeStringLE("7c32645dab0d87cfaddd5db053e2430c669f807e9efc2b91d1f50a824893352f")
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
			params: `["614a9085dc55fd0539ad3a9d68d8b8e7c52328da905c87bfe8cfca57a5c3c02f", true]`,
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
				h, _ := util.Uint256DecodeStringBE("6a46738cd6f821d3b2d96e68c3feb5fcfe81d08a711baadbb68cbfb034fa29c5")
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
			params: `["3fe72668fa667b8bb0a77ac2375402d52fafc18e6d0a4c12a401dc69bdf515c2"]`,
			result: func(e *executor) interface{} {
				h := 1
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
			params: `["80001300000000015b090ad71ea0c192adc820401b2edc6a197788ee8e5e42a9c5e5e00d700c5da1010001250815d6e93c89e6d2f7279bd5781317d1bfb1ee30aca4c338524654563b3acd0030d3dec38623002baa76ad534b886cb87c6b3720a34943d9000fa90141409646e3dd0dd87685eadac1f682fa63db83729f780a7b2e739cb372ceeaadb3f19260060b34c83fec46e48a6288b2bc5c641e75d1cb358e9c185425a2e6e3fdab232102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc2ac"]`,
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
			// If you are planning to modify test chain from `testblocks.acc`, please, update param value (first block)
			name:   "empty block",
			params: `["00000000565f6b8fab486930d5ece5feb93060f010b556b2d4aa618878326ed93604ed6400000000000000000000000000000000000000000000000000000000000000002385945ed10000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd0401408313b36a61c137d327bec8791dfbb22ae9627fbcd2b023236f65188376f1ff4913c554b276cf7e84c5446fb495df2ed7cd34577ef9a4f9dd0dd27654aa53abe040acf3d5327c50e08bf97d794b6ec021223b8c493f9b71f8844446a01a99e555d517d30b84cbb2a5dcbafb5171373f4d0e6c8aea54f46901455d62545f75bdd2a640c359e2365bfe6f9b7b58a09e7e78f2bee85dbb4f823f9bee31b297cb2eb804eb4a5f1858a729d9676ab2c63b91bee5726c7b38086a04f3fe0ca23155331c476840dfddf4c173a374aacff6d2f37b327256556aa37d7c38c41dfda1f42d25f33d921b41da271aa27f7872f7b2b0b00f90a7ee83f5823ea1cc4f9db90a0100a8b7868b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae00"]`,
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
			// If you are planning to modify test chain from `testblocks.acc`, please, update param value (second block)
			params: `["00000000565f6b8fab486930d5ece5feb93060f010b556b2d4aa618878326ed93604ed645c23aae5b26be3d759680239a4276be0d2e76de303ef3fda96da97e8a3f6a0f12385945ed10000005704000000000000be48d3a3f5d10013ab9ffee489706078714f1ea201fd040140accd1571b3c25e4a6ec80f3bfc8ca3360f51232ed615b756e88424c1c083bd4c0d37f98156c8b87a36710467a75ff3938acea5bbd6a683d439004e2a30665562405de64537c7a411055504bec46851101a3ee9a3db02a7fb0feff781bee43a71ff7e76e03f273437bb94ad1f58dbe29eff32d9299bae0f48860a89a74ed5a48900400a337ce23f8dd767d043ba2b0886ba25d3905a1493ba70a96b297179914d096dfb8621f40e5efa63205af46b8ae71198846affcf02b9137752fcf3d54a683faa408d54fcd4d5235ff48b8dbe7d6f2e0b75ea3dc1bcd563ae44c8c8df201f5039fd30357d8c6709a0639a4aef07a223ca2ae21a74494808bc8b8f2955e7b26bdd148b532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae0100001400000000000000"]`,
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
	chain, handler := initServerWithInMemoryChain(t)

	defer chain.Close()

	e := &executor{chain: chain, handler: handler}
	for method, cases := range rpcTestCases {
		t.Run(method, func(t *testing.T) {
			rpc := `{"jsonrpc": "2.0", "id": 1, "method": "%s", "params": %s}`

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					body := doRPCCall(fmt.Sprintf(rpc, method, tc.params), handler, t)
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

	t.Run("getrawtransaction", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s"]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, handler, t)
		result := checkErrGetResult(t, body, false)
		var res string
		err := json.Unmarshal(result, &res)
		require.NoErrorf(t, err, "could not parse response: %s", result)
		assert.Equal(t, "40000000000000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000", res)
	})

	t.Run("getrawtransaction 2 arguments", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 0]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, handler, t)
		result := checkErrGetResult(t, body, false)
		var res string
		err := json.Unmarshal(result, &res)
		require.NoErrorf(t, err, "could not parse response: %s", result)
		assert.Equal(t, "40000000000000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000", res)
	})

	t.Run("getrawtransaction 2 arguments, verbose", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		TXHash := block.Transactions[1].Hash()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 1]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, handler, t)
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
		assert.Equal(t, 210, actual.Confirmations)
		assert.Equal(t, TXHash, actual.Transaction.Hash())
	})

	t.Run("gettxout", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(0))
		tx := block.Transactions[3]
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "gettxout", "params": [%s, %d]}"`,
			`"`+tx.Hash().StringLE()+`"`, 0)
		body := doRPCCall(rpc, handler, t)
		res := checkErrGetResult(t, body, false)

		var txOut result.TransactionOutput
		err := json.Unmarshal(res, &txOut)
		require.NoErrorf(t, err, "could not parse response: %s", res)
		assert.Equal(t, 0, txOut.N)
		assert.Equal(t, "0x250815d6e93c89e6d2f7279bd5781317d1bfb1ee30aca4c338524654563b3acd", txOut.Asset)
		assert.Equal(t, util.Fixed8FromInt64(100000000), txOut.Value)
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
			tx := transaction.NewMinerTX()
			assert.NoError(t, mp.Add(tx, &FeerStub{}))
			expected = append(expected, tx.Hash())
		}

		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getrawmempool", "params": []}`
		body := doRPCCall(rpc, handler, t)
		res := checkErrGetResult(t, body, false)

		var actual []util.Uint256
		err := json.Unmarshal(res, &actual)
		require.NoErrorf(t, err, "could not parse response: %s", res)

		assert.ElementsMatch(t, expected, actual)
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
		assert.NotEqual(t, 0, resp.Error.Code)
		assert.NotEqual(t, "", resp.Error.Message)
	} else {
		assert.Nil(t, resp.Error)
	}
	return resp.Result
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
