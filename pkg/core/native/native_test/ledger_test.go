package native_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newLedgerClient(t *testing.T) *neotest.ContractInvoker {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(cfg *config.ProtocolConfiguration) {
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
	b := e.GetBlockByIndex(t, int(e.Chain.BlockHeight()))

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

	ledgerInvoker.Invoke(t, e.Chain.GetHeaderHash(int(e.Chain.BlockHeight())).BytesBE(), "currentHash") // Adds a block.
	b := e.GetBlockByIndex(t, int(e.Chain.BlockHeight()))

	expected := []stackitem.Item{
		stackitem.NewByteArray(b.Hash().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.Version))),
		stackitem.NewByteArray(b.PrevHash.BytesBE()),
		stackitem.NewByteArray(b.MerkleRoot.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.Timestamp))),
		stackitem.NewBigInteger(big.NewInt(int64(b.Nonce))),
		stackitem.NewBigInteger(big.NewInt(int64(b.Index))),
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
