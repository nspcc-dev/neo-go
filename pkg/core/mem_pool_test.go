package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FeerStub struct {
	lowPriority bool
	sysFee      util.Fixed8
	netFee      util.Fixed8
	perByteFee  util.Fixed8
}

func (fs *FeerStub) NetworkFee(*transaction.Transaction) util.Fixed8 {
	return fs.netFee
}

func (fs *FeerStub) IsLowPriority(*transaction.Transaction) bool {
	return fs.lowPriority
}

func (fs *FeerStub) FeePerByte(*transaction.Transaction) util.Fixed8 {
	return fs.perByteFee
}

func (fs *FeerStub) SystemFee(*transaction.Transaction) util.Fixed8 {
	return fs.sysFee
}

func testMemPoolAddRemoveWithFeer(t *testing.T, fs Feer) {
	mp := NewMemPool(10)
	tx := newMinerTX()
	item := NewPoolItem(tx, fs)
	_, ok := mp.TryGetValue(tx.Hash())
	require.Equal(t, false, ok)
	require.Equal(t, true, mp.TryAdd(tx.Hash(), item))
	// Re-adding should fail.
	require.Equal(t, false, mp.TryAdd(tx.Hash(), item))
	tx2, ok := mp.TryGetValue(tx.Hash())
	require.Equal(t, true, ok)
	require.Equal(t, tx, tx2)
	mp.Remove(tx.Hash())
	_, ok = mp.TryGetValue(tx.Hash())
	require.Equal(t, false, ok)
	// Make sure nothing left in the mempool after removal.
	assert.Equal(t, 0, len(mp.unsortedTxn))
	assert.Equal(t, 0, len(mp.unverifiedTxn))
	assert.Equal(t, 0, len(mp.sortedHighPrioTxn))
	assert.Equal(t, 0, len(mp.sortedLowPrioTxn))
	assert.Equal(t, 0, len(mp.unverifiedSortedHighPrioTxn))
	assert.Equal(t, 0, len(mp.unverifiedSortedLowPrioTxn))
}

func TestMemPoolAddRemove(t *testing.T) {
	var fs = &FeerStub{lowPriority: false}
	t.Run("low priority", func(t *testing.T) { testMemPoolAddRemoveWithFeer(t, fs) })
	fs.lowPriority = true
	t.Run("high priority", func(t *testing.T) { testMemPoolAddRemoveWithFeer(t, fs) })
}
