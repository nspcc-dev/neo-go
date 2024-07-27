package native_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/require"
)

func newLedgerClient(t *testing.T) *neotest.ContractInvoker {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(cfg *config.Blockchain) {
		cfg.MaxTraceableBlocks = 10 // reduce number of traceable blocks for Ledger tests
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	return e.CommitteeInvoker(e.NativeHash(t, nativenames.Ledger))
}

func TestLedger_GetTransactionHeight(t *testing.T) {
	c := newLedgerClient(t)
	e := c.Executor
	ledgerInvoker := c.WithSigners(c.Committee)

	height := 13
	e.GenerateNewBlocks(t, height-1)
	hash := e.InvokeScript(t, []byte{byte(opcode.RET)}, []neotest.Signer{c.Committee})

	t.Run("good", func(t *testing.T) {
		ledgerInvoker.Invoke(t, height, "getTransactionHeight", hash)
	})
	t.Run("unknown transaction", func(t *testing.T) {
		ledgerInvoker.Invoke(t, -1, "getTransactionHeight", util.Uint256{1, 2, 3})
	})
	t.Run("not a hash", func(t *testing.T) {
		ledgerInvoker.InvokeFail(t, "expected []byte of size 32", "getTransactionHeight", []byte{1, 2, 3})
	})
}

func TestLedger_GetTransactionState(t *testing.T) {
	c := newLedgerClient(t)
	e := c.Executor
	ledgerInvoker := c.WithSigners(c.Committee)

	hash := e.InvokeScript(t, []byte{byte(opcode.RET)}, []neotest.Signer{c.Committee})

	t.Run("unknown transaction", func(t *testing.T) {
		ledgerInvoker.Invoke(t, vmstate.None, "getTransactionVMState", util.Uint256{1, 2, 3})
	})
	t.Run("not a hash", func(t *testing.T) {
		ledgerInvoker.InvokeFail(t, "expected []byte of size 32", "getTransactionVMState", []byte{1, 2, 3})
	})
	t.Run("good: HALT", func(t *testing.T) {
		ledgerInvoker.Invoke(t, vmstate.Halt, "getTransactionVMState", hash)
	})
	t.Run("isn't traceable", func(t *testing.T) {
		// Add more blocks so that tx becomes untraceable.
		e.GenerateNewBlocks(t, int(e.Chain.GetConfig().MaxTraceableBlocks))
		ledgerInvoker.Invoke(t, vmstate.None, "getTransactionVMState", hash)
	})
	t.Run("good: FAULT", func(t *testing.T) {
		faultedH := e.InvokeScript(t, []byte{byte(opcode.ABORT)}, []neotest.Signer{c.Committee})
		ledgerInvoker.Invoke(t, vmstate.Fault, "getTransactionVMState", faultedH)
	})
}

func TestLedger_GetTransaction(t *testing.T) {
	c := newLedgerClient(t)
	e := c.Executor
	ledgerInvoker := c.WithSigners(c.Committee)

	hash := e.InvokeScript(t, []byte{byte(opcode.RET)}, []neotest.Signer{c.Committee})
	tx, _ := e.GetTransaction(t, hash)

	t.Run("success", func(t *testing.T) {
		ledgerInvoker.Invoke(t, []stackitem.Item{
			stackitem.NewByteArray(tx.Hash().BytesBE()),
			stackitem.NewBigInteger(big.NewInt(int64(tx.Version))),
			stackitem.NewBigInteger(big.NewInt(int64(tx.Nonce))),
			stackitem.NewByteArray(tx.Sender().BytesBE()),
			stackitem.NewBigInteger(big.NewInt(tx.SystemFee)),
			stackitem.NewBigInteger(big.NewInt(tx.NetworkFee)),
			stackitem.NewBigInteger(big.NewInt(int64(tx.ValidUntilBlock))),
			stackitem.NewByteArray(tx.Script),
		}, "getTransaction", tx.Hash())
	})
	t.Run("isn't traceable", func(t *testing.T) {
		// Add more blocks so that tx becomes untraceable.
		e.GenerateNewBlocks(t, int(e.Chain.GetConfig().MaxTraceableBlocks))
		ledgerInvoker.Invoke(t, stackitem.Null{}, "getTransaction", tx.Hash())
	})
	t.Run("bad hash", func(t *testing.T) {
		ledgerInvoker.Invoke(t, stackitem.Null{}, "getTransaction", util.Uint256{})
	})
}

func TestLedger_GetTransactionFromBlock(t *testing.T) {
	c := newLedgerClient(t)
	e := c.Executor
	ledgerInvoker := c.WithSigners(c.Committee)

	ledgerInvoker.Invoke(t, e.Chain.BlockHeight(), "currentIndex") // Adds a block.
	b := e.GetBlockByIndex(t, e.Chain.BlockHeight())

	check := func(t testing.TB, stack []stackitem.Item) {
		require.Equal(t, 1, len(stack))
		actual, ok := stack[0].Value().([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, b.Transactions[0].Hash().BytesBE(), actual[0].Value().([]byte))
	}
	t.Run("good, by hash", func(t *testing.T) {
		ledgerInvoker.InvokeAndCheck(t, check, "getTransactionFromBlock", b.Hash(), int64(0))
	})
	t.Run("good, by index", func(t *testing.T) {
		ledgerInvoker.InvokeAndCheck(t, check, "getTransactionFromBlock", int64(b.Index), int64(0))
	})
	t.Run("bad transaction index", func(t *testing.T) {
		ledgerInvoker.InvokeFail(t, "", "getTransactionFromBlock", b.Hash(), int64(1))
	})
	t.Run("bad block hash (>int64)", func(t *testing.T) {
		ledgerInvoker.InvokeFail(t, "", "getTransactionFromBlock", b.Hash().BytesBE()[:10], int64(0))
	})
	t.Run("invalid block hash (int64)", func(t *testing.T) {
		ledgerInvoker.InvokeFail(t, "", "getTransactionFromBlock", b.Hash().BytesBE()[:6], int64(0))
	})
	t.Run("unknown block hash", func(t *testing.T) {
		ledgerInvoker.Invoke(t, stackitem.Null{}, "getTransactionFromBlock", b.Hash().BytesLE(), int64(0))
	})
	t.Run("isn't traceable", func(t *testing.T) {
		e.GenerateNewBlocks(t, int(e.Chain.GetConfig().MaxTraceableBlocks))
		ledgerInvoker.Invoke(t, stackitem.Null{}, "getTransactionFromBlock", b.Hash(), int64(0))
	})
}

func TestLedger_GetBlock(t *testing.T) {
	c := newLedgerClient(t)
	e := c.Executor
	ledgerInvoker := c.WithSigners(c.Committee)

	ledgerInvoker.Invoke(t, e.Chain.GetHeaderHash(e.Chain.BlockHeight()).BytesBE(), "currentHash") // Adds a block.
	b := e.GetBlockByIndex(t, e.Chain.BlockHeight())

	expected := []stackitem.Item{
		stackitem.NewByteArray(b.Hash().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.Version))),
		stackitem.NewByteArray(b.PrevHash.BytesBE()),
		stackitem.NewByteArray(b.MerkleRoot.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.Timestamp))),
		stackitem.NewBigInteger(big.NewInt(int64(b.Nonce))),
		stackitem.NewBigInteger(big.NewInt(int64(b.Index))),
		stackitem.NewBigInteger(big.NewInt(int64(b.PrimaryIndex))),
		stackitem.NewByteArray(b.NextConsensus.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(len(b.Transactions)))),
	}
	t.Run("good, by hash", func(t *testing.T) {
		ledgerInvoker.Invoke(t, expected, "getBlock", b.Hash())
	})
	t.Run("good, by index", func(t *testing.T) {
		ledgerInvoker.Invoke(t, expected, "getBlock", int64(b.Index))
	})
	t.Run("bad hash", func(t *testing.T) {
		ledgerInvoker.Invoke(t, stackitem.Null{}, "getBlock", b.Hash().BytesLE())
	})
	t.Run("isn't traceable", func(t *testing.T) {
		e.GenerateNewBlocks(t, int(e.Chain.GetConfig().MaxTraceableBlocks))
		ledgerInvoker.Invoke(t, stackitem.Null{}, "getBlock", b.Hash())
	})
}

func TestLedger_GetTransactionSigners(t *testing.T) {
	c := newLedgerClient(t)
	e := c.Executor
	ledgerInvoker := c.WithSigners(c.Committee)

	txHash := ledgerInvoker.Invoke(t, e.Chain.BlockHeight(), "currentIndex")

	t.Run("good", func(t *testing.T) {
		s := &transaction.Signer{
			Account: c.CommitteeHash,
			Scopes:  transaction.Global,
		}
		expected := stackitem.NewArray([]stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewByteArray(s.Account.BytesBE()),
				stackitem.NewBigInteger(big.NewInt(int64(s.Scopes))),
				stackitem.NewArray([]stackitem.Item{}),
				stackitem.NewArray([]stackitem.Item{}),
				stackitem.NewArray([]stackitem.Item{}),
			}),
		})
		ledgerInvoker.Invoke(t, expected, "getTransactionSigners", txHash)
	})
	t.Run("unknown transaction", func(t *testing.T) {
		ledgerInvoker.Invoke(t, stackitem.Null{}, "getTransactionSigners", util.Uint256{1, 2, 3})
	})
	t.Run("not a hash", func(t *testing.T) {
		ledgerInvoker.InvokeFail(t, "expected []byte of size 32", "getTransactionSigners", []byte{1, 2, 3})
	})
}

func TestLedger_GetTransactionSignersInteropAPI(t *testing.T) {
	c := newLedgerClient(t)
	e := c.Executor
	ledgerInvoker := c.WithSigners(c.Committee)

	// Firstly, add transaction with CalledByEntry rule-based signer scope to the chain.
	tx := e.NewUnsignedTx(t, ledgerInvoker.Hash, "currentIndex")
	tx.Signers = []transaction.Signer{{
		Account: c.Committee.ScriptHash(),
		Scopes:  transaction.Rules,
		Rules: []transaction.WitnessRule{
			{
				Action:    transaction.WitnessAllow,
				Condition: transaction.ConditionCalledByEntry{},
			},
		},
	}}
	neotest.AddNetworkFee(t, e.Chain, tx, c.Committee)
	neotest.AddSystemFee(e.Chain, tx, -1)
	require.NoError(t, c.Committee.SignTx(e.Chain.GetConfig().Magic, tx))
	c.AddNewBlock(t, tx)
	c.CheckHalt(t, tx.Hash(), stackitem.Make(e.Chain.BlockHeight()-1))

	var (
		hashStr string
		accStr  string
		txHash  = tx.Hash().BytesBE()
		acc     = c.Committee.ScriptHash().BytesBE()
	)
	for i := 0; i < util.Uint256Size; i++ {
		hashStr += fmt.Sprintf("%#x", txHash[i])
		if i != util.Uint256Size-1 {
			hashStr += ", "
		}
	}
	for i := 0; i < util.Uint160Size; i++ {
		accStr += fmt.Sprintf("%#x", acc[i])
		if i != util.Uint160Size-1 {
			accStr += ", "
		}
	}

	// After that ensure interop API allows to retrieve signer with CalledByEntry rule-based scope.
	src := `package callledger
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/native/ledger"
			"github.com/nspcc-dev/neo-go/pkg/interop"
			"github.com/nspcc-dev/neo-go/pkg/interop/util"
		)
		func CallLedger(accessValue bool) int {
			signers := ledger.GetTransactionSigners(interop.Hash256{` + hashStr + `})
			if len(signers) != 1 {
				panic("bad length")
			}
			s0 := signers[0]
			expectedAcc := interop.Hash160{` + accStr + `}
			if !util.Equals(string(s0.Account), string(expectedAcc)) {
				panic("bad account")
			}
			if s0.Scopes != ledger.Rules {
				panic("bad signer scope")
			}
			if len(s0.Rules) != 1 {
				panic("bad rules length")
			}
			r0 := s0.Rules[0]
			if r0.Action != ledger.WitnessAllow {
				panic("bad action")
			}
			c0 := r0.Condition
			if c0.Type != ledger.WitnessCalledByEntry {
				panic("bad condition type")
			}
			if accessValue {
				// Panic should occur here, because there's only Type inside the CalledByEntry condition.
				_ = c0.Value
			}
			return 1
		}`
	ctr := neotest.CompileSource(t, c.Committee.ScriptHash(), strings.NewReader(src), &compiler.Options{
		Name: "calledger_contract",
	})
	e.DeployContract(t, ctr, nil)

	ctrInvoker := e.NewInvoker(ctr.Hash, e.Committee)
	ctrInvoker.Invoke(t, 1, "callLedger", false)                                                                    // Firstly, don't access CalledByEnrty Condition value => the call should be successful.
	ctrInvoker.InvokeFail(t, `(PICKITEM): unhandled exception: "The value 1 is out of range."`, "callLedger", true) // Then, access the value to ensure it will panic.
}
