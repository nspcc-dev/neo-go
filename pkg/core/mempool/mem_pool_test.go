package mempool

import (
	"sort"
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

func (fs *FeerStub) IsLowPriority(util.Fixed8) bool {
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
	tx := newMinerTX(0)
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
	tx := newMinerTX(1)
	inhash1 := random.Uint256()
	tx.Inputs = append(tx.Inputs, transaction.Input{PrevHash: inhash1, PrevIndex: 0})
	require.Equal(t, true, mp.Verify(tx))
	require.NoError(t, mp.Add(tx, &FeerStub{}))

	tx2 := newMinerTX(2)
	inhash2 := random.Uint256()
	tx2.Inputs = append(tx2.Inputs, transaction.Input{PrevHash: inhash2, PrevIndex: 0})
	require.Equal(t, true, mp.Verify(tx2))
	require.NoError(t, mp.Add(tx2, &FeerStub{}))

	tx3 := newMinerTX(3)
	// Different index number, but the same PrevHash as in tx1.
	tx3.Inputs = append(tx3.Inputs, transaction.Input{PrevHash: inhash1, PrevIndex: 1})
	require.Equal(t, true, mp.Verify(tx3))
	// The same input as in tx2.
	tx3.Inputs = append(tx3.Inputs, transaction.Input{PrevHash: inhash2, PrevIndex: 0})
	require.Equal(t, false, mp.Verify(tx3))
	require.Error(t, mp.Add(tx3, &FeerStub{}))
}

func newMinerTX(i uint32) *transaction.Transaction {
	return &transaction.Transaction{
		Type: transaction.MinerType,
		Data: &transaction.MinerTX{Nonce: i},
	}
}

func TestOverCapacity(t *testing.T) {
	var fs = &FeerStub{lowPriority: true}
	const mempoolSize = 10
	mp := NewMemPool(mempoolSize)

	for i := 0; i < mempoolSize; i++ {
		tx := newMinerTX(uint32(i))
		require.NoError(t, mp.Add(tx, fs))
	}
	txcnt := uint32(mempoolSize)
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// Claim TX has more priority than ordinary lowprio, so it should easily
	// fit into the pool.
	claim := &transaction.Transaction{
		Type: transaction.ClaimType,
		Data: &transaction.ClaimTX{},
	}
	require.NoError(t, mp.Add(claim, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// Fees are also prioritized.
	fs.netFee = util.Fixed8FromFloat(0.0001)
	for i := 0; i < mempoolSize-1; i++ {
		tx := newMinerTX(txcnt)
		txcnt++
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
	}
	// Less prioritized txes are not allowed anymore.
	fs.netFee = util.Fixed8FromFloat(0.00001)
	tx := newMinerTX(txcnt)
	txcnt++
	require.Error(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// But claim tx should still be there.
	require.True(t, mp.ContainsKey(claim.Hash()))

	// Low net fee, but higher per-byte fee is still a better combination.
	fs.perByteFee = util.Fixed8FromFloat(0.001)
	tx = newMinerTX(txcnt)
	txcnt++
	require.NoError(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// High priority always wins over low priority.
	fs.lowPriority = false
	for i := 0; i < mempoolSize; i++ {
		tx := newMinerTX(txcnt)
		txcnt++
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
	}
	// Good luck with low priority now.
	fs.lowPriority = true
	tx = newMinerTX(txcnt)
	require.Error(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
}

func TestGetVerified(t *testing.T) {
	var fs = &FeerStub{lowPriority: true}
	const mempoolSize = 10
	mp := NewMemPool(mempoolSize)

	txes := make([]*transaction.Transaction, 0, mempoolSize)
	for i := 0; i < mempoolSize; i++ {
		tx := newMinerTX(uint32(i))
		txes = append(txes, tx)
		require.NoError(t, mp.Add(tx, fs))
	}
	require.Equal(t, mempoolSize, mp.Count())
	verTxes := mp.GetVerifiedTransactions()
	require.Equal(t, mempoolSize, len(verTxes))
	for _, tx := range verTxes {
		require.Contains(t, txes, tx)
	}
	for _, tx := range txes {
		mp.Remove(tx.Hash())
	}
	verTxes = mp.GetVerifiedTransactions()
	require.Equal(t, 0, len(verTxes))
}

func TestRemoveStale(t *testing.T) {
	var fs = &FeerStub{lowPriority: true}
	const mempoolSize = 10
	mp := NewMemPool(mempoolSize)

	txes1 := make([]*transaction.Transaction, 0, mempoolSize/2)
	txes2 := make([]*transaction.Transaction, 0, mempoolSize/2)
	for i := 0; i < mempoolSize; i++ {
		tx := newMinerTX(uint32(i))
		if i%2 == 0 {
			txes1 = append(txes1, tx)
		} else {
			txes2 = append(txes2, tx)
		}
		require.NoError(t, mp.Add(tx, fs))
	}
	require.Equal(t, mempoolSize, mp.Count())
	mp.RemoveStale(func(t *transaction.Transaction) bool {
		for _, tx := range txes2 {
			if tx == t {
				return true
			}
		}
		return false
	})
	require.Equal(t, mempoolSize/2, mp.Count())
	verTxes := mp.GetVerifiedTransactions()
	for _, tx := range verTxes {
		require.NotContains(t, txes1, tx)
		require.Contains(t, txes2, tx)
	}
}
