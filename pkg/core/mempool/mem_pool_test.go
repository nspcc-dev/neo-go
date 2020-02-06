package mempool

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/internal/random"
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
	_, ok := mp.TryGetValue(tx.Hash())
	require.Equal(t, false, ok)
	require.NoError(t, mp.Add(tx, fs))
	// Re-adding should fail.
	require.Error(t, mp.Add(tx, fs))
	tx2, ok := mp.TryGetValue(tx.Hash())
	require.Equal(t, true, ok)
	require.Equal(t, tx, tx2)
	mp.Remove(tx.Hash())
	_, ok = mp.TryGetValue(tx.Hash())
	require.Equal(t, false, ok)
	// Make sure nothing left in the mempool after removal.
	assert.Equal(t, 0, len(mp.verifiedMap))
	assert.Equal(t, 0, len(mp.verifiedTxes))
}

func TestMemPoolAddRemove(t *testing.T) {
	var fs = &FeerStub{lowPriority: false}
	t.Run("low priority", func(t *testing.T) { testMemPoolAddRemoveWithFeer(t, fs) })
	fs.lowPriority = true
	t.Run("high priority", func(t *testing.T) { testMemPoolAddRemoveWithFeer(t, fs) })
}

func TestMemPoolVerify(t *testing.T) {
	mp := NewMemPool(10)
	tx := newMinerTX()
	inhash1 := random.Uint256()
	tx.Inputs = append(tx.Inputs, transaction.Input{PrevHash: inhash1, PrevIndex: 0})
	require.Equal(t, true, mp.Verify(tx))
	require.NoError(t, mp.Add(tx, &FeerStub{}))

	tx2 := newMinerTX()
	inhash2 := random.Uint256()
	tx2.Inputs = append(tx2.Inputs, transaction.Input{PrevHash: inhash2, PrevIndex: 0})
	require.Equal(t, true, mp.Verify(tx2))
	require.NoError(t, mp.Add(tx2, &FeerStub{}))

	tx3 := newMinerTX()
	// Different index number, but the same PrevHash as in tx1.
	tx3.Inputs = append(tx3.Inputs, transaction.Input{PrevHash: inhash1, PrevIndex: 1})
	require.Equal(t, true, mp.Verify(tx3))
	// The same input as in tx2.
	tx3.Inputs = append(tx3.Inputs, transaction.Input{PrevHash: inhash2, PrevIndex: 0})
	require.Equal(t, false, mp.Verify(tx3))
	require.Error(t, mp.Add(tx3, &FeerStub{}))
}

func newMinerTX() *transaction.Transaction {
	return &transaction.Transaction{
		Type: transaction.MinerType,
		Data: &transaction.MinerTX{},
	}
}
