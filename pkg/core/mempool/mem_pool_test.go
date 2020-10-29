package mempool

import (
	"errors"
	"math/big"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FeerStub struct {
	feePerByte int64
	p2pSigExt  bool
}

var balance = big.NewInt(10000000)

func (fs *FeerStub) FeePerByte() int64 {
	return fs.feePerByte
}

func (fs *FeerStub) BlockHeight() uint32 {
	return 0
}

func (fs *FeerStub) GetUtilityTokenBalance(uint160 util.Uint160) *big.Int {
	return balance
}

func (fs *FeerStub) P2PSigExtensionsEnabled() bool {
	return fs.p2pSigExt
}

func testMemPoolAddRemoveWithFeer(t *testing.T, fs Feer) {
	mp := New(10)
	tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = 0
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	_, ok := mp.TryGetValue(tx.Hash())
	require.Equal(t, false, ok)
	require.NoError(t, mp.Add(tx, fs))
	// Re-adding should fail.
	require.Error(t, mp.Add(tx, fs))
	tx2, ok := mp.TryGetValue(tx.Hash())
	require.Equal(t, true, ok)
	require.Equal(t, tx, tx2)
	mp.Remove(tx.Hash(), fs)
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
	mp := New(mempoolSize)

	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = uint32(i)
		tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
		require.NoError(t, mp.Add(tx, fs))
	}
	txcnt := uint32(mempoolSize)
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	bigScript := make([]byte, 64)
	bigScript[0] = byte(opcode.PUSH1)
	bigScript[1] = byte(opcode.RET)
	// Fees are also prioritized.
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, bigScript, 0)
		tx.NetworkFee = 10000
		tx.Nonce = txcnt
		tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
		txcnt++
		// size is ~90, networkFee is 10000 => feePerByte is 119
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
	}
	// Less prioritized txes are not allowed anymore.
	tx := transaction.New(netmode.UnitTestNet, bigScript, 0)
	tx.NetworkFee = 100
	tx.Nonce = txcnt
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	txcnt++
	require.Error(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// Low net fee, but higher per-byte fee is still a better combination.
	tx = transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = txcnt
	tx.NetworkFee = 7000
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	txcnt++
	// size is ~51 (small script), networkFee is 7000 (<10000)
	// => feePerByte is 137 (>119)
	require.NoError(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// High priority always wins over low priority.
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.NetworkFee = 8000
		tx.Nonce = txcnt
		tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
		txcnt++
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
	}
	// Good luck with low priority now.
	tx = transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = txcnt
	tx.NetworkFee = 7000
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
	require.Error(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
}

func TestGetVerified(t *testing.T) {
	var fs = &FeerStub{}
	const mempoolSize = 10
	mp := New(mempoolSize)

	txes := make([]*transaction.Transaction, 0, mempoolSize)
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = uint32(i)
		tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
		txes = append(txes, tx)
		require.NoError(t, mp.Add(tx, fs))
	}
	require.Equal(t, mempoolSize, mp.Count())
	verTxes := mp.GetVerifiedTransactions()
	require.Equal(t, mempoolSize, len(verTxes))
	require.ElementsMatch(t, txes, verTxes)
	for _, tx := range txes {
		mp.Remove(tx.Hash(), fs)
	}
	verTxes = mp.GetVerifiedTransactions()
	require.Equal(t, 0, len(verTxes))
}

func TestRemoveStale(t *testing.T) {
	var fs = &FeerStub{}
	const mempoolSize = 10
	mp := New(mempoolSize)

	txes1 := make([]*transaction.Transaction, 0, mempoolSize/2)
	txes2 := make([]*transaction.Transaction, 0, mempoolSize/2)
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = uint32(i)
		tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
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
	mp := New(10)
	sender0 := util.Uint160{1, 2, 3}
	tx0 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx0.NetworkFee = balance.Int64() + 1
	tx0.Signers = []transaction.Signer{{Account: sender0}}
	// insufficient funds to add transaction, and balance shouldn't be stored
	require.Equal(t, false, mp.Verify(tx0, &FeerStub{}))
	require.Error(t, mp.Add(tx0, &FeerStub{}))
	require.Equal(t, 0, len(mp.fees))

	balancePart := new(big.Int).Div(balance, big.NewInt(4))
	// no problems with adding another transaction with lower fee
	tx1 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx1.NetworkFee = balancePart.Int64()
	tx1.Signers = []transaction.Signer{{Account: sender0}}
	require.NoError(t, mp.Add(tx1, &FeerStub{}))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: balance,
		feeSum:  big.NewInt(tx1.NetworkFee),
	}, mp.fees[sender0])

	// balance shouldn't change after adding one more transaction
	tx2 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx2.NetworkFee = new(big.Int).Sub(balance, balancePart).Int64()
	tx2.Signers = []transaction.Signer{{Account: sender0}}
	require.NoError(t, mp.Add(tx2, &FeerStub{}))
	require.Equal(t, 2, len(mp.verifiedTxes))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: balance,
		feeSum:  balance,
	}, mp.fees[sender0])

	// can't add more transactions as we don't have enough GAS
	tx3 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx3.NetworkFee = 1
	tx3.Signers = []transaction.Signer{{Account: sender0}}
	require.Equal(t, false, mp.Verify(tx3, &FeerStub{}))
	require.Error(t, mp.Add(tx3, &FeerStub{}))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: balance,
		feeSum:  balance,
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
		balance: balance,
		feeSum:  big.NewInt(tx2.NetworkFee),
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

func TestMempoolItemsOrder(t *testing.T) {
	sender0 := util.Uint160{1, 2, 3}

	tx1 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx1.NetworkFee = new(big.Int).Div(balance, big.NewInt(8)).Int64()
	tx1.Signers = []transaction.Signer{{Account: sender0}}
	tx1.Attributes = []transaction.Attribute{{Type: transaction.HighPriority}}
	item1 := item{txn: tx1}

	tx2 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx2.NetworkFee = new(big.Int).Div(balance, big.NewInt(16)).Int64()
	tx2.Signers = []transaction.Signer{{Account: sender0}}
	tx2.Attributes = []transaction.Attribute{{Type: transaction.HighPriority}}
	item2 := item{txn: tx2}

	tx3 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx3.NetworkFee = new(big.Int).Div(balance, big.NewInt(2)).Int64()
	tx3.Signers = []transaction.Signer{{Account: sender0}}
	item3 := item{txn: tx3}

	tx4 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx4.NetworkFee = new(big.Int).Div(balance, big.NewInt(4)).Int64()
	tx4.Signers = []transaction.Signer{{Account: sender0}}
	item4 := item{txn: tx4}

	require.True(t, item1.CompareTo(item2) > 0)
	require.True(t, item2.CompareTo(item1) < 0)
	require.True(t, item1.CompareTo(item3) > 0)
	require.True(t, item3.CompareTo(item1) < 0)
	require.True(t, item1.CompareTo(item4) > 0)
	require.True(t, item4.CompareTo(item1) < 0)
	require.True(t, item2.CompareTo(item3) > 0)
	require.True(t, item3.CompareTo(item2) < 0)
	require.True(t, item2.CompareTo(item4) > 0)
	require.True(t, item4.CompareTo(item2) < 0)
	require.True(t, item3.CompareTo(item4) > 0)
	require.True(t, item4.CompareTo(item3) < 0)
}

func TestMempoolAddRemoveConflicts(t *testing.T) {
	capacity := 6
	mp := New(capacity)
	var fs = &FeerStub{
		p2pSigExt: true,
	}
	getConflictsTx := func(netFee int64, hashes ...util.Uint256) *transaction.Transaction {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.NetworkFee = netFee
		tx.Nonce = uint32(random.Int(0, 1e4))
		tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
		tx.Attributes = make([]transaction.Attribute, len(hashes))
		for i, h := range hashes {
			tx.Attributes[i] = transaction.Attribute{
				Type: transaction.ConflictsT,
				Value: &transaction.Conflicts{
					Hash: h,
				},
			}
		}
		_, ok := mp.TryGetValue(tx.Hash())
		require.Equal(t, false, ok)
		return tx
	}

	// tx1 in mempool and does not conflicts with anyone
	smallNetFee := int64(3)
	tx1 := getConflictsTx(smallNetFee)
	require.NoError(t, mp.Add(tx1, fs))

	// tx2 conflicts with tx1 and has smaller netfee (Step 2, negative)
	tx2 := getConflictsTx(smallNetFee-1, tx1.Hash())
	require.True(t, errors.Is(mp.Add(tx2, fs), ErrConflictsAttribute))

	// tx3 conflicts with mempooled tx1 and has larger netfee => tx1 should be replaced by tx3 (Step 2, positive)
	tx3 := getConflictsTx(smallNetFee+1, tx1.Hash())
	require.NoError(t, mp.Add(tx3, fs))
	require.Equal(t, 1, mp.Count())
	require.Equal(t, 1, len(mp.conflicts))
	require.Equal(t, []util.Uint256{tx3.Hash()}, mp.conflicts[tx1.Hash()])

	// tx1 still does not conflicts with anyone, but tx3 is mempooled, conflicts with tx1
	// and has larger netfee => tx1 shouldn't be added again (Step 1, negative)
	require.True(t, errors.Is(mp.Add(tx1, fs), ErrConflictsAttribute))

	// tx2 can now safely be added because conflicting tx1 is not in mempool => we
	// cannot check that tx2 is signed by tx1.Sender
	require.NoError(t, mp.Add(tx2, fs))
	require.Equal(t, 1, len(mp.conflicts))
	require.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// mempooled tx4 conflicts with tx5, but tx4 has smaller netfee => tx4 should be replaced by tx5 (Step 1, positive)
	tx5 := getConflictsTx(smallNetFee + 1)
	tx4 := getConflictsTx(smallNetFee, tx5.Hash())
	require.NoError(t, mp.Add(tx4, fs)) // unverified
	require.Equal(t, 2, len(mp.conflicts))
	require.Equal(t, []util.Uint256{tx4.Hash()}, mp.conflicts[tx5.Hash()])
	require.NoError(t, mp.Add(tx5, fs))
	// tx5 does not conflict with anyone
	require.Equal(t, 1, len(mp.conflicts))

	// multiple conflicts in attributes of single transaction
	tx6 := getConflictsTx(smallNetFee)
	tx7 := getConflictsTx(smallNetFee)
	tx8 := getConflictsTx(smallNetFee)
	// need small network fee later
	tx9 := getConflictsTx(smallNetFee-2, tx6.Hash(), tx7.Hash(), tx8.Hash())
	require.NoError(t, mp.Add(tx9, fs))
	require.Equal(t, 4, len(mp.conflicts))
	require.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx6.Hash()])
	require.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx7.Hash()])
	require.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx8.Hash()])
	require.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// multiple conflicts in attributes of multiple transactions
	tx10 := getConflictsTx(smallNetFee, tx6.Hash())
	tx11 := getConflictsTx(smallNetFee, tx6.Hash())
	require.NoError(t, mp.Add(tx10, fs)) // unverified, because tx6 is not in the pool
	require.NoError(t, mp.Add(tx11, fs)) // unverified, because tx6 is not in the pool
	require.Equal(t, 4, len(mp.conflicts))
	require.Equal(t, []util.Uint256{tx9.Hash(), tx10.Hash(), tx11.Hash()}, mp.conflicts[tx6.Hash()])
	require.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx7.Hash()])
	require.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx8.Hash()])
	require.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// reach capacity, remove less prioritised tx9 with its multiple conflicts
	require.Equal(t, capacity, len(mp.verifiedTxes))
	tx12 := getConflictsTx(smallNetFee + 2)
	require.NoError(t, mp.Add(tx12, fs))
	require.Equal(t, 2, len(mp.conflicts))
	require.Equal(t, []util.Uint256{tx10.Hash(), tx11.Hash()}, mp.conflicts[tx6.Hash()])
	require.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// manually remove tx11 with its single conflict
	mp.Remove(tx11.Hash(), fs)
	require.Equal(t, 2, len(mp.conflicts))
	require.Equal(t, []util.Uint256{tx10.Hash()}, mp.conflicts[tx6.Hash()])

	// manually remove last tx which conflicts with tx6 => mp.conflicts[tx6] should also be deleted
	mp.Remove(tx10.Hash(), fs)
	require.Equal(t, 1, len(mp.conflicts))
	require.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// tx13 conflicts with tx2, but is not signed by tx2.Sender
	tx13 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx13.NetworkFee = smallNetFee
	tx13.Nonce = uint32(random.Int(0, 1e4))
	tx13.Signers = []transaction.Signer{{Account: util.Uint160{3, 2, 1}}}
	tx13.Attributes = []transaction.Attribute{{
		Type: transaction.ConflictsT,
		Value: &transaction.Conflicts{
			Hash: tx2.Hash(),
		},
	}}
	_, ok := mp.TryGetValue(tx13.Hash())
	require.Equal(t, false, ok)
	require.True(t, errors.Is(mp.Add(tx13, fs), ErrConflictsAttribute))
}
