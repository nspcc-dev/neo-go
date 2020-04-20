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

const testContractHash = "1b5cbfaa2e54b584d3240e38fc4bd413a65ea40c"

var rpcTestCases = map[string][]rpcTestCase{
	"getapplicationlog": {
		{
			name:   "positive",
			params: `["a62dccca145b9df9793ddbe80fd96fd6360403c52926909b9e6fd9a4ac5549aa"]`,
			result: func(e *executor) interface{} { return &result.ApplicationLog{} },
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*result.ApplicationLog)

				require.True(t, ok)

				expectedTxHash, err := util.Uint256DecodeStringLE("a62dccca145b9df9793ddbe80fd96fd6360403c52926909b9e6fd9a4ac5549aa")
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
			params: `["AbHd9dXYUryuCvMgXRUfdR6zC2CJixS6Q6"]`,
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
			params: `["c4bba7ed4e624d038b844d6b6ff24518e7db0165"]`,
			result: func(e *executor) interface{} { return &result.NEP5Balances{} },
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*result.NEP5Balances)
				require.True(t, ok)
				require.Equal(t, "AQyx83BYr1PkyYhZhUAogaHdhkLVHn6htY", res.Address)
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
			params: `["AQyx83BYr1PkyYhZhUAogaHdhkLVHn6htY"]`,
			result: func(e *executor) interface{} { return &result.NEP5Transfers{} },
			check: func(t *testing.T, e *executor, acc interface{}) {
				res, ok := acc.(*result.NEP5Transfers)
				require.True(t, ok)
				require.Equal(t, "AQyx83BYr1PkyYhZhUAogaHdhkLVHn6htY", res.Address)

				assetHash, err := util.Uint160DecodeStringLE(testContractHash)
				require.NoError(t, err)

				require.Equal(t, 1, len(res.Received))
				require.Equal(t, "10", res.Received[0].Amount)
				require.Equal(t, assetHash, res.Received[0].Asset)
				require.Equal(t, address.Uint160ToString(assetHash), res.Received[0].Address)

				require.Equal(t, 1, len(res.Sent))
				require.Equal(t, "1.23", res.Sent[0].Amount)
				require.Equal(t, assetHash, res.Sent[0].Asset)
				require.Equal(t, "AdB6ayKfBRJZasiXX4JL5N2YtmxftNp1b3", res.Sent[0].Address)
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
			params: `["8dd7d330dd7fc103836409bdcba826d15d88119c7f843357266b253aede72dfb"]`,
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
			params: `["26d52a2541614f8639bc030493c4f2be2003094bbba2219aee3e097beb730a8d"]`,
			result: func(e *executor) interface{} {
				expected := "000000002f1f4e815a5951622f4c3863d5a27da7819259517971118de2f979d70850c15d4c8edb530dbd32f20674901fea73dd41c6b1af2ab9520b92447460351727be2c28a3995e010000005704000000000000d60ac443bb800fb08261e75fa5925d747d48586101fd040140e87e9ea015e46febb3ef938d7a458b2f70d84eb0584b2b22b0f85ecfe292d98742569d27ef2f4eafca967a9bbbb1e90efe4046d550f821f70fe39cb07aa70b87409d894b16bc8ab1b1edb0e921e0171ea7e8bf713854896eddd9d1bbc8e2ec80df616e30c0518dd3053a8d53464bb5615bd2cc83aca8fb9a8c37a259a30ba9a571405cfbfee1b86d4819c25f6a97374cef09733fc006e73ee18e5ada58d348983e1b753fad166c7220154fe21ca482feccaf1ff1598660ab7bef087ca16325db4ed9403baed63512e0bec4d5afb8c27525951a8855e579530c9ebf5aae4430492a426abb2eac309828c83e1d132bf9334b31d538fde4a2cfdb9e50da16b29db787152c94534c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e4c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd624c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc24c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee6995450683073b3bb00"
				return &expected
			},
		},
		{
			name:   "positive, verbose 0",
			params: `["26d52a2541614f8639bc030493c4f2be2003094bbba2219aee3e097beb730a8d", 0]`,
			result: func(e *executor) interface{} {
				expected := "000000002f1f4e815a5951622f4c3863d5a27da7819259517971118de2f979d70850c15d4c8edb530dbd32f20674901fea73dd41c6b1af2ab9520b92447460351727be2c28a3995e010000005704000000000000d60ac443bb800fb08261e75fa5925d747d48586101fd040140e87e9ea015e46febb3ef938d7a458b2f70d84eb0584b2b22b0f85ecfe292d98742569d27ef2f4eafca967a9bbbb1e90efe4046d550f821f70fe39cb07aa70b87409d894b16bc8ab1b1edb0e921e0171ea7e8bf713854896eddd9d1bbc8e2ec80df616e30c0518dd3053a8d53464bb5615bd2cc83aca8fb9a8c37a259a30ba9a571405cfbfee1b86d4819c25f6a97374cef09733fc006e73ee18e5ada58d348983e1b753fad166c7220154fe21ca482feccaf1ff1598660ab7bef087ca16325db4ed9403baed63512e0bec4d5afb8c27525951a8855e579530c9ebf5aae4430492a426abb2eac309828c83e1d132bf9334b31d538fde4a2cfdb9e50da16b29db787152c94534c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e4c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd624c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc24c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee6995450683073b3bb00"
				return &expected
			},
		},
		{
			name:   "positive, verbose !=0",
			params: `["26d52a2541614f8639bc030493c4f2be2003094bbba2219aee3e097beb730a8d", 2]`,
			result: func(e *executor) interface{} {
				hash, err := util.Uint256DecodeStringLE("26d52a2541614f8639bc030493c4f2be2003094bbba2219aee3e097beb730a8d")
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
			params: `["AbHd9dXYUryuCvMgXRUfdR6zC2CJixS6Q6"]`,
			result: func(*executor) interface{} {
				// hash of the issueTx
				h, _ := util.Uint256DecodeStringBE("e9a7882fa874508dff8a3f21d32da2afb9f9d303753953a524c75b81e295c914")
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
					Address:   "AbHd9dXYUryuCvMgXRUfdR6zC2CJixS6Q6",
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
			params: `["4b0be2562c7f49a496f08eb6983b46904c99dfb7c65d3a32d7b09cbc503a7889"]`,
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
			params: `["AbHd9dXYUryuCvMgXRUfdR6zC2CJixS6Q6"]`,
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
			params: `["AbHd9dXYUryuCvMgXRUfdR6zC2CJixS6Q6"]`,
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
			params: `["800013000000b004000000013bc3087e4af3b30310500396aa972d7a8ac87a7046dff9dc030e03c2bbc09be9010001a9bf999e43ccfe9d40c3450fc75ca4b02db9b1691a7e1989b331f621456289050030d3dec38623006501dbe71845f26f6b4d848b034d624eeda7bbc40141401026a8d3bd5839d1ead869dcba75dd19c112671d5a86e3443e4fd8c456e8c3fae1ed108065bfe47d62127e33a2419ee1d2140a66712c6ae4b5d2ad0758c5b5dd294c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc250680a906ad4"]`,
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
			params: `["00000000f0fae05006c8c784404415faf69023f9e607f04a1fcfde2a4bd93a8c3fdfd1980000000000000000000000000000000000000000000000000000000000000000f8a3995ed10000005704000000000000d60ac443bb800fb08261e75fa5925d747d48586101fd040140a3f627a07ae219f3dce5525f1b01b913c7dbe9ba6a725bdf49733290e5f3e50f2ba8f434d564ebfaeae40433a4b8746dccb2d4ab36712a87d3600e07548f398140b2e23d220f965736277f509e659f76c8bc376b1b28c3d4c817fbd96ec82eb4e11a5c4fe67b7df6ff8f6efe2a71c3582a7c0403494f84f90c4efc7a314b2c6c8d404f5dcd7ac82e541e7fda965460df0a8563f0ca71c7211263f9dc783a77a6b968bc1f7bf57f34fe48f8978aa94d6c21bcde59bc6dae16eee0d37ec5d8bb0447d240dc6dd5692f1a49862805e18f2a77016ee94e3529cb2856571b9146e53f572fed1c6e31e655b8f94e55a665c25001d3477375db31d6924e88d32ded33a6d6f6cb94534c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e4c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd624c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc24c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee6995450683073b3bb00"]`,
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
			params: `["00000000f0fae05006c8c784404415faf69023f9e607f04a1fcfde2a4bd93a8c3fdfd198980135e10c52e5444c962659b388a5b226114a45d3912a88700ac6b05aaaf961f8a3995ed10000005704000000000000d60ac443bb800fb08261e75fa5925d747d48586101fd0401406413cf294f0506e81b73a8b6f9ecc8d1ac73afc55852ca65ff2dc08ee217425030d8b16ec36d7f7b1111198337185f71ac2b24b9e4ca8806bc2c3ba5f63bf31540f48633e1541acdbdb4509e522b49e4df48b55bc884f5fd5abae014292f74ca7066232967176826fbb0b6919e14b3c9e91b916cb7003c019b8afb42e37cf97e9140d8f44efe93ab380bab8b6fc29e5d8ba1502feda12eb35c9472906e5c46ffdfff56047f383aa87b9724c219eca43d165c31fb246528e410fbcb4a9a04816c77aa40187874fb7193ce5613382e0059f0ff421fc252c31c83e299dfd097a453afb8a65fd95c7711d4d247ad82179e98146338b58864caf8601a6bb7b62300fa9af9e594534c2102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e4c2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd624c2102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc24c2103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee6995450683073b3bb01000014000000b004000000000000"]`,
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
		assert.Equal(t, "4000000000000000000000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000", res)
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
		assert.Equal(t, "4000000000000000000000455b7b226c616e67223a227a682d434e222c226e616d65223a22e5b08fe89a81e882a1227d2c7b226c616e67223a22656e222c226e616d65223a22416e745368617265227d5d0000c16ff28623000000da1745e9b549bd0bfa1a569971c77eba30cd5a4b00000000", res)
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
		assert.Equal(t, "0xa9bf999e43ccfe9d40c3450fc75ca4b02db9b1691a7e1989b331f62145628905", txOut.Asset)
		assert.Equal(t, util.Fixed8FromInt64(100000000), txOut.Value)
		assert.Equal(t, "AbHd9dXYUryuCvMgXRUfdR6zC2CJixS6Q6", txOut.Address)
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
