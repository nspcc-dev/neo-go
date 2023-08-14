package rpcsrv

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	gio "io"
	"math"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dboper"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	rpc2 "github.com/nspcc-dev/neo-go/pkg/services/oracle/broadcaster"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv/params"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/invocations"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap/zapcore"
)

type executor struct {
	chain   *core.Blockchain
	httpSrv *httptest.Server
}

type rpcTestCase struct {
	name    string
	params  string
	fail    bool
	errCode int64
	result  func(e *executor) any
	check   func(t *testing.T, e *executor, result any)
}

const genesisBlockHash = "0f8fb4e17d2ab9f3097af75ca7fd16064160fb8043db94909e00dd4e257b9dc4"
const testContractHash = "565cff9508ebc75aadd7fe59f38dac610ab6093c"
const deploymentTxHash = "a14390941cc3a1d87393eff720a722e9cd350bd6ed233c5fe2001326c80eb68e"

const (
	verifyContractHash                = "06ed5314c2e4cb103029a60b86d46afa2fb8f67c"
	verifyContractAVM                 = "VwIAQS1RCDBwDBTunqIsJ+NL0BSPxBCOCPdOj1BIskrZMCQE2zBxaBPOStkoJATbKGlK2SgkBNsol0A="
	verifyWithArgsContractHash        = "4dc916254efd2947c93b11207e8ffc0bb56161c5"
	nnsContractHash                   = "892429fcd47c30f8451781acc627e8b20e0d64f3"
	nnsToken1ID                       = "6e656f2e636f6d"
	nfsoContractHash                  = "730ebe719ab8e3b69d11dafc95cdb9bf409db179"
	nfsoToken1ID                      = "7e244ffd6aa85fb1579d2ed22e9b761ab62e3486"
	storageContractHash               = "ebc0c16a76c808cd4dde6bcc063f09e45e331ec7"
	faultedTxHashLE                   = "82279bfe9bada282ca0f8cb8e0bb124b921af36f00c69a518320322c6f4fef60"
	faultedTxBlock             uint32 = 23
	invokescriptContractAVM           = "VwIADBQBDAMOBQYMDQIODw0DDgcJAAAAAErZMCQE2zBwaEH4J+yMqiYEEUAMFA0PAwIJAAIBAwcDBAUCAQAOBgwJStkwJATbMHFpQfgn7IyqJgQSQBNA"
	block20StateRootLE                = "ae445869283f8d7e0debc3f455014c73cde21b9802db99e80248da9f393bce14"
)

var (
	nnsHash, _            = util.Uint160DecodeStringLE(nnsContractHash)
	nfsoHash, _           = util.Uint160DecodeStringLE(nfsoContractHash)
	nfsoToken1ContainerID = util.Uint256{1, 2, 3}
	nfsoToken1ObjectID    = util.Uint256{4, 5, 6}
)

var rpcFunctionsWithUnsupportedStatesTestCases = map[string][]rpcTestCase{
	"getproof": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.ErrUnsupportedStateCode,
		},
	},
	"verifyproof": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.ErrUnsupportedStateCode,
		},
	},
	"getstate": {
		{
			name:    "unknown root/item",
			params:  `["0000000000000000000000000000000000000000000000000000000000000000", "` + testContractHash + `", "QQ=="]`,
			fail:    true,
			errCode: neorpc.ErrUnsupportedStateCode,
		},
	},
	"findstates": {
		{
			name:    "invalid contract",
			params:  `["` + block20StateRootLE + `", "0xabcdef"]`,
			fail:    true,
			errCode: neorpc.ErrUnsupportedStateCode,
		},
	},
	"invokefunctionhistoric": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.ErrUnsupportedStateCode,
		},
	},
}

var rpcTestCases = map[string][]rpcTestCase{
	"getapplicationlog": {
		{
			name:   "positive",
			params: `["` + deploymentTxHash + `"]`,
			result: func(e *executor) any { return &result.ApplicationLog{} },
			check: func(t *testing.T, e *executor, acc any) {
				res, ok := acc.(*result.ApplicationLog)
				require.True(t, ok)
				expectedTxHash, err := util.Uint256DecodeStringLE(deploymentTxHash)
				require.NoError(t, err)
				assert.Equal(t, 1, len(res.Executions))
				assert.Equal(t, expectedTxHash, res.Container)
				assert.Equal(t, trigger.Application, res.Executions[0].Trigger)
				assert.Equal(t, vmstate.Halt, res.Executions[0].VMState)
			},
		},
		{
			name:   "positive, genesis block",
			params: `["` + genesisBlockHash + `"]`,
			result: func(e *executor) any { return &result.ApplicationLog{} },
			check: func(t *testing.T, e *executor, acc any) {
				res, ok := acc.(*result.ApplicationLog)
				require.True(t, ok)
				assert.Equal(t, genesisBlockHash, res.Container.StringLE())
				assert.Equal(t, 2, len(res.Executions))
				assert.Equal(t, trigger.OnPersist, res.Executions[0].Trigger)
				assert.Equal(t, trigger.PostPersist, res.Executions[1].Trigger)
				assert.Equal(t, vmstate.Halt, res.Executions[0].VMState)
			},
		},
		{
			name:   "positive, genesis block, postPersist",
			params: `["` + genesisBlockHash + `", "PostPersist"]`,
			result: func(e *executor) any { return &result.ApplicationLog{} },
			check: func(t *testing.T, e *executor, acc any) {
				res, ok := acc.(*result.ApplicationLog)
				require.True(t, ok)
				assert.Equal(t, genesisBlockHash, res.Container.StringLE())
				assert.Equal(t, 1, len(res.Executions))
				assert.Equal(t, trigger.PostPersist, res.Executions[0].Trigger)
				assert.Equal(t, vmstate.Halt, res.Executions[0].VMState)
			},
		},
		{
			name:   "positive, genesis block, onPersist",
			params: `["` + genesisBlockHash + `", "OnPersist"]`,
			result: func(e *executor) any { return &result.ApplicationLog{} },
			check: func(t *testing.T, e *executor, acc any) {
				res, ok := acc.(*result.ApplicationLog)
				require.True(t, ok)
				assert.Equal(t, genesisBlockHash, res.Container.StringLE())
				assert.Equal(t, 1, len(res.Executions))
				assert.Equal(t, trigger.OnPersist, res.Executions[0].Trigger)
				assert.Equal(t, vmstate.Halt, res.Executions[0].VMState)
			},
		},
		{
			name:    "invalid trigger (not a string)",
			params:  `["` + genesisBlockHash + `", 1]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid address",
			params:  `["notahash"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid tx hash",
			params:  `["d24cc1d52b5c0216cbf3835bb5bac8ccf32639fa1ab6627ec4e2b9f33f7ec02f"]`,
			fail:    true,
			errCode: neorpc.ErrUnknownScriptContainerCode,
		},
	},
	"getcontractstate": {
		{
			name:   "positive, by hash",
			params: fmt.Sprintf(`["%s"]`, testContractHash),
			result: func(e *executor) any { return &state.Contract{} },
			check: func(t *testing.T, e *executor, cs any) {
				res, ok := cs.(*state.Contract)
				require.True(t, ok)
				assert.Equal(t, testContractHash, res.Hash.StringLE())
			},
		},
		{
			name:   "positive, by id",
			params: `[1]`,
			result: func(e *executor) any { return &state.Contract{} },
			check: func(t *testing.T, e *executor, cs any) {
				res, ok := cs.(*state.Contract)
				require.True(t, ok)
				assert.Equal(t, int32(1), res.ID)
			},
		},
		{
			name:   "positive, native by id",
			params: `[-3]`,
			result: func(e *executor) any { return &state.Contract{} },
			check: func(t *testing.T, e *executor, cs any) {
				res, ok := cs.(*state.Contract)
				require.True(t, ok)
				assert.Equal(t, int32(-3), res.ID)
			},
		},
		{
			name:   "positive, native by name",
			params: `["PolicyContract"]`,
			result: func(e *executor) any { return &state.Contract{} },
			check: func(t *testing.T, e *executor, cs any) {
				res, ok := cs.(*state.Contract)
				require.True(t, ok)
				assert.Equal(t, int32(-7), res.ID)
			},
		},
		{
			name:    "negative, bad hash",
			params:  `["6d1eeca891ee93de2b7a77eb91c26f3b3c04d6c3"]`,
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
		{
			name:    "negative, bad ID",
			params:  `[-100]`,
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
		{
			name:    "negative, bad native name",
			params:  `["unknown_native"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid hash",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"getnep11balances": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid address",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:   "positive",
			params: `["` + testchain.PrivateKeyByID(0).GetScriptHash().StringLE() + `"]`,
			result: func(e *executor) any { return &result.NEP11Balances{} },
			check:  checkNep11Balances,
		},
		{
			name:   "positive_address",
			params: `["` + address.Uint160ToString(testchain.PrivateKeyByID(0).GetScriptHash()) + `"]`,
			result: func(e *executor) any { return &result.NEP11Balances{} },
			check:  checkNep11Balances,
		},
	},
	"getnep11properties": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid address",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "no token",
			params:  `["` + nnsContractHash + `"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bad token",
			params:  `["` + nnsContractHash + `", "abcdef"]`,
			fail:    true,
			errCode: neorpc.ErrExecutionFailedCode,
		},
		{
			name:   "positive",
			params: `["` + nnsContractHash + `", "6e656f2e636f6d"]`,
			result: func(e *executor) any {
				return &map[string]any{
					"name":       "neo.com",
					"expiration": "lhbLRl0B",
					"admin":      nil,
				}
			},
		},
	},
	"getnep11transfers": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid address",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid timestamp",
			params:  `["` + testchain.PrivateKeyByID(0).Address() + `", "notanumber"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid stop timestamp",
			params:  `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "blah"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:   "positive",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", 0]`,
			result: func(e *executor) any { return &result.NEP11Transfers{} },
			check:  checkNep11Transfers,
		},
	},
	"getnep17balances": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid address",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:   "positive",
			params: `["` + testchain.PrivateKeyByID(0).GetScriptHash().StringLE() + `"]`,
			result: func(e *executor) any { return &result.NEP17Balances{} },
			check:  checkNep17Balances,
		},
		{
			name:   "positive_address",
			params: `["` + address.Uint160ToString(testchain.PrivateKeyByID(0).GetScriptHash()) + `"]`,
			result: func(e *executor) any { return &result.NEP17Balances{} },
			check:  checkNep17Balances,
		},
	},
	"getnep17transfers": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid address",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid timestamp",
			params:  `["` + testchain.PrivateKeyByID(0).Address() + `", "notanumber"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid stop timestamp",
			params:  `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "blah"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid limit",
			params:  `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "0"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid limit 2",
			params:  `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "bleh"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid limit 3",
			params:  `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "100500"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid page",
			params:  `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "3", "-1"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid page 2",
			params:  `["` + testchain.PrivateKeyByID(0).Address() + `", "1", "2", "3", "jajaja"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:   "positive",
			params: `["` + testchain.PrivateKeyByID(0).Address() + `", 0]`,
			result: func(e *executor) any { return &result.NEP17Transfers{} },
			check:  checkNep17Transfers,
		},
		{
			name:   "positive_hash",
			params: `["` + testchain.PrivateKeyByID(0).GetScriptHash().StringLE() + `", 0]`,
			result: func(e *executor) any { return &result.NEP17Transfers{} },
			check:  checkNep17Transfers,
		},
	},
	"getproof": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid root",
			params:  `["0xabcdef"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid contract",
			params:  `["0000000000000000000000000000000000000000000000000000000000000000", "0xabcdef"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid key",
			params:  `["0000000000000000000000000000000000000000000000000000000000000000", "` + testContractHash + `", "notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"getstate": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid root",
			params:  `["0xabcdef"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid contract",
			params:  `["0000000000000000000000000000000000000000000000000000000000000000", "0xabcdef"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid key",
			params:  `["0000000000000000000000000000000000000000000000000000000000000000", "` + testContractHash + `", "notabase64%"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "unknown contract",
			params:  `["0000000000000000000000000000000000000000000000000000000000000000", "0000000000000000000000000000000000000000", "QQ=="]`,
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
		{
			name:    "unknown root/item",
			params:  `["0000000000000000000000000000000000000000000000000000000000000000", "` + testContractHash + `", "QQ=="]`,
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
	},
	"findstates": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid root",
			params:  `["0xabcdef"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid contract",
			params:  `["` + block20StateRootLE + `", "0xabcdef"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid prefix",
			params:  `["` + block20StateRootLE + `", "` + testContractHash + `", "notabase64%"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid key",
			params:  `["` + block20StateRootLE + `", "` + testContractHash + `", "QQ==", "notabase64%"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "unknown contract/large count",
			params:  `["` + block20StateRootLE + `", "0000000000000000000000000000000000000000", "QQ==", "QQ==", 101]`,
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
	},
	"getstateheight": {
		{
			name:   "positive",
			params: `[]`,
			result: func(_ *executor) any { return new(result.StateHeight) },
			check: func(t *testing.T, e *executor, res any) {
				sh, ok := res.(*result.StateHeight)
				require.True(t, ok)

				require.Equal(t, e.chain.BlockHeight(), sh.Local)
				require.Equal(t, uint32(0), sh.Validated)
			},
		},
	},
	"getstateroot": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid hash",
			params:  `["0x1234567890"]`,
			fail:    true,
			errCode: neorpc.ErrUnknownStateRootCode,
		},
	},
	"getstorage": {
		{
			name:   "positive",
			params: fmt.Sprintf(`["%s", "dGVzdGtleQ=="]`, testContractHash),
			result: func(e *executor) any {
				v := base64.StdEncoding.EncodeToString([]byte("newtestvalue"))
				return &v
			},
		},
		{
			name:   "missing key",
			params: fmt.Sprintf(`["%s", "dGU="]`, testContractHash),
			result: func(e *executor) any {
				v := ""
				return &v
			},
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "no second parameter",
			params:  fmt.Sprintf(`["%s"]`, testContractHash),
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid hash",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid key",
			params:  fmt.Sprintf(`["%s", "notabase64$"]`, testContractHash),
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"getbestblockhash": {
		{
			params: "[]",
			result: func(e *executor) any {
				v := "0x" + e.chain.CurrentBlockHash().StringLE()
				return &v
			},
		},
	},
	"getblock": {
		{
			name:   "positive",
			params: "[3, 1]",
			result: func(_ *executor) any { return &result.Block{} },
			check: func(t *testing.T, e *executor, blockRes any) {
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
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bad params",
			params:  `[[]]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid height",
			params:  `[-1]`,
			fail:    true,
			errCode: neorpc.ErrUnknownHeightCode,
		},
		{
			name:    "invalid hash",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "missing hash",
			params:  `["` + util.Uint256{}.String() + `"]`,
			fail:    true,
			errCode: neorpc.ErrUnknownBlockCode,
		},
	},
	"getblockcount": {
		{
			params: "[]",
			result: func(e *executor) any {
				v := int(e.chain.BlockHeight() + 1)
				return &v
			},
		},
	},
	"getblockhash": {
		{
			params: "[1]",
			result: func(e *executor) any {
				// We don't have `t` here for proper handling, but
				// error here would lead to panic down below.
				block, _ := e.chain.GetBlock(e.chain.GetHeaderHash(1))
				expectedHash := "0x" + block.Hash().StringLE()
				return &expectedHash
			},
		},
		{
			name:    "string height",
			params:  `["first"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid number height",
			params:  `[-2]`,
			fail:    true,
			errCode: neorpc.ErrUnknownHeightCode,
		},
	},
	"getblockheader": {
		{
			name:    "invalid verbose type",
			params:  `["9673799c5b5a294427401cb07d6cc615ada3a0d5c5bf7ed6f0f54f24abb2e2ac", true]`,
			fail:    true,
			errCode: neorpc.ErrUnknownBlockCode,
		},
		{
			name:    "invalid block hash",
			params:  `["notahash"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "unknown block",
			params:  `["a6e526375a780335112299f2262501e5e9574c3ba61b16bbc1e282b344f6c141"]`,
			fail:    true,
			errCode: neorpc.ErrUnknownBlockCode,
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"getblockheadercount": {
		{
			params: "[]",
			result: func(e *executor) any {
				v := int(e.chain.HeaderHeight() + 1)
				return &v
			},
		},
	},
	"getblocksysfee": {
		{
			name:   "positive",
			params: "[1]",
			result: func(e *executor) any {
				block, _ := e.chain.GetBlock(e.chain.GetHeaderHash(1))

				var expectedBlockSysFee int64
				for _, tx := range block.Transactions {
					expectedBlockSysFee += tx.SystemFee
				}
				return &expectedBlockSysFee
			},
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "string height",
			params:  `["first"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid number height",
			params:  `[-2]`,
			fail:    true,
			errCode: neorpc.ErrUnknownHeightCode,
		},
	},
	"getcommittee": {
		{
			params: "[]",
			result: func(e *executor) any {
				expected, _ := e.chain.GetCommittee()
				sort.Sort(expected)
				return &expected
			},
		},
	},
	"getconnectioncount": {
		{
			params: "[]",
			result: func(*executor) any {
				v := 0
				return &v
			},
		},
	},
	"getnativecontracts": {
		{
			params: "[]",
			result: func(e *executor) any {
				return new([]state.NativeContract)
			},
			check: func(t *testing.T, e *executor, res any) {
				lst := res.(*[]state.NativeContract)
				for i := range *lst {
					cs := e.chain.GetContractState((*lst)[i].Hash)
					require.NotNil(t, cs)
					require.True(t, cs.ID <= 0)
					require.Equal(t, []uint32{0}, (*lst)[i].UpdateHistory)
				}
			},
		},
	},
	"getpeers": {
		{
			params: "[]",
			result: func(*executor) any {
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
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid hash",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "missing hash",
			params:  `["` + util.Uint256{}.String() + `"]`,
			fail:    true,
			errCode: neorpc.ErrUnknownTransactionCode,
		},
	},
	"gettransactionheight": {
		{
			name:   "positive",
			params: `["` + deploymentTxHash + `"]`,
			result: func(e *executor) any {
				h := 0
				return &h
			},
			check: func(t *testing.T, e *executor, resp any) {
				h, ok := resp.(*int)
				require.True(t, ok)
				assert.Equal(t, 2, *h)
			},
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid hash",
			params:  `["notahex"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "missing hash",
			params:  `["` + util.Uint256{}.String() + `"]`,
			fail:    true,
			errCode: neorpc.ErrUnknownTransactionCode,
		},
	},
	"getunclaimedgas": {
		{
			name:    "no params",
			params:  "[]",
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid address",
			params:  `["invalid"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:   "positive",
			params: `["` + testchain.MultisigAddress() + `"]`,
			result: func(*executor) any {
				return &result.UnclaimedGas{}
			},
			check: func(t *testing.T, e *executor, resp any) {
				actual, ok := resp.(*result.UnclaimedGas)
				require.True(t, ok)
				expected := result.UnclaimedGas{
					Address:   testchain.MultisigScriptHash(),
					Unclaimed: *big.NewInt(11500),
				}
				assert.Equal(t, expected, *actual)
			},
		},
	},
	"getcandidates": {
		{
			params: "[]",
			result: func(*executor) any {
				return &[]result.Candidate{}
			},
		},
	},
	"getnextblockvalidators": {
		{
			params: "[]",
			result: func(*executor) any {
				return &[]result.Validator{}
			},
			/* preview3 doesn't return any validators until there is a vote
			check: func(t *testing.T, e *executor, validators any) {
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
			result: func(*executor) any { return &result.Version{} },
			check: func(t *testing.T, e *executor, ver any) {
				resp, ok := ver.(*result.Version)
				require.True(t, ok)
				require.Equal(t, "/NEO-GO:0.98.6-test/", resp.UserAgent)

				cfg := e.chain.GetConfig()
				require.EqualValues(t, address.NEO3Prefix, resp.Protocol.AddressVersion)
				require.EqualValues(t, cfg.Magic, resp.Protocol.Network)
				require.EqualValues(t, cfg.TimePerBlock/time.Millisecond, resp.Protocol.MillisecondsPerBlock)
				require.EqualValues(t, cfg.MaxTraceableBlocks, resp.Protocol.MaxTraceableBlocks)
				require.EqualValues(t, cfg.MaxValidUntilBlockIncrement, resp.Protocol.MaxValidUntilBlockIncrement)
				require.EqualValues(t, cfg.MaxTransactionsPerBlock, resp.Protocol.MaxTransactionsPerBlock)
				require.EqualValues(t, cfg.MemPoolSize, resp.Protocol.MemoryPoolMaxTransactions)
				require.EqualValues(t, cfg.ValidatorsCount, resp.Protocol.ValidatorsCount)
				require.EqualValues(t, cfg.InitialGASSupply, resp.Protocol.InitialGasDistribution)

				require.Equal(t, 0, len(resp.Protocol.CommitteeHistory))
				require.True(t, resp.Protocol.P2PSigExtensions) // Yeah, notary is enabled.
				require.False(t, resp.Protocol.StateRootInHeader)
				require.Equal(t, 0, len(resp.Protocol.ValidatorsHistory))
			},
		},
	},
	"invokefunction": {
		{
			name:   "positive",
			params: `["50befd26fdf6e4d957c11e078b24ebce6291456f", "test", []]`,
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.NotNil(t, res.Script)
				assert.NotEqual(t, "", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
			},
		},
		{
			name:   "positive, with notifications",
			params: `["` + nnsContractHash + `", "transfer", [{"type":"Hash160", "value":"0x0bcd2978634d961c24f5aea0802297ff128724d6"},{"type":"String", "value":"neo.com"},{"type":"Any", "value":null}],["0xb248508f4ef7088e10c48f14d04be3272ca29eee"]]`,
			result: func(e *executor) any {
				script := append([]byte{0x0b, 0x0c, 0x07, 0x6e, 0x65, 0x6f, 0x2e, 0x63, 0x6f, 0x6d, 0x0c, 0x14, 0xd6, 0x24, 0x87, 0x12, 0xff, 0x97, 0x22, 0x80, 0xa0, 0xae, 0xf5, 0x24, 0x1c, 0x96, 0x4d, 0x63, 0x78, 0x29, 0xcd, 0xb, 0x13, 0xc0, 0x1f, 0xc, 0x8, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0xc, 0x14}, nnsHash.BytesBE()...)
				script = append(script, 0x41, 0x62, 0x7d, 0x5b, 0x52)
				return &result.Invoke{
					State:       "HALT",
					GasConsumed: 31922970,
					Script:      script,
					Stack:       []stackitem.Item{stackitem.Make(true)},
					Notifications: []state.NotificationEvent{{
						ScriptHash: nnsHash,
						Name:       "Transfer",
						Item: stackitem.NewArray([]stackitem.Item{
							stackitem.Make([]byte{0xee, 0x9e, 0xa2, 0x2c, 0x27, 0xe3, 0x4b, 0xd0, 0x14, 0x8f, 0xc4, 0x10, 0x8e, 0x08, 0xf7, 0x4e, 0x8f, 0x50, 0x48, 0xb2}),
							stackitem.Make([]byte{0xd6, 0x24, 0x87, 0x12, 0xff, 0x97, 0x22, 0x80, 0xa0, 0xae, 0xf5, 0x24, 0x1c, 0x96, 0x4d, 0x63, 0x78, 0x29, 0xcd, 0x0b}),
							stackitem.Make(1),
							stackitem.Make("neo.com"),
						}),
					}},
				}
			},
		},
		{
			name:   "positive, with storage changes",
			params: `["0xef4073a0f2b305a38ec4050e4d3d28bc40ea63f5", "transfer", [{"type":"Hash160", "value":"0xb248508f4ef7088e10c48f14d04be3272ca29eee"},{"type":"Hash160", "value":"0x0bcd2978634d961c24f5aea0802297ff128724d6"},{"type":"Integer", "value":1},{"type":"Any", "value":null}],["0xb248508f4ef7088e10c48f14d04be3272ca29eee"],true]`,
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.NotNil(t, res.Script)
				assert.Equal(t, "HALT", res.State)
				assert.Equal(t, []stackitem.Item{stackitem.Make(true)}, res.Stack)
				assert.NotEqual(t, 0, res.GasConsumed)
				chg := []dboper.Operation{{
					State: "Changed",
					Key:   []byte{0xfa, 0xff, 0xff, 0xff, 0xb},
					Value: []byte{0x54, 0xb2, 0xd2, 0xa3, 0x51, 0x79, 0x12},
				}, {
					State: "Added",
					Key:   []byte{0xfb, 0xff, 0xff, 0xff, 0x14, 0xd6, 0x24, 0x87, 0x12, 0xff, 0x97, 0x22, 0x80, 0xa0, 0xae, 0xf5, 0x24, 0x1c, 0x96, 0x4d, 0x63, 0x78, 0x29, 0xcd, 0xb},
					Value: []byte{0x41, 0x04, 0x21, 0x01, 0x01, 0x21, 0x01, 0x18, 0x00, 0x21, 0x00},
				}, {
					State: "Changed",
					Key:   []byte{0xfb, 0xff, 0xff, 0xff, 0x14, 0xee, 0x9e, 0xa2, 0x2c, 0x27, 0xe3, 0x4b, 0xd0, 0x14, 0x8f, 0xc4, 0x10, 0x8e, 0x8, 0xf7, 0x4e, 0x8f, 0x50, 0x48, 0xb2},
					Value: []byte{0x41, 0x04, 0x21, 0x04, 0x2f, 0xd9, 0xf5, 0x05, 0x21, 0x01, 0x18, 0x00, 0x21, 0x00},
				}, {
					State: "Changed",
					Key:   []byte{0xfa, 0xff, 0xff, 0xff, 0x14, 0xee, 0x9e, 0xa2, 0x2c, 0x27, 0xe3, 0x4b, 0xd0, 0x14, 0x8f, 0xc4, 0x10, 0x8e, 0x8, 0xf7, 0x4e, 0x8f, 0x50, 0x48, 0xb2},
					Value: []byte{0x41, 0x01, 0x21, 0x05, 0x0c, 0x76, 0x4f, 0xdf, 0x08},
				}}
				// Can be returned in any order.
				assert.ElementsMatch(t, chg, res.Diagnostics.Changes)
			},
		},
		{
			name:   "positive, verbose",
			params: `["` + nnsContractHash + `", "resolve", [{"type":"String", "value":"neo.com"},{"type":"Integer","value":1}], [], true]`,
			result: func(e *executor) any {
				script := append([]byte{0x11, 0xc, 0x7, 0x6e, 0x65, 0x6f, 0x2e, 0x63, 0x6f, 0x6d, 0x12, 0xc0, 0x1f, 0xc, 0x7, 0x72, 0x65, 0x73, 0x6f, 0x6c, 0x76, 0x65, 0xc, 0x14}, nnsHash.BytesBE()...)
				script = append(script, 0x41, 0x62, 0x7d, 0x5b, 0x52)
				stdHash, _ := e.chain.GetNativeContractScriptHash(nativenames.StdLib)
				cryptoHash, _ := e.chain.GetNativeContractScriptHash(nativenames.CryptoLib)
				return &result.Invoke{
					State:         "HALT",
					GasConsumed:   13970250,
					Script:        script,
					Stack:         []stackitem.Item{stackitem.Make("1.2.3.4")},
					Notifications: []state.NotificationEvent{},
					Diagnostics: &result.InvokeDiag{
						Changes: []dboper.Operation{},
						Invocations: []*invocations.Tree{{
							Current: hash.Hash160(script),
							Calls: []*invocations.Tree{
								{
									Current: nnsHash,
									Calls: []*invocations.Tree{
										{
											Current: stdHash,
										},
										{
											Current: cryptoHash,
										},
										{
											Current: stdHash,
										},
										{
											Current: cryptoHash,
										},
										{
											Current: cryptoHash,
										},
									},
								},
							},
						}},
					},
				}
			},
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "not a string",
			params:  `[42, "test", []]`,
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
		{
			name:    "not a scripthash",
			params:  `["qwerty", "test", []]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bad params",
			params:  `["50befd26fdf6e4d957c11e078b24ebce6291456f", "test", [{"type": "Integer", "value": "qwerty"}]]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"invokefunctionhistoric": {
		{
			name:   "positive, by index",
			params: `[20, "50befd26fdf6e4d957c11e078b24ebce6291456f", "test", []]`,
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.NotNil(t, res.Script)
				assert.NotEqual(t, "", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
			},
		},
		{
			name:   "positive, by stateroot",
			params: `["` + block20StateRootLE + `", "50befd26fdf6e4d957c11e078b24ebce6291456f", "test", []]`,
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.NotNil(t, res.Script)
				assert.NotEqual(t, "", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
			},
		},
		{
			name:   "positive, with notifications",
			params: `[20, "` + nnsContractHash + `", "transfer", [{"type":"Hash160", "value":"0x0bcd2978634d961c24f5aea0802297ff128724d6"},{"type":"String", "value":"neo.com"},{"type":"Any", "value":null}],["0xb248508f4ef7088e10c48f14d04be3272ca29eee"]]`,
			result: func(e *executor) any {
				script := append([]byte{0x0b, 0x0c, 0x07, 0x6e, 0x65, 0x6f, 0x2e, 0x63, 0x6f, 0x6d, 0x0c, 0x14, 0xd6, 0x24, 0x87, 0x12, 0xff, 0x97, 0x22, 0x80, 0xa0, 0xae, 0xf5, 0x24, 0x1c, 0x96, 0x4d, 0x63, 0x78, 0x29, 0xcd, 0xb, 0x13, 0xc0, 0x1f, 0xc, 0x8, 0x74, 0x72, 0x61, 0x6e, 0x73, 0x66, 0x65, 0x72, 0xc, 0x14}, nnsHash.BytesBE()...)
				script = append(script, 0x41, 0x62, 0x7d, 0x5b, 0x52)
				return &result.Invoke{
					State:       "HALT",
					GasConsumed: 31922970,
					Script:      script,
					Stack:       []stackitem.Item{stackitem.Make(true)},
					Notifications: []state.NotificationEvent{{
						ScriptHash: nnsHash,
						Name:       "Transfer",
						Item: stackitem.NewArray([]stackitem.Item{
							stackitem.Make([]byte{0xee, 0x9e, 0xa2, 0x2c, 0x27, 0xe3, 0x4b, 0xd0, 0x14, 0x8f, 0xc4, 0x10, 0x8e, 0x08, 0xf7, 0x4e, 0x8f, 0x50, 0x48, 0xb2}),
							stackitem.Make([]byte{0xd6, 0x24, 0x87, 0x12, 0xff, 0x97, 0x22, 0x80, 0xa0, 0xae, 0xf5, 0x24, 0x1c, 0x96, 0x4d, 0x63, 0x78, 0x29, 0xcd, 0x0b}),
							stackitem.Make(1),
							stackitem.Make("neo.com"),
						}),
					}},
				}
			},
		},
		{
			name:   "positive, verbose",
			params: `[20, "` + nnsContractHash + `", "resolve", [{"type":"String", "value":"neo.com"},{"type":"Integer","value":1}], [], true]`,
			result: func(e *executor) any {
				script := append([]byte{0x11, 0xc, 0x7, 0x6e, 0x65, 0x6f, 0x2e, 0x63, 0x6f, 0x6d, 0x12, 0xc0, 0x1f, 0xc, 0x7, 0x72, 0x65, 0x73, 0x6f, 0x6c, 0x76, 0x65, 0xc, 0x14}, nnsHash.BytesBE()...)
				script = append(script, 0x41, 0x62, 0x7d, 0x5b, 0x52)
				stdHash, _ := e.chain.GetNativeContractScriptHash(nativenames.StdLib)
				cryptoHash, _ := e.chain.GetNativeContractScriptHash(nativenames.CryptoLib)
				return &result.Invoke{
					State:         "HALT",
					GasConsumed:   13970250,
					Script:        script,
					Stack:         []stackitem.Item{stackitem.Make("1.2.3.4")},
					Notifications: []state.NotificationEvent{},
					Diagnostics: &result.InvokeDiag{
						Changes: []dboper.Operation{},
						Invocations: []*invocations.Tree{{
							Current: hash.Hash160(script),
							Calls: []*invocations.Tree{
								{
									Current: nnsHash,
									Calls: []*invocations.Tree{
										{
											Current: stdHash,
										},
										{
											Current: cryptoHash,
										},
										{
											Current: stdHash,
										},
										{
											Current: cryptoHash,
										},
										{
											Current: cryptoHash,
										},
									},
								},
							},
						}},
					},
				}
			},
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "no args",
			params:  `[20]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "not a string",
			params:  `[20, 42, "test", []]`,
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
		{
			name:    "not a scripthash",
			params:  `[20,"qwerty", "test", []]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bad params",
			params:  `[20,"50befd26fdf6e4d957c11e078b24ebce6291456f", "test", [{"type": "Integer", "value": "qwerty"}]]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bad height",
			params:  `[100500,"50befd26fdf6e4d957c11e078b24ebce6291456f", "test", [{"type": "Integer", "value": 1}]]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bad stateroot",
			params:  `["` + util.Uint256{1, 2, 3}.StringLE() + `","50befd26fdf6e4d957c11e078b24ebce6291456f", "test", [{"type": "Integer", "value": 1}]]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"invokescript": {
		{
			name:   "positive",
			params: `["UcVrDUhlbGxvLCB3b3JsZCFoD05lby5SdW50aW1lLkxvZ2FsdWY="]`,
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.NotEqual(t, "", res.Script)
				assert.NotEqual(t, "", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
			},
		},
		{
			name:   "positive,verbose",
			params: `["UcVrDUhlbGxvLCB3b3JsZCFoD05lby5SdW50aW1lLkxvZ2FsdWY=",[],true]`,
			result: func(e *executor) any {
				script := []byte{0x51, 0xc5, 0x6b, 0xd, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64, 0x21, 0x68, 0xf, 0x4e, 0x65, 0x6f, 0x2e, 0x52, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2e, 0x4c, 0x6f, 0x67, 0x61, 0x6c, 0x75, 0x66}
				return &result.Invoke{
					State:          "FAULT",
					GasConsumed:    60,
					Script:         script,
					Stack:          []stackitem.Item{},
					FaultException: "at instruction 0 (ROT): too big index",
					Notifications:  []state.NotificationEvent{},
					Diagnostics: &result.InvokeDiag{
						Changes: []dboper.Operation{},
						Invocations: []*invocations.Tree{{
							Current: hash.Hash160(script),
						}},
					},
				}
			},
		},
		{
			name: "positive, good witness",
			// script is base64-encoded `invokescript_contract.avm` representation, hashes are hex-encoded LE bytes of hashes used in the contract with `0x` prefix
			params: fmt.Sprintf(`["%s",["0x0000000009070e030d0f0e020d0c06050e030c01","0x090c060e00010205040307030102000902030f0d"]]`, invokescriptContractAVM),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				require.Equal(t, 1, len(res.Stack))
				require.Equal(t, big.NewInt(3), res.Stack[0].Value())
			},
		},
		{
			name:   "positive, bad witness of second hash",
			params: fmt.Sprintf(`["%s",["0x0000000009070e030d0f0e020d0c06050e030c01"]]`, invokescriptContractAVM),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				require.Equal(t, 1, len(res.Stack))
				require.Equal(t, big.NewInt(2), res.Stack[0].Value())
			},
		},
		{
			name:   "positive, no good hashes",
			params: fmt.Sprintf(`["%s"]`, invokescriptContractAVM),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				require.Equal(t, 1, len(res.Stack))
				require.Equal(t, big.NewInt(1), res.Stack[0].Value())
			},
		},
		{
			name:   "positive, bad hashes witness",
			params: fmt.Sprintf(`["%s",["0x0000000009070e030d0f0e020d0c06050e030c02"]]`, invokescriptContractAVM),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				assert.Equal(t, 1, len(res.Stack))
				assert.Equal(t, big.NewInt(1), res.Stack[0].Value())
			},
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "not a string",
			params:  `[42]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bas string",
			params:  `["qwerty"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"invokescripthistoric": {
		{
			name:   "positive, by index",
			params: `[20,"UcVrDUhlbGxvLCB3b3JsZCFoD05lby5SdW50aW1lLkxvZ2FsdWY="]`,
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.NotEqual(t, "", res.Script)
				assert.NotEqual(t, "", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
			},
		},
		{
			name:   "positive, by stateroot",
			params: `["` + block20StateRootLE + `","UcVrDUhlbGxvLCB3b3JsZCFoD05lby5SdW50aW1lLkxvZ2FsdWY="]`,
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.NotEqual(t, "", res.Script)
				assert.NotEqual(t, "", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
			},
		},
		{
			name:   "positive,verbose",
			params: `[20, "UcVrDUhlbGxvLCB3b3JsZCFoD05lby5SdW50aW1lLkxvZ2FsdWY=",[],true]`,
			result: func(e *executor) any {
				script := []byte{0x51, 0xc5, 0x6b, 0xd, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64, 0x21, 0x68, 0xf, 0x4e, 0x65, 0x6f, 0x2e, 0x52, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2e, 0x4c, 0x6f, 0x67, 0x61, 0x6c, 0x75, 0x66}
				return &result.Invoke{
					State:          "FAULT",
					GasConsumed:    60,
					Script:         script,
					Stack:          []stackitem.Item{},
					FaultException: "at instruction 0 (ROT): too big index",
					Notifications:  []state.NotificationEvent{},
					Diagnostics: &result.InvokeDiag{
						Changes: []dboper.Operation{},
						Invocations: []*invocations.Tree{{
							Current: hash.Hash160(script),
						}},
					},
				}
			},
		},
		{
			name: "positive, good witness",
			// script is base64-encoded `invokescript_contract.avm` representation, hashes are hex-encoded LE bytes of hashes used in the contract with `0x` prefix
			params: fmt.Sprintf(`[20,"%s",["0x0000000009070e030d0f0e020d0c06050e030c01","0x090c060e00010205040307030102000902030f0d"]]`, invokescriptContractAVM),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				require.Equal(t, 1, len(res.Stack))
				require.Equal(t, big.NewInt(3), res.Stack[0].Value())
			},
		},
		{
			name:   "positive, bad witness of second hash",
			params: fmt.Sprintf(`[20,"%s",["0x0000000009070e030d0f0e020d0c06050e030c01"]]`, invokescriptContractAVM),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				require.Equal(t, 1, len(res.Stack))
				require.Equal(t, big.NewInt(2), res.Stack[0].Value())
			},
		},
		{
			name:   "positive, no good hashes",
			params: fmt.Sprintf(`[20,"%s"]`, invokescriptContractAVM),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				require.Equal(t, 1, len(res.Stack))
				require.Equal(t, big.NewInt(1), res.Stack[0].Value())
			},
		},
		{
			name:   "positive, bad hashes witness",
			params: fmt.Sprintf(`[20,"%s",["0x0000000009070e030d0f0e020d0c06050e030c02"]]`, invokescriptContractAVM),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Equal(t, "HALT", res.State)
				assert.Equal(t, 1, len(res.Stack))
				assert.Equal(t, big.NewInt(1), res.Stack[0].Value())
			},
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "no script",
			params:  `[20]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "not a string",
			params:  `[20,42]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bad string",
			params:  `[20, "qwerty"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bad height",
			params:  `[100500,"qwerty"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "bad stateroot",
			params:  `["` + util.Uint256{1, 2, 3}.StringLE() + `","UcVrDUhlbGxvLCB3b3JsZCFoD05lby5SdW50aW1lLkxvZ2FsdWY="]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"invokecontractverify": {
		{
			name:   "positive",
			params: fmt.Sprintf(`["%s", [], [{"account":"%s"}]]`, verifyContractHash, testchain.PrivateKeyByID(0).PublicKey().GetScriptHash().StringLE()),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Nil(t, res.Script) // empty witness invocation script (pushes args of `verify` on stack, but this `verify` don't have args)
				assert.Equal(t, "HALT", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, true, res.Stack[0].Value().(bool), fmt.Sprintf("check address in verification_contract.go: expected %s", testchain.PrivateKeyByID(0).Address()))
			},
		},
		{
			name:   "positive, no signers",
			params: fmt.Sprintf(`["%s", []]`, verifyContractHash),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Nil(t, res.Script)
				assert.Equal(t, "HALT", res.State, res.FaultException)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, false, res.Stack[0].Value().(bool))
			},
		},
		{
			name:   "positive, no arguments",
			params: fmt.Sprintf(`["%s"]`, verifyContractHash),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Nil(t, res.Script)
				assert.Equal(t, "HALT", res.State, res.FaultException)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, false, res.Stack[0].Value().(bool))
			},
		},
		{
			name:   "positive, with signers and scripts",
			params: fmt.Sprintf(`["%s", [], [{"account":"%s", "invocation":"MQo=", "verification": ""}]]`, verifyContractHash, testchain.PrivateKeyByID(0).PublicKey().GetScriptHash().StringLE()),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Nil(t, res.Script)
				assert.Equal(t, "HALT", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, true, res.Stack[0].Value().(bool))
			},
		},
		{
			name:   "positive, with arguments, result=true",
			params: fmt.Sprintf(`["%s", [{"type": "String", "value": "good_string"}, {"type": "Integer", "value": "4"}, {"type":"Boolean", "value": false}]]`, verifyWithArgsContractHash),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				expectedInvScript := io.NewBufBinWriter()
				emit.Bool(expectedInvScript.BinWriter, false)
				emit.Int(expectedInvScript.BinWriter, int64(4))
				emit.String(expectedInvScript.BinWriter, "good_string")
				require.NoError(t, expectedInvScript.Err)
				assert.Equal(t, expectedInvScript.Bytes(), res.Script) // witness invocation script (pushes args of `verify` on stack)
				assert.Equal(t, "HALT", res.State, res.FaultException)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, true, res.Stack[0].Value().(bool))
			},
		},
		{
			name:   "positive, with arguments, result=false",
			params: fmt.Sprintf(`["%s", [{"type": "String", "value": "invalid_string"}, {"type": "Integer", "value": "4"}, {"type":"Boolean", "value": false}]]`, verifyWithArgsContractHash),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				expectedInvScript := io.NewBufBinWriter()
				emit.Bool(expectedInvScript.BinWriter, false)
				emit.Int(expectedInvScript.BinWriter, int64(4))
				emit.String(expectedInvScript.BinWriter, "invalid_string")
				require.NoError(t, expectedInvScript.Err)
				assert.Equal(t, expectedInvScript.Bytes(), res.Script)
				assert.Equal(t, "HALT", res.State, res.FaultException)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, false, res.Stack[0].Value().(bool))
			},
		},
		{
			name:    "invalid call args",
			params:  fmt.Sprintf(`["%s", [{"type":"Map","value":{"key":"value"}}]]`, verifyWithArgsContractHash),
			fail:    true,
			errCode: neorpc.InternalServerErrorCode,
		},
		{
			name:    "negative, wrong signer",
			params:  fmt.Sprintf(`["%s", [], [{"account":"aaa"}]]`, verifyContractHash),
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "unknown contract",
			params:  fmt.Sprintf(`["%s", []]`, util.Uint160{}.String()),
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "not a string",
			params:  `[42, []]`,
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
	},
	"invokecontractverifyhistoric": {
		{
			name:   "positive, by index",
			params: fmt.Sprintf(`[20,"%s", [], [{"account":"%s"}]]`, verifyContractHash, testchain.PrivateKeyByID(0).PublicKey().GetScriptHash().StringLE()),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Nil(t, res.Script) // empty witness invocation script (pushes args of `verify` on stack, but this `verify` don't have args)
				assert.Equal(t, "HALT", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, true, res.Stack[0].Value().(bool), fmt.Sprintf("check address in verification_contract.go: expected %s", testchain.PrivateKeyByID(0).Address()))
			},
		},
		{
			name:   "positive, by stateroot",
			params: fmt.Sprintf(`["`+block20StateRootLE+`","%s", [], [{"account":"%s"}]]`, verifyContractHash, testchain.PrivateKeyByID(0).PublicKey().GetScriptHash().StringLE()),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Nil(t, res.Script) // empty witness invocation script (pushes args of `verify` on stack, but this `verify` don't have args)
				assert.Equal(t, "HALT", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, true, res.Stack[0].Value().(bool), fmt.Sprintf("check address in verification_contract.go: expected %s", testchain.PrivateKeyByID(0).Address()))
			},
		},
		{
			name:   "positive, no signers",
			params: fmt.Sprintf(`[20,"%s", []]`, verifyContractHash),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Nil(t, res.Script)
				assert.Equal(t, "HALT", res.State, res.FaultException)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, false, res.Stack[0].Value().(bool))
			},
		},
		{
			name:   "positive, no arguments",
			params: fmt.Sprintf(`[20,"%s"]`, verifyContractHash),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Nil(t, res.Script)
				assert.Equal(t, "HALT", res.State, res.FaultException)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, false, res.Stack[0].Value().(bool))
			},
		},
		{
			name:   "positive, with signers and scripts",
			params: fmt.Sprintf(`[20,"%s", [], [{"account":"%s", "invocation":"MQo=", "verification": ""}]]`, verifyContractHash, testchain.PrivateKeyByID(0).PublicKey().GetScriptHash().StringLE()),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				assert.Nil(t, res.Script)
				assert.Equal(t, "HALT", res.State)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, true, res.Stack[0].Value().(bool))
			},
		},
		{
			name:   "positive, with arguments, result=true",
			params: fmt.Sprintf(`[20,"%s", [{"type": "String", "value": "good_string"}, {"type": "Integer", "value": "4"}, {"type":"Boolean", "value": false}]]`, verifyWithArgsContractHash),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				expectedInvScript := io.NewBufBinWriter()
				emit.Bool(expectedInvScript.BinWriter, false)
				emit.Int(expectedInvScript.BinWriter, int64(4))
				emit.String(expectedInvScript.BinWriter, "good_string")
				require.NoError(t, expectedInvScript.Err)
				assert.Equal(t, expectedInvScript.Bytes(), res.Script) // witness invocation script (pushes args of `verify` on stack)
				assert.Equal(t, "HALT", res.State, res.FaultException)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, true, res.Stack[0].Value().(bool))
			},
		},
		{
			name:   "positive, with arguments, result=false",
			params: fmt.Sprintf(`[20, "%s", [{"type": "String", "value": "invalid_string"}, {"type": "Integer", "value": "4"}, {"type":"Boolean", "value": false}]]`, verifyWithArgsContractHash),
			result: func(e *executor) any { return &result.Invoke{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.Invoke)
				require.True(t, ok)
				expectedInvScript := io.NewBufBinWriter()
				emit.Bool(expectedInvScript.BinWriter, false)
				emit.Int(expectedInvScript.BinWriter, int64(4))
				emit.String(expectedInvScript.BinWriter, "invalid_string")
				require.NoError(t, expectedInvScript.Err)
				assert.Equal(t, expectedInvScript.Bytes(), res.Script)
				assert.Equal(t, "HALT", res.State, res.FaultException)
				assert.NotEqual(t, 0, res.GasConsumed)
				assert.Equal(t, false, res.Stack[0].Value().(bool))
			},
		},
		{
			name:    "unknown contract",
			params:  fmt.Sprintf(`[20, "%s", []]`, util.Uint160{}.String()),
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "no args",
			params:  `[20]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "not a string",
			params:  `[20,42, []]`,
			fail:    true,
			errCode: neorpc.ErrUnknownContractCode,
		},
	},
	"sendrawtransaction": {
		{
			name:   "positive",
			params: `["AB0AAACWP5gAAAAAAEDaEgAAAAAAGAAAAAHunqIsJ+NL0BSPxBCOCPdOj1BIsoAAXgsDAOh2SBcAAAAMFBEmW7QXJQBBvgTo+iQOOPV8HlabDBTunqIsJ+NL0BSPxBCOCPdOj1BIshTAHwwIdHJhbnNmZXIMFPVj6kC8KD1NDgXEjqMFs/Kgc0DvQWJ9W1IBQgxAJ6norhWoZxp+Hj1JFhi+Z3qI9DUkLSbfsbaLSaJIqxTfdmPbNFDVK1G+oa+LWmpRp/bj9+QZM7yC+S6HXUI7rigMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwkFW57Mn"]`,
			result: func(e *executor) any { return &result.RelayResult{} },
			check: func(t *testing.T, e *executor, inv any) {
				res, ok := inv.(*result.RelayResult)
				require.True(t, ok)
				expectedHash := "c11861dec1dd0f188608b725095041fcfc90abe51eea044993f122f22472753e"
				assert.Equal(t, expectedHash, res.Hash.StringLE())
			},
		},
		{
			name:    "already in pool",
			params:  `["AB0AAACWP5gAAAAAAEDaEgAAAAAAGAAAAAHunqIsJ+NL0BSPxBCOCPdOj1BIsoAAXgsDAOh2SBcAAAAMFBEmW7QXJQBBvgTo+iQOOPV8HlabDBTunqIsJ+NL0BSPxBCOCPdOj1BIshTAHwwIdHJhbnNmZXIMFPVj6kC8KD1NDgXEjqMFs/Kgc0DvQWJ9W1IBQgxAJ6norhWoZxp+Hj1JFhi+Z3qI9DUkLSbfsbaLSaJIqxTfdmPbNFDVK1G+oa+LWmpRp/bj9+QZM7yC+S6HXUI7rigMIQKzYiv0AXvf4xfFiu1fTHU/IGt9uJYEb6fXdLvEv3+NwkFW57Mn"]`,
			fail:    true,
			errCode: neorpc.ErrAlreadyInPoolCode,
		},
		{
			name:    "negative",
			params:  `["AAoAAAAxboUQOQGdOd/Cw31sP+4Z/VgJhwAAAAAAAAAA8q0FAAAAAACwBAAAAAExboUQOQGdOd/Cw31sP+4Z/VgJhwFdAwDodkgXAAAADBQgcoJ0r6/Db0OgcdMoz6PmKdnLsAwUMW6FEDkBnTnfwsN9bD/uGf1YCYcTwAwIdHJhbnNmZXIMFIl3INjNdvTwCr+jfA7diJwgj96bQWJ9W1I4AUIMQN+VMUEnEWlCHOurXSegFj4pTXx/LQUltEmHRTRIFP09bFxZHJsXI9BdQoVvQJrbCEz2esySHPr8YpEzpeteen4pDCECs2Ir9AF73+MXxYrtX0x1PyBrfbiWBG+n13S7xL9/jcILQQqQav8="]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid string",
			params:  `["notabase64%"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid tx",
			params:  `["AnTXkgcmF3IGNvbnRyYWNw=="]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"submitblock": {
		{
			name:    "invalid base64",
			params:  `["%%%"]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "invalid block bytes",
			params:  `["AAAAACc="]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"submitoracleresponse": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.ErrOracleDisabledCode,
		},
	},
	"submitnotaryrequest": {
		{
			name:    "no params",
			params:  `[]`,
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
		},
	},
	"validateaddress": {
		{
			name:   "positive",
			params: `["Nbb1qkwcwNSBs9pAnrVVrnFbWnbWBk91U2"]`,
			result: func(*executor) any { return &result.ValidateAddress{} },
			check: func(t *testing.T, e *executor, va any) {
				res, ok := va.(*result.ValidateAddress)
				require.True(t, ok)
				assert.Equal(t, "Nbb1qkwcwNSBs9pAnrVVrnFbWnbWBk91U2", res.Address)
				assert.True(t, res.IsValid)
			},
		},
		{
			name:   "negative",
			params: "[1]",
			result: func(*executor) any {
				return &result.ValidateAddress{
					Address: float64(1),
					IsValid: false,
				}
			},
		},
		{
			name:    "no params",
			params:  "[]",
			fail:    true,
			errCode: neorpc.InvalidParamsCode,
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

func TestSubmitOracle(t *testing.T) {
	rpc := `{"jsonrpc": "2.0", "id": 1, "method": "submitoracleresponse", "params": %s}`

	t.Run("OracleDisabled", func(t *testing.T) {
		chain, rpcSrv, httpSrv := initClearServerWithCustomConfig(t, func(c *config.Config) {
			c.ApplicationConfiguration.Oracle.Enabled = false
		})
		defer chain.Close()
		defer rpcSrv.Shutdown()
		req := fmt.Sprintf(rpc, "[]")
		body := doRPCCallOverHTTP(req, httpSrv.URL, t)
		checkErrGetResult(t, body, true, neorpc.ErrOracleDisabledCode)
	})

	chain, rpcSrv, httpSrv := initClearServerWithServices(t, true, false, false)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	runCase := func(t *testing.T, fail bool, errCode int64, params ...string) func(t *testing.T) {
		return func(t *testing.T) {
			ps := `[` + strings.Join(params, ",") + `]`
			req := fmt.Sprintf(rpc, ps)
			body := doRPCCallOverHTTP(req, httpSrv.URL, t)
			checkErrGetResult(t, body, fail, errCode)
		}
	}
	t.Run("MissingKey", runCase(t, true, neorpc.InvalidParamsCode))
	t.Run("InvalidKey", runCase(t, true, neorpc.InvalidParamsCode, `"1234"`))

	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pubStr := `"` + base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()) + `"`
	t.Run("InvalidReqID", runCase(t, true, neorpc.InvalidParamsCode, pubStr, `"notanumber"`))
	t.Run("InvalidTxSignature", runCase(t, true, neorpc.InvalidParamsCode, pubStr, `1`, `"qwerty"`))

	txSig := priv.Sign([]byte{1, 2, 3})
	txSigStr := `"` + base64.StdEncoding.EncodeToString(txSig) + `"`
	t.Run("MissingMsgSignature", runCase(t, true, neorpc.InvalidParamsCode, pubStr, `1`, txSigStr))
	t.Run("InvalidMsgSignature", runCase(t, true, neorpc.ErrInvalidSignatureCode, pubStr, `1`, txSigStr, `"0123"`))

	msg := rpc2.GetMessage(priv.PublicKey().Bytes(), 1, txSig)
	msgSigStr := `"` + base64.StdEncoding.EncodeToString(priv.Sign(msg)) + `"`
	t.Run("Valid", runCase(t, false, 0, pubStr, `1`, txSigStr, msgSigStr))
}

func TestSubmitNotaryRequest(t *testing.T) {
	rpc := `{"jsonrpc": "2.0", "id": 1, "method": "submitnotaryrequest", "params": %s}`

	t.Run("disabled P2PSigExtensions", func(t *testing.T) {
		chain, rpcSrv, httpSrv := initClearServerWithCustomConfig(t, func(c *config.Config) {
			c.ProtocolConfiguration.P2PSigExtensions = false
		})
		defer chain.Close()
		defer rpcSrv.Shutdown()
		req := fmt.Sprintf(rpc, "[]")
		body := doRPCCallOverHTTP(req, httpSrv.URL, t)
		checkErrGetResult(t, body, true, neorpc.InternalServerErrorCode)
	})

	chain, rpcSrv, httpSrv := initServerWithInMemoryChainAndServices(t, false, true, false)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	runCase := func(t *testing.T, fail bool, errCode int64, params ...string) func(t *testing.T) {
		return func(t *testing.T) {
			ps := `[` + strings.Join(params, ",") + `]`
			req := fmt.Sprintf(rpc, ps)
			body := doRPCCallOverHTTP(req, httpSrv.URL, t)
			checkErrGetResult(t, body, fail, errCode)
		}
	}
	t.Run("missing request", runCase(t, true, neorpc.InvalidParamsCode))
	t.Run("not a base64", runCase(t, true, neorpc.InvalidParamsCode, `"not-a-base64$"`))
	t.Run("invalid request bytes", runCase(t, true, neorpc.InvalidParamsCode, `"not-a-request"`))
	t.Run("invalid request", func(t *testing.T) {
		mainTx := &transaction.Transaction{
			Attributes:      []transaction.Attribute{{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 1}}},
			Script:          []byte{byte(opcode.RET)},
			ValidUntilBlock: 123,
			Signers:         []transaction.Signer{{Account: util.Uint160{1, 5, 9}}},
			Scripts: []transaction.Witness{{
				InvocationScript:   []byte{1, 4, 7},
				VerificationScript: []byte{3, 6, 9},
			}},
		}
		fallbackTx := &transaction.Transaction{
			Script:          []byte{byte(opcode.RET)},
			ValidUntilBlock: 123,
			Attributes: []transaction.Attribute{
				{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: 123}},
				{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: mainTx.Hash()}},
				{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 0}},
			},
			Signers: []transaction.Signer{{Account: util.Uint160{1, 4, 7}}, {Account: util.Uint160{9, 8, 7}}},
			Scripts: []transaction.Witness{
				{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, make([]byte, keys.SignatureLen)...), VerificationScript: make([]byte, 0)},
				{InvocationScript: []byte{1, 2, 3}, VerificationScript: []byte{1, 2, 3}}},
		}
		p := &payload.P2PNotaryRequest{
			MainTransaction:     mainTx,
			FallbackTransaction: fallbackTx,
			Witness: transaction.Witness{
				InvocationScript:   []byte{1, 2, 3},
				VerificationScript: []byte{7, 8, 9},
			},
		}
		bytes, err := p.Bytes()
		require.NoError(t, err)
		str := fmt.Sprintf(`"%s"`, base64.StdEncoding.EncodeToString(bytes))
		runCase(t, true, neorpc.ErrVerificationFailedCode, str)(t)
	})
	t.Run("valid request", func(t *testing.T) {
		sender := testchain.PrivateKeyByID(0) // owner of the deposit in testchain
		p := createValidNotaryRequest(chain, sender, 1)
		bytes, err := p.Bytes()
		require.NoError(t, err)
		str := fmt.Sprintf(`"%s"`, base64.StdEncoding.EncodeToString(bytes))
		runCase(t, false, 0, str)(t)
	})
}

// createValidNotaryRequest creates and signs P2PNotaryRequest payload which can
// pass verification.
func createValidNotaryRequest(chain *core.Blockchain, sender *keys.PrivateKey, nonce uint32) *payload.P2PNotaryRequest {
	h := chain.BlockHeight()
	mainTx := &transaction.Transaction{
		Nonce:           nonce,
		Attributes:      []transaction.Attribute{{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 1}}},
		Script:          []byte{byte(opcode.RET)},
		ValidUntilBlock: h + 100,
		Signers:         []transaction.Signer{{Account: sender.GetScriptHash()}},
		Scripts: []transaction.Witness{{
			InvocationScript:   []byte{1, 4, 7},
			VerificationScript: []byte{3, 6, 9},
		}},
	}
	fallbackTx := &transaction.Transaction{
		Script:          []byte{byte(opcode.RET)},
		ValidUntilBlock: h + 100,
		Attributes: []transaction.Attribute{
			{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: h + 50}},
			{Type: transaction.ConflictsT, Value: &transaction.Conflicts{Hash: mainTx.Hash()}},
			{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 0}},
		},
		Signers: []transaction.Signer{{Account: chain.GetNotaryContractScriptHash()}, {Account: sender.GetScriptHash()}},
		Scripts: []transaction.Witness{
			{InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, make([]byte, keys.SignatureLen)...), VerificationScript: []byte{}},
		},
		NetworkFee: 2_0000_0000,
	}
	fallbackTx.Scripts = append(fallbackTx.Scripts, transaction.Witness{
		InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, sender.SignHashable(uint32(testchain.Network()), fallbackTx)...),
		VerificationScript: sender.PublicKey().GetVerificationScript(),
	})
	p := &payload.P2PNotaryRequest{
		MainTransaction:     mainTx,
		FallbackTransaction: fallbackTx,
	}
	p.Witness = transaction.Witness{
		InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, sender.SignHashable(uint32(testchain.Network()), p)...),
		VerificationScript: sender.PublicKey().GetVerificationScript(),
	}
	return p
}

func runTestCasesWithExecutor(t *testing.T, e *executor, rpcCall string, method string, testCases []rpcTestCase, doRPCCall func(string, string, *testing.T) []byte, checkErrResult func(t *testing.T, body []byte, expectingFail bool, expectedErrCode int64, expectedErr ...string) json.RawMessage) {
	t.Run(method, func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				body := doRPCCall(fmt.Sprintf(rpcCall, method, tc.params), e.httpSrv.URL, t)
				result := checkErrResult(t, body, tc.fail, tc.errCode)
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

// testRPCProtocol runs a full set of tests using given callback to make actual
// calls. Some tests change the chain state, thus we reinitialize the chain from
// scratch here.
func testRPCProtocol(t *testing.T, doRPCCall func(string, string, *testing.T) []byte) {
	chain, rpcSrv, httpSrv := initServerWithInMemoryChain(t)

	defer chain.Close()
	defer rpcSrv.Shutdown()

	e := &executor{chain: chain, httpSrv: httpSrv}
	t.Run("single request", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "%s", "params": %s}`
		for method, cases := range rpcTestCases {
			runTestCasesWithExecutor(t, e, rpc, method, cases, doRPCCall, checkErrGetResult)
		}
	})
	t.Run("batch with single request", func(t *testing.T) {
		for method, cases := range rpcTestCases {
			if method == "sendrawtransaction" {
				continue // cannot send the same transaction twice
			}
			rpc := `[{"jsonrpc": "2.0", "id": 1, "method": "%s", "params": %s}]`
			runTestCasesWithExecutor(t, e, rpc, method, cases, doRPCCall, checkErrGetBatchResult)
		}
	})

	t.Run("batch with multiple requests", func(t *testing.T) {
		for method, cases := range rpcTestCases {
			if method == "sendrawtransaction" {
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
				var responses []neorpc.Response
				err := json.Unmarshal(body, &responses)
				require.Nil(t, err)
				for i, tc := range cases {
					var resp neorpc.Response
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

	t.Run("getapplicationlog for block", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getapplicationlog", "params": ["%s"]}`
		body := doRPCCall(fmt.Sprintf(rpc, e.chain.GetHeaderHash(1).StringLE()), httpSrv.URL, t)
		data := checkErrGetResult(t, body, false, 0)
		var res result.ApplicationLog
		require.NoError(t, json.Unmarshal(data, &res))
		require.Equal(t, 2, len(res.Executions))
		require.Equal(t, trigger.OnPersist, res.Executions[0].Trigger)
		require.Equal(t, vmstate.Halt, res.Executions[0].VMState)
		require.Equal(t, trigger.PostPersist, res.Executions[1].Trigger)
		require.Equal(t, vmstate.Halt, res.Executions[1].VMState)
	})
	t.Run("submitblock", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "submitblock", "params": ["%s"]}`
		t.Run("invalid signature", func(t *testing.T) {
			s := testchain.NewBlock(t, chain, 1, 0)
			s.Script.VerificationScript[8] ^= 0xff
			body := doRPCCall(fmt.Sprintf(rpc, encodeBinaryToString(t, s)), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrVerificationFailedCode)
		})

		t.Run("invalid height", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 2, 0, newTxWithParams(t, chain, opcode.PUSH1, 10, 0, 1, false))
			body := doRPCCall(fmt.Sprintf(rpc, encodeBinaryToString(t, b)), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrAlreadyExistsCode)
		})
		t.Run("invalid script", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 1, 0, newTxWithParams(t, chain, 0xDD, 10, 0, 1, false))
			body := doRPCCall(fmt.Sprintf(rpc, encodeBinaryToString(t, b)), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrInvalidScriptCode)
		})
		t.Run("invalid ValidUntilBlock", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 1, 0, newTxWithParams(t, chain, opcode.PUSH1, 0, 0, 1, false))
			body := doRPCCall(fmt.Sprintf(rpc, encodeBinaryToString(t, b)), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrExpiredTransactionCode)
		})
		t.Run("invalid SystemFee", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 1, 0, newTxWithParams(t, chain, opcode.PUSH1, 10, 999999999999, 1, false))
			body := doRPCCall(fmt.Sprintf(rpc, encodeBinaryToString(t, b)), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrPolicyFailedCode)
		})
		t.Run("invalid NetworkFee", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 1, 0, newTxWithParams(t, chain, opcode.PUSH1, 10, 0, 0, false))
			body := doRPCCall(fmt.Sprintf(rpc, encodeBinaryToString(t, b)), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrInsufficientNetworkFeeCode)
		})
		t.Run("invalid attribute", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 1, 0, newTxWithParams(t, chain, opcode.PUSH1, 10, 0, 2, true))
			body := doRPCCall(fmt.Sprintf(rpc, encodeBinaryToString(t, b)), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrInvalidAttributeCode)
		})
		t.Run("insufficient funds", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 1, 0, newTxWithParams(t, chain, opcode.PUSH1, 10, 899999999999, 1, false))
			body := doRPCCall(fmt.Sprintf(rpc, encodeBinaryToString(t, b)), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrInsufficientFundsCode)
		})
		t.Run("positive", func(t *testing.T) {
			b := testchain.NewBlock(t, chain, 1, 0, newTxWithParams(t, chain, opcode.PUSH1, 10, 0, 1, false))
			body := doRPCCall(fmt.Sprintf(rpc, encodeBinaryToString(t, b)), httpSrv.URL, t)
			data := checkErrGetResult(t, body, false, 0)
			var res = new(result.RelayResult)
			require.NoError(t, json.Unmarshal(data, res))
			require.Equal(t, b.Hash(), res.Hash)
		})
	})
	t.Run("getproof", func(t *testing.T) {
		r, err := chain.GetStateModule().GetStateRoot(3)
		require.NoError(t, err)

		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getproof", "params": ["%s", "%s", "%s"]}`,
			r.Root.StringLE(), testContractHash, base64.StdEncoding.EncodeToString([]byte("testkey")))
		body := doRPCCall(rpc, httpSrv.URL, t)
		rawRes := checkErrGetResult(t, body, false, 0)
		res := new(result.ProofWithKey)
		require.NoError(t, json.Unmarshal(rawRes, res))
		h, _ := util.Uint160DecodeStringLE(testContractHash)
		skey := makeStorageKey(chain.GetContractState(h).ID, []byte("testkey"))
		require.Equal(t, skey, res.Key)
		require.True(t, len(res.Proof) > 0)

		rpc = fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "verifyproof", "params": ["%s", "%s"]}`,
			r.Root.StringLE(), res.String())
		body = doRPCCall(rpc, httpSrv.URL, t)
		rawRes = checkErrGetResult(t, body, false, 0)
		vp := new(result.VerifyProof)
		require.NoError(t, json.Unmarshal(rawRes, vp))
		require.Equal(t, []byte("testvalue"), vp.Value)
	})
	t.Run("getstateroot", func(t *testing.T) {
		testRoot := func(t *testing.T, p string) {
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getstateroot", "params": [%s]}`, p)
			body := doRPCCall(rpc, httpSrv.URL, t)
			rawRes := checkErrGetResult(t, body, false, 0)

			res := &state.MPTRoot{}
			require.NoError(t, json.Unmarshal(rawRes, res))
			require.NotEqual(t, util.Uint256{}, res.Root) // be sure this test uses valid height

			expected, err := e.chain.GetStateModule().GetStateRoot(5)
			require.NoError(t, err)
			require.Equal(t, expected, res)
		}
		t.Run("ByHeight", func(t *testing.T) { testRoot(t, strconv.FormatInt(5, 10)) })
		t.Run("ByHash", func(t *testing.T) { testRoot(t, `"`+chain.GetHeaderHash(5).StringLE()+`"`) })
		t.Run("20", func(t *testing.T) {
			rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getstateroot", "params": [20]}`
			body := doRPCCall(rpc, httpSrv.URL, t)
			rawRes := checkErrGetResult(t, body, false, 0)

			res := &state.MPTRoot{}
			require.NoError(t, json.Unmarshal(rawRes, res))
			require.Equal(t, block20StateRootLE, res.Root.StringLE())
		})
	})
	t.Run("getstate", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getstate", "params": [%s]}`
		testGetState := func(t *testing.T, p string, expected string) {
			body := doRPCCall(fmt.Sprintf(rpc, p), httpSrv.URL, t)
			rawRes := checkErrGetResult(t, body, false, 0)

			var actual string
			require.NoError(t, json.Unmarshal(rawRes, &actual))
			require.Equal(t, expected, actual)
		}
		t.Run("good: historical state", func(t *testing.T) {
			root, err := e.chain.GetStateModule().GetStateRoot(4)
			require.NoError(t, err)
			// `testkey`-`testvalue` pair was put to the contract storage at block #3
			params := fmt.Sprintf(`"%s", "%s", "%s"`, root.Root.StringLE(), testContractHash, base64.StdEncoding.EncodeToString([]byte("testkey")))
			testGetState(t, params, base64.StdEncoding.EncodeToString([]byte("testvalue")))
		})
		t.Run("negative: invalid key", func(t *testing.T) {
			root, err := e.chain.GetStateModule().GetStateRoot(4)
			require.NoError(t, err)
			// `testkey`-`testvalue` pair was put to the contract storage at block #3
			params := fmt.Sprintf(`"%s", "%s", "%s"`, root.Root.StringLE(), testContractHash, base64.StdEncoding.EncodeToString([]byte("invalidkey")))
			body := doRPCCall(fmt.Sprintf(rpc, params), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.InvalidParamsCode)
		})
		t.Run("good: fresh state", func(t *testing.T) {
			root, err := e.chain.GetStateModule().GetStateRoot(16)
			require.NoError(t, err)
			// `testkey`-`newtestvalue` pair was put to the contract storage at block #16
			params := fmt.Sprintf(`"%s", "%s", "%s"`, root.Root.StringLE(), testContractHash, base64.StdEncoding.EncodeToString([]byte("testkey")))
			testGetState(t, params, base64.StdEncoding.EncodeToString([]byte("newtestvalue")))
		})
	})
	t.Run("findstates", func(t *testing.T) {
		testFindStates := func(t *testing.T, p string, root util.Uint256, expected result.FindStates) {
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "findstates", "params": [%s]}`, p)
			body := doRPCCall(rpc, httpSrv.URL, t)
			rawRes := checkErrGetResult(t, body, false, 0)

			var actual result.FindStates
			require.NoError(t, json.Unmarshal(rawRes, &actual))
			require.Equal(t, expected.Results, actual.Results)

			checkProof := func(t *testing.T, proof *result.ProofWithKey, value []byte) {
				rpc = fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "verifyproof", "params": ["%s", "%s"]}`,
					root.StringLE(), proof.String())
				body = doRPCCall(rpc, httpSrv.URL, t)
				rawRes = checkErrGetResult(t, body, false, 0)
				vp := new(result.VerifyProof)
				require.NoError(t, json.Unmarshal(rawRes, vp))
				require.Equal(t, value, vp.Value)
			}
			if len(actual.Results) > 0 {
				checkProof(t, actual.FirstProof, actual.Results[0].Value)
			}
			if len(actual.Results) > 1 {
				checkProof(t, actual.LastProof, actual.Results[len(actual.Results)-1].Value)
			}
			require.Equal(t, expected.Truncated, actual.Truncated)
		}
		t.Run("good: no prefix, no limit", func(t *testing.T) {
			// pairs for this test where put to the contract storage at block #16
			root, err := e.chain.GetStateModule().GetStateRoot(16)
			require.NoError(t, err)
			params := fmt.Sprintf(`"%s", "%s", "%s"`, root.Root.StringLE(), testContractHash, base64.StdEncoding.EncodeToString([]byte("aa")))
			testFindStates(t, params, root.Root, result.FindStates{
				Results: []result.KeyValue{
					{Key: []byte("aa10"), Value: []byte("v2")},
					{Key: []byte("aa50"), Value: []byte("v3")},
					{Key: []byte("aa"), Value: []byte("v1")},
				},
				Truncated: false,
			})
		})
		t.Run("good: empty prefix, no limit", func(t *testing.T) {
			// empty prefix should be considered as no prefix specified.
			root, err := e.chain.GetStateModule().GetStateRoot(16)
			require.NoError(t, err)
			params := fmt.Sprintf(`"%s", "%s", "%s", ""`, root.Root.StringLE(), testContractHash, base64.StdEncoding.EncodeToString([]byte("aa")))
			testFindStates(t, params, root.Root, result.FindStates{
				Results: []result.KeyValue{
					{Key: []byte("aa10"), Value: []byte("v2")},
					{Key: []byte("aa50"), Value: []byte("v3")},
					{Key: []byte("aa"), Value: []byte("v1")},
				},
				Truncated: false,
			})
		})
		t.Run("good: empty prefix, no limit, no data", func(t *testing.T) {
			// empty prefix should be considered as no prefix specified.
			root, err := e.chain.GetStateModule().GetStateRoot(20)
			require.NoError(t, err)
			stdHash, _ := e.chain.GetNativeContractScriptHash(nativenames.StdLib) // It has no data.
			params := fmt.Sprintf(`"%s", "%s", ""`, root.Root.StringLE(), stdHash.StringLE())
			testFindStates(t, params, root.Root, result.FindStates{
				Results:   []result.KeyValue{},
				Truncated: false,
			})
		})
		t.Run("good: with prefix, no limit", func(t *testing.T) {
			// pairs for this test where put to the contract storage at block #16
			root, err := e.chain.GetStateModule().GetStateRoot(16)
			require.NoError(t, err)
			params := fmt.Sprintf(`"%s", "%s", "%s", "%s"`, root.Root.StringLE(), testContractHash, base64.StdEncoding.EncodeToString([]byte("aa")), base64.StdEncoding.EncodeToString([]byte("aa10")))
			testFindStates(t, params, root.Root, result.FindStates{
				Results: []result.KeyValue{
					{Key: []byte("aa50"), Value: []byte("v3")},
				},
				Truncated: false,
			})
		})
		t.Run("good: empty prefix, with limit", func(t *testing.T) {
			for limit := 2; limit < 5; limit++ {
				// pairs for this test where put to the contract storage at block #16
				root, err := e.chain.GetStateModule().GetStateRoot(16)
				require.NoError(t, err)
				params := fmt.Sprintf(`"%s", "%s", "%s", "", %d`, root.Root.StringLE(), testContractHash, base64.StdEncoding.EncodeToString([]byte("aa")), limit)
				expected := result.FindStates{
					Results: []result.KeyValue{
						{Key: []byte("aa10"), Value: []byte("v2")},
						{Key: []byte("aa50"), Value: []byte("v3")},
					},
					Truncated: limit == 2,
				}
				if limit != 2 {
					expected.Results = append(expected.Results, result.KeyValue{Key: []byte("aa"), Value: []byte("v1")})
				}
				testFindStates(t, params, root.Root, expected)
			}
		})
		t.Run("good: with prefix, with limit", func(t *testing.T) {
			// pairs for this test where put to the contract storage at block #16
			root, err := e.chain.GetStateModule().GetStateRoot(16)
			require.NoError(t, err)
			params := fmt.Sprintf(`"%s", "%s", "%s", "%s", %d`, root.Root.StringLE(), testContractHash, base64.StdEncoding.EncodeToString([]byte("aa")), base64.StdEncoding.EncodeToString([]byte("aa00")), 1)
			testFindStates(t, params, root.Root, result.FindStates{
				Results: []result.KeyValue{
					{Key: []byte("aa10"), Value: []byte("v2")},
				},
				Truncated: true,
			})
		})
	})

	t.Run("getrawtransaction", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(1))
		tx := block.Transactions[0]
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s"]}"`, tx.Hash().StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		result := checkErrGetResult(t, body, false, 0)
		var res string
		err := json.Unmarshal(result, &res)
		require.NoErrorf(t, err, "could not parse response: %s", result)
		txBin, err := testserdes.EncodeBinary(tx)
		require.NoError(t, err)
		expected := base64.StdEncoding.EncodeToString(txBin)
		assert.Equal(t, expected, res)
	})

	t.Run("getrawtransaction 2 arguments", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(1))
		tx := block.Transactions[0]
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 0]}"`, tx.Hash().StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		result := checkErrGetResult(t, body, false, 0)
		var res string
		err := json.Unmarshal(result, &res)
		require.NoErrorf(t, err, "could not parse response: %s", result)
		txBin, err := testserdes.EncodeBinary(tx)
		require.NoError(t, err)
		expected := base64.StdEncoding.EncodeToString(txBin)
		assert.Equal(t, expected, res)
	})

	t.Run("getrawtransaction 2 arguments, verbose", func(t *testing.T) {
		block, _ := chain.GetBlock(chain.GetHeaderHash(1))
		TXHash := block.Transactions[0].Hash()
		_ = block.Transactions[0].Size()
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["%s", 1]}"`, TXHash.StringLE())
		body := doRPCCall(rpc, httpSrv.URL, t)
		txOut := checkErrGetResult(t, body, false, 0)
		actual := result.TransactionOutputRaw{Transaction: transaction.Transaction{}}
		err := json.Unmarshal(txOut, &actual)
		require.NoErrorf(t, err, "could not parse response: %s", txOut)

		assert.Equal(t, *block.Transactions[0], actual.Transaction)
		assert.Equal(t, 24, actual.Confirmations)
		assert.Equal(t, TXHash, actual.Transaction.Hash())
	})

	t.Run("getblockheader_positive", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getblockheader", "params": %s}`
		testHeaderHash := chain.GetHeaderHash(1).StringLE()
		hdr := e.getHeader(testHeaderHash)

		runCase := func(t *testing.T, rpc string, expected, actual any) {
			body := doRPCCall(rpc, httpSrv.URL, t)
			data := checkErrGetResult(t, body, false, 0)
			require.NoError(t, json.Unmarshal(data, actual))
			require.Equal(t, expected, actual)
		}

		t.Run("no verbose", func(t *testing.T) {
			w := io.NewBufBinWriter()
			hdr.EncodeBinary(w.BinWriter)
			require.NoError(t, w.Err)
			encoded := base64.StdEncoding.EncodeToString(w.Bytes())

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
			nextHash := chain.GetHeaderHash(hdr.Index + 1)
			expected := &result.Header{
				Header: *hdr,
				BlockMetadata: result.BlockMetadata{
					Size:          io.GetVarSize(hdr),
					NextBlockHash: &nextHash,
					Confirmations: e.chain.BlockHeight() - hdr.Index + 1,
				},
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
			tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
			tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
			assert.NoError(t, mp.Add(tx, &FeerStub{}))
			expected = append(expected, tx.Hash())
		}

		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "getrawmempool", "params": []}`
		body := doRPCCall(rpc, httpSrv.URL, t)
		res := checkErrGetResult(t, body, false, 0)

		var actual []util.Uint256
		err := json.Unmarshal(res, &actual)
		require.NoErrorf(t, err, "could not parse response: %s", res)

		assert.ElementsMatch(t, expected, actual)
	})

	t.Run("getnep17transfers", func(t *testing.T) {
		testNEP17T := func(t *testing.T, start, stop, limit, page int, sent, rcvd []int) {
			ps := []string{`"` + testchain.PrivateKeyByID(0).Address() + `"`}
			if start != 0 {
				h, err := e.chain.GetHeader(e.chain.GetHeaderHash(uint32(start)))
				var ts uint64
				if err == nil {
					ts = h.Timestamp
				} else {
					ts = uint64(time.Now().UnixNano() / 1_000_000)
				}
				ps = append(ps, strconv.FormatUint(ts, 10))
			}
			if stop != 0 {
				h, err := e.chain.GetHeader(e.chain.GetHeaderHash(uint32(stop)))
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
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "getnep17transfers", "params": [%s]}`, p)
			body := doRPCCall(rpc, httpSrv.URL, t)
			res := checkErrGetResult(t, body, false, 0)
			actual := new(result.NEP17Transfers)
			require.NoError(t, json.Unmarshal(res, actual))
			checkNep17TransfersAux(t, e, actual, sent, rcvd)
		}
		t.Run("time frame only", func(t *testing.T) { testNEP17T(t, 4, 5, 0, 0, []int{19, 20, 21, 22}, []int{3, 4}) })
		t.Run("no res", func(t *testing.T) { testNEP17T(t, 100, 100, 0, 0, []int{}, []int{}) })
		t.Run("limit", func(t *testing.T) { testNEP17T(t, 1, 7, 3, 0, []int{16, 17}, []int{2}) })
		t.Run("limit 2", func(t *testing.T) { testNEP17T(t, 4, 5, 2, 0, []int{19}, []int{3}) })
		t.Run("limit with page", func(t *testing.T) { testNEP17T(t, 1, 7, 3, 1, []int{18, 19}, []int{3}) })
		t.Run("limit with page 2", func(t *testing.T) { testNEP17T(t, 1, 7, 3, 2, []int{20, 21}, []int{4}) })
	})

	prepareIteratorSession := func(t *testing.T) (uuid.UUID, uuid.UUID) {
		rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "invokefunction", "params": ["%s", "iterateOverValues"]}"`, storageContractHash)
		body := doRPCCall(rpc, httpSrv.URL, t)
		resp := checkErrGetResult(t, body, false, 0)
		res := new(result.Invoke)
		err := json.Unmarshal(resp, &res)
		require.NoErrorf(t, err, "could not parse response: %s", resp)
		require.NotEmpty(t, res.Session)
		require.Equal(t, 1, len(res.Stack))
		require.Equal(t, stackitem.InteropT, res.Stack[0].Type())
		iterator, ok := res.Stack[0].Value().(result.Iterator)
		require.True(t, ok)
		require.NotEmpty(t, iterator.ID)
		return res.Session, *iterator.ID
	}
	t.Run("traverseiterator", func(t *testing.T) {
		t.Run("good", func(t *testing.T) {
			sID, iID := prepareIteratorSession(t)
			expectedCount := 99
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "traverseiterator", "params": ["%s", "%s", %d]}"`, sID.String(), iID.String(), expectedCount)
			body := doRPCCall(rpc, httpSrv.URL, t)
			resp := checkErrGetResult(t, body, false, 0)
			res := new([]json.RawMessage)
			require.NoError(t, json.Unmarshal(resp, res))
			require.Equal(t, expectedCount, len(*res))
		})
		t.Run("invalid session id", func(t *testing.T) {
			_, iID := prepareIteratorSession(t)
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "traverseiterator", "params": ["not-a-uuid", "%s", %d]}"`, iID.String(), 1)
			body := doRPCCall(rpc, httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.InvalidParamsCode, "invalid session ID: not a valid UUID")
		})
		t.Run("invalid iterator id", func(t *testing.T) {
			sID, _ := prepareIteratorSession(t)
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "traverseiterator", "params": ["%s", "not-a-uuid", %d]}"`, sID.String(), 1)
			body := doRPCCall(rpc, httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.InvalidParamsCode, "invalid iterator ID: not a valid UUID")
		})
		t.Run("invalid items count", func(t *testing.T) {
			sID, iID := prepareIteratorSession(t)
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "traverseiterator", "params": ["%s", "%s"]}"`, sID.String(), iID.String())
			body := doRPCCall(rpc, httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.InvalidParamsCode, "invalid iterator items count")
		})
		t.Run("items count is not an int32", func(t *testing.T) {
			sID, iID := prepareIteratorSession(t)
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "traverseiterator", "params": ["%s", "%s", %d]}"`, sID.String(), iID.String(), math.MaxInt32+1)
			body := doRPCCall(rpc, httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.InvalidParamsCode, "invalid iterator items count: not an int32")
		})
		t.Run("count is out of range", func(t *testing.T) {
			sID, iID := prepareIteratorSession(t)
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "traverseiterator", "params": ["%s", "%s", %d]}"`, sID.String(), iID.String(), rpcSrv.config.MaxIteratorResultItems+1)
			body := doRPCCall(rpc, httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.InvalidParamsCode, fmt.Sprintf("iterator items count is out of range (%d at max)", rpcSrv.config.MaxIteratorResultItems))
		})
		t.Run("unknown session", func(t *testing.T) {
			_, iID := prepareIteratorSession(t)
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "traverseiterator", "params": ["%s", "%s", %d]}"`, uuid.NewString(), iID.String(), 1)
			body := doRPCCall(rpc, httpSrv.URL, t)
			resp := checkErrGetResult(t, body, false, 0)
			res := new([]json.RawMessage)
			require.NoError(t, json.Unmarshal(resp, res))
			require.Equal(t, 0, len(*res)) // No errors expected, no elements should be returned.
		})
		t.Run("unknown iterator", func(t *testing.T) {
			sID, _ := prepareIteratorSession(t)
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "traverseiterator", "params": ["%s", "%s", %d]}"`, sID.String(), uuid.NewString(), 1)
			body := doRPCCall(rpc, httpSrv.URL, t)
			resp := checkErrGetResult(t, body, false, 0)
			res := new([]json.RawMessage)
			require.NoError(t, json.Unmarshal(resp, res))
			require.Equal(t, 0, len(*res)) // No errors expected, no elements should be returned.
		})
	})
	t.Run("terminatesession", func(t *testing.T) {
		check := func(t *testing.T, id string, expected bool) {
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "terminatesession", "params": ["%s"]}"`, id)
			body := doRPCCall(rpc, httpSrv.URL, t)
			resp := checkErrGetResult(t, body, false, 0)
			res := new(bool)
			require.NoError(t, json.Unmarshal(resp, res))
			require.Equal(t, expected, *res)
		}
		t.Run("true", func(t *testing.T) {
			sID, _ := prepareIteratorSession(t)
			check(t, sID.String(), true)
		})
		t.Run("false", func(t *testing.T) {
			check(t, uuid.NewString(), false)
		})
		t.Run("expired", func(t *testing.T) {
			_, _ = prepareIteratorSession(t)
			// Wait until session is terminated by timer.
			require.Eventually(t, func() bool {
				rpcSrv.sessionsLock.Lock()
				defer rpcSrv.sessionsLock.Unlock()
				return len(rpcSrv.sessions) == 0
			}, 2*time.Duration(rpcSrv.config.SessionExpirationTime)*time.Second, 10*time.Millisecond)
		})
	})
	t.Run("calculatenetworkfee", func(t *testing.T) {
		t.Run("no parameters", func(t *testing.T) {
			body := doRPCCall(`{"jsonrpc": "2.0", "id": 1, "method": "calculatenetworkfee", "params": []}"`, httpSrv.URL, t)
			_ = checkErrGetResult(t, body, true, neorpc.InvalidParamsCode, "Invalid Params")
		})
		t.Run("non-base64 parameter", func(t *testing.T) {
			body := doRPCCall(`{"jsonrpc": "2.0", "id": 1, "method": "calculatenetworkfee", "params": ["noatbase64"]}"`, httpSrv.URL, t)
			_ = checkErrGetResult(t, body, true, neorpc.InvalidParamsCode, "Invalid Params")
		})
		t.Run("non-transaction parameter", func(t *testing.T) {
			body := doRPCCall(`{"jsonrpc": "2.0", "id": 1, "method": "calculatenetworkfee", "params": ["bm90IGEgdHJhbnNhY3Rpb24K"]}"`, httpSrv.URL, t)
			_ = checkErrGetResult(t, body, true, neorpc.InvalidParamsCode, "Invalid Params")
		})
		calcReq := func(t *testing.T, tx *transaction.Transaction) []byte {
			rpc := fmt.Sprintf(`{"jsonrpc": "2.0", "id": 1, "method": "calculatenetworkfee", "params": ["%s"]}"`, base64.StdEncoding.EncodeToString(tx.Bytes()))
			return doRPCCall(rpc, httpSrv.URL, t)
		}
		t.Run("non-contract with zero verification", func(t *testing.T) {
			tx := &transaction.Transaction{
				Script:  []byte{byte(opcode.RET)},
				Signers: []transaction.Signer{{Account: util.Uint160{1, 2, 3}, Scopes: transaction.CalledByEntry}},
				Scripts: []transaction.Witness{{
					InvocationScript:   []byte{},
					VerificationScript: []byte{},
				}},
			}
			body := calcReq(t, tx)
			_ = checkErrGetResult(t, body, true, neorpc.ErrInvalidVerificationFunctionCode, "signer 0 has no verification script and no deployed contract")
		})
		t.Run("contract with no verify", func(t *testing.T) {
			tx := &transaction.Transaction{
				Script:  []byte{byte(opcode.RET)},
				Signers: []transaction.Signer{{Account: nnsHash, Scopes: transaction.CalledByEntry}},
				Scripts: []transaction.Witness{{
					InvocationScript:   []byte{},
					VerificationScript: []byte{},
				}},
			}
			body := calcReq(t, tx)
			_ = checkErrGetResult(t, body, true, neorpc.ErrInvalidVerificationFunctionCode, "signer 0 has no verify method in deployed contract")
		})
		checkCalc := func(t *testing.T, tx *transaction.Transaction, fee int64) {
			resp := checkErrGetResult(t, calcReq(t, tx), false, 0)
			res := new(result.NetworkFee)
			require.NoError(t, json.Unmarshal(resp, res))
			require.Equal(t, fee, res.Value)
		}
		t.Run("simple GAS transfer", func(t *testing.T) {
			priv0 := testchain.PrivateKeyByID(0)
			script, err := smartcontract.CreateCallWithAssertScript(chain.UtilityTokenHash(), "transfer",
				priv0.GetScriptHash(), priv0.GetScriptHash(), 1, nil)
			require.NoError(t, err)
			tx := &transaction.Transaction{
				Script:  script,
				Signers: []transaction.Signer{{Account: priv0.GetScriptHash(), Scopes: transaction.CalledByEntry}},
				Scripts: []transaction.Witness{{
					InvocationScript:   []byte{},
					VerificationScript: priv0.PublicKey().GetVerificationScript(),
				}},
			}
			checkCalc(t, tx, 1228520) // Perfectly matches FeeIsSignatureContractDetailed() C# test.
		})
		t.Run("multisignature tx", func(t *testing.T) {
			priv0 := testchain.PrivateKeyByID(0)
			priv1 := testchain.PrivateKeyByID(1)
			accScript, err := smartcontract.CreateDefaultMultiSigRedeemScript(keys.PublicKeys{priv0.PublicKey(), priv1.PublicKey()})
			require.NoError(t, err)
			multiAcc := hash.Hash160(accScript)
			txScript, err := smartcontract.CreateCallWithAssertScript(chain.UtilityTokenHash(), "transfer",
				multiAcc, priv0.GetScriptHash(), 1, nil)
			require.NoError(t, err)
			tx := &transaction.Transaction{
				Script:  txScript,
				Signers: []transaction.Signer{{Account: multiAcc, Scopes: transaction.CalledByEntry}},
				Scripts: []transaction.Witness{{
					InvocationScript:   []byte{},
					VerificationScript: accScript,
				}},
			}
			checkCalc(t, tx, 2315100) // Perfectly matches FeeIsMultiSigContract() C# test.
		})
		checkContract := func(t *testing.T, verAcc util.Uint160, invoc []byte, fee int64) {
			txScript, err := smartcontract.CreateCallWithAssertScript(chain.UtilityTokenHash(), "transfer",
				verAcc, verAcc, 1, nil)
			require.NoError(t, err)
			tx := &transaction.Transaction{
				Script:  txScript,
				Signers: []transaction.Signer{{Account: verAcc, Scopes: transaction.CalledByEntry}},
				Scripts: []transaction.Witness{{
					InvocationScript:   invoc,
					VerificationScript: []byte{},
				}},
			}
			checkCalc(t, tx, fee)
		}
		t.Run("contract-based verification", func(t *testing.T) {
			verAcc, err := util.Uint160DecodeStringLE(verifyContractHash)
			require.NoError(t, err)
			checkContract(t, verAcc, []byte{}, 636610) // No C# match, but we believe it's OK.
		})
		t.Run("contract-based verification with parameters", func(t *testing.T) {
			verAcc, err := util.Uint160DecodeStringLE(verifyWithArgsContractHash)
			require.NoError(t, err)
			checkContract(t, verAcc, []byte{}, 244130) // No C# match, but we believe it's OK and it differs from the one above.
		})
		t.Run("contract-based verification with invocation script", func(t *testing.T) {
			verAcc, err := util.Uint160DecodeStringLE(verifyWithArgsContractHash)
			require.NoError(t, err)
			invocWriter := io.NewBufBinWriter()
			emit.Bool(invocWriter.BinWriter, false)
			emit.Int(invocWriter.BinWriter, 5)
			emit.String(invocWriter.BinWriter, "")
			invocScript := invocWriter.Bytes()
			checkContract(t, verAcc, invocScript, 146960) // No C# match, but we believe it's OK and it has a specific invocation script overriding anything server-side.
		})
	})
	t.Run("sendrawtransaction", func(t *testing.T) {
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "sendrawtransaction", "params": ["%s"]}`
		t.Run("invalid signature", func(t *testing.T) {
			tx := newTxWithParams(t, chain, opcode.PUSH1, 10, 1, 1, false)
			tx.Scripts[0].InvocationScript[10] = ^tx.Scripts[0].InvocationScript[10]
			rawTx := encodeBinaryToString(t, tx)
			body := doRPCCall(fmt.Sprintf(rpc, rawTx), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrInvalidSignatureCode)
		})
		t.Run("too big tx", func(t *testing.T) {
			script := make([]byte, transaction.MaxScriptLength)
			for i := range script {
				script[i] = byte(opcode.PUSH0)
			}
			groups := make([]*keys.PublicKey, 16)
			for i := range groups {
				pk, _ := keys.NewPrivateKey()
				groups[i] = pk.PublicKey()
			}
			signers := make([]transaction.Signer, transaction.MaxAttributes)
			for i := range signers {
				signers[i] = transaction.Signer{
					Account:          random.Uint160(),
					Scopes:           transaction.CustomContracts | transaction.CustomGroups,
					AllowedContracts: make([]util.Uint160, 16),
					AllowedGroups:    groups,
				}
			}
			scripts := make([]transaction.Witness, len(signers))
			for i := range scripts {
				scripts[i] = transaction.Witness{
					InvocationScript:   random.Bytes(transaction.MaxInvocationScript),
					VerificationScript: random.Bytes(transaction.MaxVerificationScript),
				}
			}
			tx := &transaction.Transaction{
				ValidUntilBlock: chain.BlockHeight() + 1,
				Script:          script,
				Attributes:      []transaction.Attribute{},
				Signers:         signers,
				Scripts:         scripts,
			}
			rawTx := encodeBinaryToString(t, tx)
			body := doRPCCall(fmt.Sprintf(rpc, rawTx), httpSrv.URL, t)
			checkErrGetResult(t, body, true, neorpc.ErrInvalidSizeCode)
		})
		t.Run("mempool OOM", func(t *testing.T) {
			chain, rpcSrv, httpSrv := initClearServerWithCustomConfig(t, func(c *config.Config) {
				c.ProtocolConfiguration.MemPoolSize = 1
			})

			defer chain.Close()
			defer rpcSrv.Shutdown()

			// create and push the first (prioritized) transaction with increased networkFee
			tx := newTxWithParams(t, chain, opcode.PUSH1, 10, 1, 2, false)
			rawTx := encodeBinaryToString(t, tx)
			body := doRPCCall(fmt.Sprintf(rpc, rawTx), httpSrv.URL, t)
			checkErrGetResult(t, body, false, 0)

			// create and push the second transaction with standard networkFee
			tx2 := newTxWithParams(t, chain, opcode.PUSH1, 10, 1, 1, false)
			rawTx2 := encodeBinaryToString(t, tx2)
			body2 := doRPCCall(fmt.Sprintf(rpc, rawTx2), httpSrv.URL, t)
			checkErrGetResult(t, body2, true, neorpc.ErrMempoolCapReachedCode)
		})
	})
	t.Run("test functions with unsupported states", func(t *testing.T) {
		chain, rpcSrv, httpSrv := initClearServerWithCustomConfig(t, func(c *config.Config) {
			c.ApplicationConfiguration.Ledger.KeepOnlyLatestState = true
		})

		defer chain.Close()
		defer rpcSrv.Shutdown()

		e := &executor{chain: chain, httpSrv: httpSrv}
		rpc := `{"jsonrpc": "2.0", "id": 1, "method": "%s", "params": %s}`
		for method, cases := range rpcFunctionsWithUnsupportedStatesTestCases {
			runTestCasesWithExecutor(t, e, rpc, method, cases, doRPCCall, checkErrGetResult)
		}
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
	return &block.Header
}

func encodeBinaryToString(t *testing.T, a io.Serializable) string {
	bytes, err := testserdes.EncodeBinary(a)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(bytes)
}

func newTxWithParams(t *testing.T, chain *core.Blockchain, code opcode.Opcode, validUntilIncr uint32, systemFee int64,
	networkFeeMultiplier int64, addAttrNotValidBeforeT bool) *transaction.Transaction {
	priv0 := testchain.PrivateKeyByID(0)
	acc0 := wallet.NewAccountFromPrivateKey(priv0)

	height := chain.BlockHeight()
	tx := transaction.New([]byte{byte(code)}, 0)
	tx.Nonce = height + 1
	tx.ValidUntilBlock = height + validUntilIncr
	tx.Signers = []transaction.Signer{{Account: acc0.PrivateKey().GetScriptHash()}}
	tx.SystemFee = systemFee
	// add network fee
	size := io.GetVarSize(tx)
	netFee, sizeDelta := fee.Calculate(chain.GetBaseExecFee(), acc0.Contract.Script)
	tx.NetworkFee += netFee
	size += sizeDelta
	tx.NetworkFee += int64(size) * chain.FeePerByte()
	tx.NetworkFee = tx.NetworkFee * networkFeeMultiplier
	if addAttrNotValidBeforeT {
		tx.Attributes = []transaction.Attribute{
			{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: height + 1}},
		}
	}
	require.NoError(t, acc0.SignTx(testchain.Network(), tx))
	return tx
}

func (tc rpcTestCase) getResultPair(e *executor) (expected any, res any) {
	expected = tc.result(e)
	resVal := reflect.New(reflect.TypeOf(expected).Elem())
	res = resVal.Interface()
	return expected, res
}

func checkErrGetResult(t *testing.T, body []byte, expectingFail bool, expectedErrCode int64, expectedErr ...string) json.RawMessage {
	var resp neorpc.Response
	err := json.Unmarshal(body, &resp)
	require.Nil(t, err)
	if expectingFail {
		require.NotNil(t, resp.Error)
		assert.NotEqual(t, 0, resp.Error.Code)
		assert.Equal(t, expectedErrCode, resp.Error.Code)
		assert.NotEqual(t, "", resp.Error.Message)
		if len(expectedErr) != 0 {
			assert.True(t, strings.Contains(resp.Error.Error(), expectedErr[0]), fmt.Sprintf("expected: %s, got: %s", expectedErr[0], resp.Error.Error()))
		}
	} else {
		assert.Nil(t, resp.Error)
	}
	return resp.Result
}

func checkErrGetBatchResult(t *testing.T, body []byte, expectingFail bool, expectedErrCode int64, expectedErr ...string) json.RawMessage {
	var resp []neorpc.Response
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
	c, r, err := dialer.Dial(url+"/ws", nil)
	require.NoError(t, err)
	defer r.Body.Close()
	err = c.SetWriteDeadline(time.Now().Add(time.Second))
	require.NoError(t, err)
	require.NoError(t, c.WriteMessage(1, []byte(rpcCall)))
	err = c.SetReadDeadline(time.Now().Add(time.Second))
	require.NoError(t, err)
	_, body, err := c.ReadMessage()
	require.NoError(t, err)
	require.NoError(t, c.Close())
	return bytes.TrimSpace(body)
}

func doRPCCallOverHTTP(rpcCall string, url string, t *testing.T) []byte {
	cl := http.Client{Timeout: time.Second}
	resp, err := cl.Post(url, "application/json", strings.NewReader(rpcCall))
	require.NoErrorf(t, err, "could not make a POST request")
	body, err := gio.ReadAll(resp.Body)
	resp.Body.Close()
	assert.NoErrorf(t, err, "could not read response from the request: %s", rpcCall)
	return bytes.TrimSpace(body)
}

func checkNep11Balances(t *testing.T, e *executor, acc any) {
	res, ok := acc.(*result.NEP11Balances)
	require.True(t, ok)

	expected := result.NEP11Balances{
		Balances: []result.NEP11AssetBalance{
			{
				Asset:  nnsHash,
				Name:   "NameService",
				Symbol: "NNS",
				Tokens: []result.NEP11TokenBalance{
					{
						ID:          nnsToken1ID,
						Amount:      "1",
						LastUpdated: 14,
					},
				},
			},
			{
				Asset:    nfsoHash,
				Decimals: 2,
				Name:     "NeoFS Object NFT",
				Symbol:   "NFSO",
				Tokens: []result.NEP11TokenBalance{
					{
						ID:          nfsoToken1ID,
						Amount:      "80",
						LastUpdated: 21,
					},
				},
			},
		},
		Address: testchain.PrivateKeyByID(0).GetScriptHash().StringLE(),
	}
	require.Equal(t, testchain.PrivateKeyByID(0).Address(), res.Address)
	require.ElementsMatch(t, expected.Balances, res.Balances)
}

func checkNep17Balances(t *testing.T, e *executor, acc any) {
	res, ok := acc.(*result.NEP17Balances)
	require.True(t, ok)
	rubles, err := util.Uint160DecodeStringLE(testContractHash)
	require.NoError(t, err)
	expected := result.NEP17Balances{
		Balances: []result.NEP17Balance{
			{
				Asset:       rubles,
				Amount:      "877",
				Decimals:    2,
				LastUpdated: 6,
				Name:        "Rubl",
				Symbol:      "RUB",
			},
			{
				Asset:       e.chain.GoverningTokenHash(),
				Amount:      "99998000",
				LastUpdated: 4,
				Name:        "NeoToken",
				Symbol:      "NEO",
			},
			{
				Asset:       e.chain.UtilityTokenHash(),
				Amount:      "37106285100",
				LastUpdated: 23,
				Decimals:    8,
				Name:        "GasToken",
				Symbol:      "GAS",
			}},
		Address: testchain.PrivateKeyByID(0).GetScriptHash().StringLE(),
	}
	require.Equal(t, testchain.PrivateKeyByID(0).Address(), res.Address)
	require.ElementsMatch(t, expected.Balances, res.Balances)
}

func checkNep11Transfers(t *testing.T, e *executor, acc any) {
	checkNep11TransfersAux(t, e, acc, []int{0}, []int{0, 1, 2})
}

func checkNep11TransfersAux(t *testing.T, e *executor, acc any, sent, rcvd []int) {
	res, ok := acc.(*result.NEP11Transfers)
	require.True(t, ok)

	blockReceiveNFSO, err := e.chain.GetBlock(e.chain.GetHeaderHash(21)) // transfer 0.05 NFSO from priv1 back to priv0.
	require.NoError(t, err)
	require.Equal(t, 1, len(blockReceiveNFSO.Transactions))
	txReceiveNFSO := blockReceiveNFSO.Transactions[0]

	blockSendNFSO, err := e.chain.GetBlock(e.chain.GetHeaderHash(19)) // transfer 0.25 NFSO from priv0 to priv1.
	require.NoError(t, err)
	require.Equal(t, 1, len(blockSendNFSO.Transactions))
	txSendNFSO := blockSendNFSO.Transactions[0]

	blockMintNFSO, err := e.chain.GetBlock(e.chain.GetHeaderHash(18)) // mint 1.00 NFSO token by transferring 10 GAS to NFSO contract.
	require.NoError(t, err)
	require.Equal(t, 1, len(blockMintNFSO.Transactions))
	txMintNFSO := blockMintNFSO.Transactions[0]

	blockRegisterNSRecordA, err := e.chain.GetBlock(e.chain.GetHeaderHash(14)) // register `neo.com` with A record type and priv0 owner via NS
	require.NoError(t, err)
	require.Equal(t, 1, len(blockRegisterNSRecordA.Transactions))
	txRegisterNSRecordA := blockRegisterNSRecordA.Transactions[0]

	// These are laid out here explicitly for 2 purposes:
	//  * to be able to reference any particular event for paging
	//  * to check chain events consistency
	// Technically these could be retrieved from application log, but that would almost
	// duplicate the Server method.
	expected := result.NEP11Transfers{
		Sent: []result.NEP11Transfer{
			{
				Timestamp: blockSendNFSO.Timestamp,
				Asset:     nfsoHash,
				Address:   testchain.PrivateKeyByID(1).Address(), // to priv1
				ID:        nfsoToken1ID,                          // NFSO ID
				Amount:    big.NewInt(25).String(),
				Index:     19,
				TxHash:    txSendNFSO.Hash(),
			},
		},
		Received: []result.NEP11Transfer{
			{
				Timestamp: blockReceiveNFSO.Timestamp,
				Asset:     nfsoHash,
				ID:        nfsoToken1ID,
				Address:   testchain.PrivateKeyByID(1).Address(), // from priv1
				Amount:    "5",
				Index:     21,
				TxHash:    txReceiveNFSO.Hash(),
			},
			{
				Timestamp: blockMintNFSO.Timestamp,
				Asset:     nfsoHash,
				ID:        nfsoToken1ID,
				Address:   "", // minting
				Amount:    "100",
				Index:     18,
				TxHash:    txMintNFSO.Hash(),
			},
			{
				Timestamp: blockRegisterNSRecordA.Timestamp,
				Asset:     nnsHash,
				ID:        nnsToken1ID,
				Address:   "", // minting
				Amount:    "1",
				Index:     14,
				TxHash:    txRegisterNSRecordA.Hash(),
			},
		},
		Address: testchain.PrivateKeyByID(0).Address(),
	}

	require.Equal(t, expected.Address, res.Address)

	arr := make([]result.NEP11Transfer, 0, len(expected.Sent))
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

func checkNep17Transfers(t *testing.T, e *executor, acc any) {
	checkNep17TransfersAux(t, e, acc, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8})
}

func checkNep17TransfersAux(t *testing.T, e *executor, acc any, sent, rcvd []int) {
	res, ok := acc.(*result.NEP17Transfers)
	require.True(t, ok)
	rublesHash, err := util.Uint160DecodeStringLE(testContractHash)
	require.NoError(t, err)

	blockWithFAULTedTx, err := e.chain.GetBlock(e.chain.GetHeaderHash(faultedTxBlock)) // Transaction with ABORT inside.
	require.NoError(t, err)
	require.Equal(t, 1, len(blockWithFAULTedTx.Transactions))
	txFAULTed := blockWithFAULTedTx.Transactions[0]

	blockDeploy6, err := e.chain.GetBlock(e.chain.GetHeaderHash(22)) // deploy Storage contract (storage_contract.go)
	require.NoError(t, err)
	require.Equal(t, 1, len(blockDeploy6.Transactions))
	txDeploy6 := blockDeploy6.Transactions[0]

	blockTransferNFSO, err := e.chain.GetBlock(e.chain.GetHeaderHash(19)) // transfer 0.25 NFSO from priv0 to priv1.
	require.NoError(t, err)
	require.Equal(t, 1, len(blockTransferNFSO.Transactions))
	txTransferNFSO := blockTransferNFSO.Transactions[0]

	blockMintNFSO, err := e.chain.GetBlock(e.chain.GetHeaderHash(18)) // mint 1.00 NFSO token for priv0 by transferring 10 GAS to NFSO contract.
	require.NoError(t, err)
	require.Equal(t, 1, len(blockMintNFSO.Transactions))
	txMintNFSO := blockMintNFSO.Transactions[0]

	blockDeploy5, err := e.chain.GetBlock(e.chain.GetHeaderHash(17)) // deploy NeoFS Object contract (NEP11-Divisible)
	require.NoError(t, err)
	require.Equal(t, 1, len(blockDeploy5.Transactions))
	txDeploy5 := blockDeploy5.Transactions[0]

	blockPutNewTestValue, err := e.chain.GetBlock(e.chain.GetHeaderHash(16)) // invoke `put` method of `test_contract.go` with `testkey`, `newtestvalue` args
	require.NoError(t, err)
	require.Equal(t, 4, len(blockPutNewTestValue.Transactions))
	txPutNewTestValue := blockPutNewTestValue.Transactions[0]
	txPutValue1 := blockPutNewTestValue.Transactions[1] // invoke `put` method of `test_contract.go` with `aa`, `v1` args
	txPutValue2 := blockPutNewTestValue.Transactions[2] // invoke `put` method of `test_contract.go` with `aa10`, `v2` args
	txPutValue3 := blockPutNewTestValue.Transactions[3] // invoke `put` method of `test_contract.go` with `aa50`, `v3` args

	blockSetRecord, err := e.chain.GetBlock(e.chain.GetHeaderHash(15)) // add type A record to `neo.com` domain via NNS
	require.NoError(t, err)
	require.Equal(t, 1, len(blockSetRecord.Transactions))
	txSetRecord := blockSetRecord.Transactions[0]

	blockRegisterDomain, err := e.chain.GetBlock(e.chain.GetHeaderHash(14)) // register `neo.com` domain via NNS
	require.NoError(t, err)
	require.Equal(t, 1, len(blockRegisterDomain.Transactions))
	txRegisterDomain := blockRegisterDomain.Transactions[0]

	blockGASBounty2, err := e.chain.GetBlock(e.chain.GetHeaderHash(12)) // size of committee = 6
	require.NoError(t, err)

	blockDeploy4, err := e.chain.GetBlock(e.chain.GetHeaderHash(11)) // deploy ns.go (non-native neo name service contract)
	require.NoError(t, err)
	require.Equal(t, 1, len(blockDeploy4.Transactions))
	txDeploy4 := blockDeploy4.Transactions[0]

	blockDeploy3, err := e.chain.GetBlock(e.chain.GetHeaderHash(10)) // deploy verification_with_args_contract.go
	require.NoError(t, err)
	require.Equal(t, 1, len(blockDeploy3.Transactions))
	txDeploy3 := blockDeploy3.Transactions[0]

	blockDepositGAS, err := e.chain.GetBlock(e.chain.GetHeaderHash(8))
	require.NoError(t, err)
	require.Equal(t, 1, len(blockDepositGAS.Transactions))
	txDepositGAS := blockDepositGAS.Transactions[0]

	blockDeploy2, err := e.chain.GetBlock(e.chain.GetHeaderHash(7)) // deploy verification_contract.go
	require.NoError(t, err)
	require.Equal(t, 1, len(blockDeploy2.Transactions))
	txDeploy2 := blockDeploy2.Transactions[0]

	blockSendRubles, err := e.chain.GetBlock(e.chain.GetHeaderHash(6))
	require.NoError(t, err)
	require.Equal(t, 1, len(blockSendRubles.Transactions))
	txSendRubles := blockSendRubles.Transactions[0]
	blockGASBounty1 := blockSendRubles // index 6 = size of committee

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
	expected := result.NEP17Transfers{
		Sent: []result.NEP17Transfer{
			{
				Timestamp: blockWithFAULTedTx.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txFAULTed.SystemFee + txFAULTed.NetworkFee).String(),
				Index:     23,
				TxHash:    blockWithFAULTedTx.Hash(),
			},
			{
				Timestamp: blockDeploy6.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txDeploy6.SystemFee + txDeploy6.NetworkFee).String(),
				Index:     22,
				TxHash:    blockDeploy6.Hash(),
			},
			{
				Timestamp: blockTransferNFSO.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txTransferNFSO.SystemFee + txTransferNFSO.NetworkFee).String(),
				Index:     19,
				TxHash:    blockTransferNFSO.Hash(),
			},
			{
				Timestamp:   blockMintNFSO.Timestamp,
				Asset:       e.chain.UtilityTokenHash(),
				Address:     address.Uint160ToString(nfsoHash),
				Amount:      "1000000000",
				Index:       18,
				NotifyIndex: 0,
				TxHash:      txMintNFSO.Hash(),
			},
			{
				Timestamp: blockMintNFSO.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txMintNFSO.SystemFee + txMintNFSO.NetworkFee).String(),
				Index:     18,
				TxHash:    blockMintNFSO.Hash(),
			},
			{
				Timestamp: blockDeploy5.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txDeploy5.SystemFee + txDeploy5.NetworkFee).String(),
				Index:     17,
				TxHash:    blockDeploy5.Hash(),
			},
			{
				Timestamp: blockPutNewTestValue.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txPutValue3.SystemFee + txPutValue3.NetworkFee).String(),
				Index:     16,
				TxHash:    blockPutNewTestValue.Hash(),
			},
			{
				Timestamp: blockPutNewTestValue.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txPutValue2.SystemFee + txPutValue2.NetworkFee).String(),
				Index:     16,
				TxHash:    blockPutNewTestValue.Hash(),
			},
			{
				Timestamp: blockPutNewTestValue.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txPutValue1.SystemFee + txPutValue1.NetworkFee).String(),
				Index:     16,
				TxHash:    blockPutNewTestValue.Hash(),
			},
			{
				Timestamp: blockPutNewTestValue.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txPutNewTestValue.SystemFee + txPutNewTestValue.NetworkFee).String(),
				Index:     16,
				TxHash:    blockPutNewTestValue.Hash(),
			},
			{
				Timestamp: blockSetRecord.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txSetRecord.SystemFee + txSetRecord.NetworkFee).String(),
				Index:     15,
				TxHash:    blockSetRecord.Hash(),
			},
			{
				Timestamp: blockRegisterDomain.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txRegisterDomain.SystemFee + txRegisterDomain.NetworkFee).String(),
				Index:     14,
				TxHash:    blockRegisterDomain.Hash(),
			},
			{
				Timestamp: blockDeploy4.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txDeploy4.SystemFee + txDeploy4.NetworkFee).String(),
				Index:     11,
				TxHash:    blockDeploy4.Hash(),
			},
			{
				Timestamp: blockDeploy3.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txDeploy3.SystemFee + txDeploy3.NetworkFee).String(),
				Index:     10,
				TxHash:    blockDeploy3.Hash(),
			},
			{
				Timestamp:   blockDepositGAS.Timestamp,
				Asset:       e.chain.UtilityTokenHash(),
				Address:     address.Uint160ToString(e.chain.GetNotaryContractScriptHash()),
				Amount:      "1000000000",
				Index:       8,
				NotifyIndex: 0,
				TxHash:      txDepositGAS.Hash(),
			},
			{
				Timestamp: blockDepositGAS.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txDepositGAS.SystemFee + txDepositGAS.NetworkFee).String(),
				Index:     8,
				TxHash:    blockDepositGAS.Hash(),
			},
			{
				Timestamp: blockDeploy2.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txDeploy2.SystemFee + txDeploy2.NetworkFee).String(),
				Index:     7,
				TxHash:    blockDeploy2.Hash(),
			},
			{
				Timestamp:   blockSendRubles.Timestamp,
				Asset:       rublesHash,
				Address:     testchain.PrivateKeyByID(1).Address(),
				Amount:      "123",
				Index:       6,
				NotifyIndex: 0,
				TxHash:      txSendRubles.Hash(),
			},
			{
				Timestamp: blockSendRubles.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txSendRubles.SystemFee + txSendRubles.NetworkFee).String(),
				Index:     6,
				TxHash:    blockSendRubles.Hash(),
			},
			{
				Timestamp: blockReceiveRubles.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txReceiveRubles.SystemFee + txReceiveRubles.NetworkFee).String(),
				Index:     5,
				TxHash:    blockReceiveRubles.Hash(),
			},
			{
				Timestamp: blockReceiveRubles.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn
				Amount:    big.NewInt(txInitCall.SystemFee + txInitCall.NetworkFee).String(),
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
				Amount:    big.NewInt(txSendNEO.SystemFee + txSendNEO.NetworkFee).String(),
				Index:     4,
				TxHash:    blockSendNEO.Hash(),
			},
			{
				Timestamp: blockCtrInv1.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn has empty receiver
				Amount:    big.NewInt(txCtrInv1.SystemFee + txCtrInv1.NetworkFee).String(),
				Index:     3,
				TxHash:    blockCtrInv1.Hash(),
			},
			{
				Timestamp: blockCtrDeploy.Timestamp,
				Asset:     e.chain.UtilityTokenHash(),
				Address:   "", // burn has empty receiver
				Amount:    big.NewInt(txCtrDeploy.SystemFee + txCtrDeploy.NetworkFee).String(),
				Index:     2,
				TxHash:    blockCtrDeploy.Hash(),
			},
		},
		Received: []result.NEP17Transfer{
			{
				Timestamp:   blockMintNFSO.Timestamp, // GAS bounty
				Asset:       e.chain.UtilityTokenHash(),
				Address:     "",
				Amount:      "50000000",
				Index:       18,
				NotifyIndex: 0,
				TxHash:      blockMintNFSO.Hash(),
			},
			{
				Timestamp:   blockGASBounty2.Timestamp,
				Asset:       e.chain.UtilityTokenHash(),
				Address:     "",
				Amount:      "50000000",
				Index:       12,
				NotifyIndex: 0,
				TxHash:      blockGASBounty2.Hash(),
			},
			{
				Timestamp:   blockGASBounty1.Timestamp,
				Asset:       e.chain.UtilityTokenHash(),
				Address:     "",
				Amount:      "50000000",
				Index:       6,
				NotifyIndex: 0,
				TxHash:      blockGASBounty1.Hash(),
			},
			{
				Timestamp:   blockReceiveRubles.Timestamp,
				Asset:       rublesHash,
				Address:     address.Uint160ToString(rublesHash),
				Amount:      "1000",
				Index:       5,
				NotifyIndex: 0,
				TxHash:      txReceiveRubles.Hash(),
			},
			{
				Timestamp:   blockSendNEO.Timestamp,
				Asset:       e.chain.UtilityTokenHash(),
				Address:     "", // Minted GAS.
				Amount:      "149998500",
				Index:       4,
				NotifyIndex: 0,
				TxHash:      txSendNEO.Hash(),
			},
			{
				Timestamp:   blockReceiveGAS.Timestamp,
				Asset:       e.chain.UtilityTokenHash(),
				Address:     testchain.MultisigAddress(),
				Amount:      "100000000000",
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
				Amount:    "50000000",
				Index:     0,
				TxHash:    blockGASBounty0.Hash(),
			},
		},
		Address: testchain.PrivateKeyByID(0).Address(),
	}

	require.Equal(t, expected.Address, res.Address)

	arr := make([]result.NEP17Transfer, 0, len(expected.Sent))
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

func TestEscapeForLog(t *testing.T) {
	in := "\n\tbad"
	require.Equal(t, "bad", escapeForLog(in))
}

func BenchmarkHandleIn(b *testing.B) {
	chain, orc, cfg, logger := getUnitTestChain(b, false, false, false)

	serverConfig, err := network.NewServerConfig(cfg)
	require.NoError(b, err)
	serverConfig.UserAgent = fmt.Sprintf(config.UserAgentFormat, "0.98.6-test")
	serverConfig.LogLevel = zapcore.FatalLevel
	server, err := network.NewServer(serverConfig, chain, chain.GetStateSyncModule(), logger)
	require.NoError(b, err)
	rpcServer := New(chain, cfg.ApplicationConfiguration.RPC, server, orc, logger, make(chan error))
	defer chain.Close()

	do := func(b *testing.B, req []byte) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			in := new(params.In)
			b.StartTimer()
			err := json.Unmarshal(req, in)
			if err != nil {
				b.FailNow()
			}

			res := rpcServer.handleIn(in, nil)
			if res.Error != nil {
				b.FailNow()
			}
		}
		b.StopTimer()
	}

	b.Run("no extra params", func(b *testing.B) {
		do(b, []byte(`{"jsonrpc":"2.0", "method":"validateaddress","params":["Nbb1qkwcwNSBs9pAnrVVrnFbWnbWBk91U2"]}`))
	})

	b.Run("with extra params", func(b *testing.B) {
		do(b, []byte(`{"jsonrpc":"2.0", "method":"validateaddress","params":["Nbb1qkwcwNSBs9pAnrVVrnFbWnbWBk91U2", 
"set", "of", "different", "parameters", "to", "see", "the", "difference", "between", "unmarshalling", "algorithms", 1234, 5678, 1234567, 765432, true, false, null,
"0x50befd26fdf6e4d957c11e078b24ebce6291456f", "someMethod", [{"type": "String", "value": "50befd26fdf6e4d957c11e078b24ebce6291456f"}, 
{"type": "Integer", "value": "42"}, {"type": "Boolean", "value": false}]]}`))
	})
}

func TestFailedPreconditionShutdown(t *testing.T) {
	_, srv, _ := initClearServerWithCustomConfig(t, func(c *config.Config) {
		c.ApplicationConfiguration.RPC.Addresses = []string{"not an address"}
	})

	srv.Start()
	require.Positive(t, len(srv.errChan)) // this is how Start reports internal failures

	var stopped atomic.Bool

	go func() {
		srv.Shutdown()
		stopped.Store(true)
	}()

	require.Eventually(t, stopped.Load, 5*time.Second, 100*time.Millisecond, "Shutdown should return")
}

func TestErrorResponseContentType(t *testing.T) {
	chain, rpcSrv, httpSrv := initClearServerWithServices(t, true, false, false)
	defer chain.Close()
	defer rpcSrv.Shutdown()

	const (
		expectedContentType = "application/json; charset=utf-8"
		req                 = `{"jsonrpc":"2.0", "method":"unknown","params":[]}`
	)

	cl := http.Client{Timeout: time.Second}
	resp, err := cl.Post(httpSrv.URL, "application/json", strings.NewReader(req))
	require.NoErrorf(t, err, "could not make a POST request")
	resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	require.Equal(t, expectedContentType, contentType)
}
