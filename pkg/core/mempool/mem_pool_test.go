package mempool

import (
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FeerStub struct {
	feePerByte util.Fixed8
}

func (fs *FeerStub) FeePerByte() util.Fixed8 {
	return fs.feePerByte
}

func (fs *FeerStub) GetUtilityTokenBalance(uint160 util.Uint160) util.Fixed8 {
	return util.Fixed8FromInt64(10000)
}

func testMemPoolAddRemoveWithFeer(t *testing.T, fs Feer) {
	mp := NewMemPool(10)
	tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = 0
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
	var fs = &FeerStub{}
	testMemPoolAddRemoveWithFeer(t, fs)
}

func TestOverCapacity(t *testing.T) {
	var fs = &FeerStub{}
	const mempoolSize = 10
	mp := NewMemPool(mempoolSize)

	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = uint32(i)
		require.NoError(t, mp.Add(tx, fs))
	}
	txcnt := uint32(mempoolSize)
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// Fees are also prioritized.
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.Attributes = append(tx.Attributes, transaction.Attribute{
			Usage: transaction.Hash1,
			Data:  util.Uint256{1, 2, 3, 4}.BytesBE(),
		})
		tx.NetworkFee = util.Fixed8FromFloat(0.0001)
		tx.Nonce = txcnt
		txcnt++
		// size is 84, networkFee is 0.0001 => feePerByte is 0.00000119
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
	}
	// Less prioritized txes are not allowed anymore.
	tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.Attributes = append(tx.Attributes, transaction.Attribute{
		Usage: transaction.Hash1,
		Data:  util.Uint256{1, 2, 3, 4}.BytesBE(),
	})
	tx.NetworkFee = util.Fixed8FromFloat(0.000001)
	tx.Nonce = txcnt
	txcnt++
	require.Error(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// Low net fee, but higher per-byte fee is still a better combination.
	tx = transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = txcnt
	tx.NetworkFee = util.Fixed8FromFloat(0.00007)
	txcnt++
	// size is 51 (no attributes), networkFee is 0.00007 (<0.0001)
	// => feePerByte is 0.00000137 (>0.00000119)
	require.NoError(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// High priority always wins over low priority.
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.NetworkFee = util.Fixed8FromFloat(0.00008)
		tx.Nonce = txcnt
		txcnt++
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
	}
	// Good luck with low priority now.
	tx = transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = txcnt
	tx.NetworkFee = util.Fixed8FromFloat(0.00007)
	require.Error(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
}

func TestGetVerified(t *testing.T) {
	var fs = &FeerStub{}
	const mempoolSize = 10
	mp := NewMemPool(mempoolSize)

	txes := make([]*transaction.Transaction, 0, mempoolSize)
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = uint32(i)
		txes = append(txes, tx)
		require.NoError(t, mp.Add(tx, fs))
	}
	require.Equal(t, mempoolSize, mp.Count())
	verTxes := mp.GetVerifiedTransactions()
	require.Equal(t, mempoolSize, len(verTxes))
	require.ElementsMatch(t, txes, verTxes)
	for _, tx := range txes {
		mp.Remove(tx.Hash())
	}
	verTxes = mp.GetVerifiedTransactions()
	require.Equal(t, 0, len(verTxes))
}

func TestRemoveStale(t *testing.T) {
	var fs = &FeerStub{}
	const mempoolSize = 10
	mp := NewMemPool(mempoolSize)

	txes1 := make([]*transaction.Transaction, 0, mempoolSize/2)
	txes2 := make([]*transaction.Transaction, 0, mempoolSize/2)
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = uint32(i)
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
	}, &FeerStub{})
	require.Equal(t, mempoolSize/2, mp.Count())
	verTxes := mp.GetVerifiedTransactions()
	for _, txf := range verTxes {
		require.NotContains(t, txes1, txf)
		require.Contains(t, txes2, txf)
	}
}

func TestMemPoolFees(t *testing.T) {
	mp := NewMemPool(10)
	sender0 := util.Uint160{1, 2, 3}
	tx0 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx0.NetworkFee = util.Fixed8FromInt64(11000)
	tx0.Sender = sender0
	// insufficient funds to add transaction, but balance should be stored
	require.Equal(t, false, mp.Verify(tx0, &FeerStub{}))
	require.Error(t, mp.Add(tx0, &FeerStub{}))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: util.Fixed8FromInt64(10000),
		feeSum:  0,
	}, mp.fees[sender0])

	// no problems with adding another transaction with lower fee
	tx1 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx1.NetworkFee = util.Fixed8FromInt64(7000)
	tx1.Sender = sender0
	require.NoError(t, mp.Add(tx1, &FeerStub{}))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: util.Fixed8FromInt64(10000),
		feeSum:  util.Fixed8FromInt64(7000),
	}, mp.fees[sender0])

	// balance shouldn't change after adding one more transaction
	tx2 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx2.NetworkFee = util.Fixed8FromFloat(3000)
	tx2.Sender = sender0
	require.NoError(t, mp.Add(tx2, &FeerStub{}))
	require.Equal(t, 2, len(mp.verifiedTxes))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: util.Fixed8FromInt64(10000),
		feeSum:  util.Fixed8FromInt64(10000),
	}, mp.fees[sender0])

	// can't add more transactions as we don't have enough GAS
	tx3 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx3.NetworkFee = util.Fixed8FromFloat(0.5)
	tx3.Sender = sender0
	require.Equal(t, false, mp.Verify(tx3, &FeerStub{}))
	require.Error(t, mp.Add(tx3, &FeerStub{}))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: util.Fixed8FromInt64(10000),
		feeSum:  util.Fixed8FromInt64(10000),
	}, mp.fees[sender0])

	// check whether sender's fee updates correctly
	mp.RemoveStale(func(t *transaction.Transaction) bool {
		if t == tx2 {
			return true
		}
		return false
	}, &FeerStub{})
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: util.Fixed8FromInt64(10000),
		feeSum:  util.Fixed8FromFloat(3000),
	}, mp.fees[sender0])

	// there should be nothing left
	mp.RemoveStale(func(t *transaction.Transaction) bool {
		if t == tx3 {
			return true
		}
		return false
	}, &FeerStub{})
	require.Equal(t, 0, len(mp.fees))
}
