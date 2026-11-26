package benchmarks

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

// --- LedgerContract.CurrentHash ---

func genLedgerCurrentHash(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		_, ic, md := newNativeContext(b, nativenames.Ledger)
		ic.VM.LoadScript(nil)
		method, ok := md.GetMethod("currentHash", 0)
		require.True(b, ok)
		benchNative(b, ic, method)
	})
	return ns / nopNs
}

// --- LedgerContract.CurrentIndex ---

func genLedgerCurrentIndex(t *testing.T, nopNs float64) float64 {
	ns := measureNsPerOp(t, func(b *testing.B) {
		_, ic, md := newNativeContext(b, nativenames.Ledger)
		ic.VM.LoadScript(nil)
		method, ok := md.GetMethod("currentIndex", 0)
		require.True(b, ok)
		benchNative(b, ic, method)
	})
	return ns / nopNs
}

// --- LedgerContract.GetBlock ---

func genLedgerGetBlockTxCount(t *testing.T, nopNs float64) []sample {
	e, signer := newPriceExecutor(t)
	tx := newTestTx(t, e, signer, txParams{})
	return sampleRange(0, getMaxTxCount(t, e, tx), func(curr int) sample {
		return measureLedgerGetBlock(t, nopNs, e, signer, tx, curr)
	})
}

func genLedgerGetBlockSize(t *testing.T, nopNs float64) []sample {
	e, signer := newPriceExecutor(t)
	header := e.SignBlock(e.NewUnsignedBlock(t))
	targetSize := (int(e.Chain.GetConfig().MaxBlockSize) - header.GetExpectedBlockSizeWithoutTransactions(countPoints)) / countPoints
	tx := newTestTx(t, e, signer, txParams{})
	rules := newLimitedWitnessRules(targetSize - tx.Size())
	tx = newTestTx(t, e, signer, txParams{rules: rules})
	require.NotEmpty(t, rules)

	rows := make([]sample, 0, countPoints)
	curr := 1
	for range countPoints {
		rows = append(rows, measureLedgerGetBlock(t, nopNs, e, signer, tx, curr))
		curr++
	}
	return rows
}

func genLedgerGetBlockScriptLen(t *testing.T, nopNs float64) []sample {
	e, signer := newPriceExecutor(t)
	tx := newTestTx(t, e, signer, txParams{scriptlen: transaction.MaxScriptLength})
	txCount := getMaxTxCount(t, e, tx)
	return sampleRange(1, transaction.MaxScriptLength, func(curr int) sample {
		tx := newTestTx(t, e, signer, txParams{scriptlen: curr})
		return measureLedgerGetBlock(t, nopNs, e, signer, tx, txCount)
	})
}

func measureLedgerGetBlock(t *testing.T, nopNs float64, e *neotest.Executor, signer neotest.Signer, tx *transaction.Transaction, txCount int) sample {
	txs := make([]*transaction.Transaction, txCount)
	for i := range txs {
		txs[i] = tx.Copy()
		txs[i].Nonce = neotest.Nonce()
		txs[i].ValidUntilBlock = e.Chain.BlockHeight() + 1
		require.NoError(t, signer.SignTx(e.Chain.GetConfig().Magic, txs[i]))
	}
	blk := e.AddNewBlock(t, txs...)
	// The latest block is returned directly from cache. Read an older block to
	// include trimmed block and transaction deserialization.
	e.AddNewBlock(t)
	ns := measureNsPerOp(t, func(b *testing.B) {
		_, ic, md := newNativeContextForChain(b, e.Chain, nativenames.Ledger)
		ic.VM.LoadScript(nil)
		method, ok := md.GetMethod("getBlock", 1)
		require.True(b, ok)
		hash := stackitem.NewByteArray(blk.Hash().BytesBE())
		benchNative(b, ic, method, hash)
	})
	return sample{deps: []int{len(blk.Transactions), blk.GetExpectedBlockSize()}, value: ns / nopNs}
}

// --- LedgerContract.GetTransaction ---

func genLedgerGetTransactionScriptLen(t *testing.T, nopNs float64) []sample {
	e, signer := newPriceExecutor(t)
	return sampleRange(1, transaction.MaxScriptLength, func(curr int) sample {
		tx := newTestTx(t, e, signer, txParams{scriptlen: curr})
		return measureLedgerGetTransaction(t, nopNs, e, tx)
	})
}

func genLedgerGetTransactionSize(t *testing.T, nopNs float64) []sample {
	e, signer := newPriceExecutor(t)
	minTx := newTestTx(t, e, signer, txParams{})
	rules := newLimitedWitnessRules(transaction.MaxTransactionSize - minTx.Size())
	maxTx := newTestTx(t, e, signer, txParams{rules: rules})
	return sampleRange(0, maxTx.Size()-minTx.Size(), func(curr int) sample {
		tx := newTestTx(t, e, signer, txParams{rules: newLimitedWitnessRules(curr)})
		return measureLedgerGetTransaction(t, nopNs, e, tx)
	})
}

func measureLedgerGetTransaction(t *testing.T, nopNs float64, e *neotest.Executor, tx *transaction.Transaction) sample {
	e.AddNewBlock(t, tx)
	ns := measureNsPerOp(t, func(b *testing.B) {
		_, ic, md := newNativeContextForChain(b, e.Chain, nativenames.Ledger)
		ic.VM.LoadScript(nil)
		method, ok := md.GetMethod("getTransaction", 1)
		require.True(b, ok)
		hash := stackitem.NewByteArray(tx.Hash().BytesBE())
		benchNative(b, ic, method, hash)
	})
	return sample{deps: []int{tx.Size()}, value: ns / nopNs}
}
