package mempool

import (
	"fmt"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/holiman/uint256"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FeerStub struct {
	feePerByte  int64
	p2pSigExt   bool
	blockHeight uint32
	balance     int64
}

func (fs *FeerStub) GetBaseExecFee() int64 {
	return 30
}

func (fs *FeerStub) FeePerByte() int64 {
	return fs.feePerByte
}

func (fs *FeerStub) BlockHeight() uint32 {
	return fs.blockHeight
}

func (fs *FeerStub) GetUtilityTokenBalance(uint160 util.Uint160) *big.Int {
	return big.NewInt(fs.balance)
}

func testMemPoolAddRemoveWithFeer(t *testing.T, fs Feer) {
	mp := New(10, 0, false, nil)
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
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
	mp.Remove(tx.Hash())
	_, ok = mp.TryGetValue(tx.Hash())
	require.Equal(t, false, ok)
	// Make sure nothing left in the mempool after removal.
	assert.Equal(t, 0, len(mp.verifiedMap))
	assert.Equal(t, 0, len(mp.verifiedTxes))
}

func TestMemPoolRemoveStale(t *testing.T) {
	mp := New(5, 0, false, nil)
	txs := make([]*transaction.Transaction, 5)
	for i := range txs {
		txs[i] = transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		txs[i].Nonce = uint32(i)
		txs[i].Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
		require.NoError(t, mp.Add(txs[i], &FeerStub{blockHeight: uint32(i)}))
	}

	staleTxs := make(chan *transaction.Transaction, 5)
	f := func(tx *transaction.Transaction, _ any) {
		staleTxs <- tx
	}
	mp.SetResendThreshold(5, f)

	isValid := func(tx *transaction.Transaction) bool {
		return tx.Nonce%2 == 0
	}

	mp.RemoveStale(isValid, &FeerStub{blockHeight: 5}) // 0 + 5
	require.Eventually(t, func() bool { return len(staleTxs) == 1 }, time.Second, time.Millisecond*100)
	require.Equal(t, txs[0], <-staleTxs)

	mp.RemoveStale(isValid, &FeerStub{blockHeight: 7}) // 2 + 5
	require.Eventually(t, func() bool { return len(staleTxs) == 1 }, time.Second, time.Millisecond*100)
	require.Equal(t, txs[2], <-staleTxs)

	mp.RemoveStale(isValid, &FeerStub{blockHeight: 10}) // 0 + 2 * 5
	require.Eventually(t, func() bool { return len(staleTxs) == 1 }, time.Second, time.Millisecond*100)
	require.Equal(t, txs[0], <-staleTxs)

	mp.RemoveStale(isValid, &FeerStub{blockHeight: 15}) // 0 + 3 * 5

	// tx[2] should appear, so it is also checked that tx[0] wasn't sent on height 15.
	mp.RemoveStale(isValid, &FeerStub{blockHeight: 22}) // 2 + 4 * 5
	require.Eventually(t, func() bool { return len(staleTxs) == 1 }, time.Second, time.Millisecond*100)
	require.Equal(t, txs[2], <-staleTxs)

	// panic if something is sent after this.
	close(staleTxs)
	require.Len(t, staleTxs, 0)
}

func TestMemPoolAddRemove(t *testing.T) {
	var fs = &FeerStub{}
	testMemPoolAddRemoveWithFeer(t, fs)
}

func TestOverCapacity(t *testing.T) {
	var fs = &FeerStub{balance: 10000000}
	var acc = util.Uint160{1, 2, 3}
	const mempoolSize = 10
	mp := New(mempoolSize, 0, false, nil)

	var checkPoolIsSorted = func() {
		require.True(t, slices.IsSortedFunc(mp.verifiedTxes, func(a, b item) int { return -a.Compare(b) }))
	}

	for i := range mempoolSize {
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = uint32(i)
		tx.Signers = []transaction.Signer{{Account: acc}}
		require.NoError(t, mp.Add(tx, fs))
	}
	txcnt := uint32(mempoolSize)
	require.Equal(t, mempoolSize, mp.Count())
	checkPoolIsSorted()
	require.Equal(t, *uint256.NewInt(0), mp.fees[acc].feeSum)

	bigScript := make([]byte, 64)
	bigScript[0] = byte(opcode.PUSH1)
	bigScript[1] = byte(opcode.RET)
	// Fees are also prioritized.
	for range mempoolSize {
		tx := transaction.New(bigScript, 0)
		tx.NetworkFee = 10000
		tx.Nonce = txcnt
		tx.Signers = []transaction.Signer{{Account: acc}}
		txcnt++
		// size is ~90, networkFee is 10000 => feePerByte is 119
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		checkPoolIsSorted()
	}
	require.Equal(t, *uint256.NewInt(10 * 10000), mp.fees[acc].feeSum)

	// Less prioritized txes are not allowed anymore.
	tx := transaction.New(bigScript, 0)
	tx.NetworkFee = 100
	tx.Nonce = txcnt
	tx.Signers = []transaction.Signer{{Account: acc}}
	txcnt++
	require.Error(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, mempoolSize, len(mp.verifiedMap))
	require.Equal(t, mempoolSize, len(mp.verifiedTxes))
	require.False(t, mp.containsKey(tx.Hash()))
	checkPoolIsSorted()
	require.Equal(t, *uint256.NewInt(100000), mp.fees[acc].feeSum)

	// Low net fee, but higher per-byte fee is still a better combination.
	tx = transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = txcnt
	tx.NetworkFee = 7000
	tx.Signers = []transaction.Signer{{Account: acc}}
	txcnt++
	// size is ~51 (small script), networkFee is 7000 (<10000)
	// => feePerByte is 137 (>119)
	require.NoError(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	checkPoolIsSorted()
	require.Equal(t, *uint256.NewInt(9*10000 + 7000), mp.fees[acc].feeSum)

	// High priority always wins over low priority.
	for range mempoolSize {
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		tx.NetworkFee = 8000
		tx.Nonce = txcnt
		tx.Signers = []transaction.Signer{{Account: acc}}
		txcnt++
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		checkPoolIsSorted()
	}
	require.Equal(t, *uint256.NewInt(10 * 8000), mp.fees[acc].feeSum)
	// Good luck with low priority now.
	tx = transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = txcnt
	tx.NetworkFee = 7000
	tx.Signers = []transaction.Signer{{Account: acc}}
	require.Error(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	checkPoolIsSorted()
}

func TestGetVerified(t *testing.T) {
	var fs = &FeerStub{}
	const mempoolSize = 10
	mp := New(mempoolSize, 0, false, nil)

	txes := make([]*transaction.Transaction, 0, mempoolSize)
	for i := range mempoolSize {
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
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
		mp.Remove(tx.Hash())
	}
	verTxes = mp.GetVerifiedTransactions()
	require.Equal(t, 0, len(verTxes))
}

func TestRemoveStale(t *testing.T) {
	var fs = &FeerStub{}
	const mempoolSize = 10
	mp := New(mempoolSize, 0, false, nil)

	txes1 := make([]*transaction.Transaction, 0, mempoolSize/2)
	txes2 := make([]*transaction.Transaction, 0, mempoolSize/2)
	for i := range mempoolSize {
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
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
		return slices.Contains(txes2, t)
	}, &FeerStub{})
	require.Equal(t, mempoolSize/2, mp.Count())
	verTxes := mp.GetVerifiedTransactions()
	for _, txf := range verTxes {
		require.NotContains(t, txes1, txf)
		require.Contains(t, txes2, txf)
	}
}

func TestMemPoolFees(t *testing.T) {
	mp := New(10, 0, false, nil)
	fs := &FeerStub{balance: 10000000}
	sender0 := util.Uint160{1, 2, 3}
	tx0 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx0.NetworkFee = fs.balance + 1
	tx0.Signers = []transaction.Signer{{Account: sender0}}
	// insufficient funds to add transaction, and balance shouldn't be stored
	require.Equal(t, false, mp.Verify(tx0, fs))
	require.Error(t, mp.Add(tx0, fs))
	require.Equal(t, 0, len(mp.fees))

	balancePart := new(big.Int).Div(big.NewInt(fs.balance), big.NewInt(4))
	// no problems with adding another transaction with lower fee
	tx1 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx1.NetworkFee = balancePart.Int64()
	tx1.Signers = []transaction.Signer{{Account: sender0}}
	require.NoError(t, mp.Add(tx1, fs))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: *uint256.NewInt(uint64(fs.balance)),
		feeSum:  *uint256.NewInt(uint64(tx1.NetworkFee)),
	}, mp.fees[sender0])

	// balance shouldn't change after adding one more transaction
	tx2 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx2.NetworkFee = new(big.Int).Sub(big.NewInt(fs.balance), balancePart).Int64()
	tx2.Signers = []transaction.Signer{{Account: sender0}}
	require.NoError(t, mp.Add(tx2, fs))
	require.Equal(t, 2, len(mp.verifiedTxes))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: *uint256.NewInt(uint64(fs.balance)),
		feeSum:  *uint256.NewInt(uint64(fs.balance)),
	}, mp.fees[sender0])

	// can't add more transactions as we don't have enough GAS
	tx3 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx3.NetworkFee = 1
	tx3.Signers = []transaction.Signer{{Account: sender0}}
	require.Equal(t, false, mp.Verify(tx3, fs))
	require.Error(t, mp.Add(tx3, fs))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: *uint256.NewInt(uint64(fs.balance)),
		feeSum:  *uint256.NewInt(uint64(fs.balance)),
	}, mp.fees[sender0])

	// check whether sender's fee updates correctly
	mp.RemoveStale(func(t *transaction.Transaction) bool {
		return t == tx2
	}, fs)
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: *uint256.NewInt(uint64(fs.balance)),
		feeSum:  *uint256.NewInt(uint64(tx2.NetworkFee)),
	}, mp.fees[sender0])

	// there should be nothing left
	mp.RemoveStale(func(t *transaction.Transaction) bool {
		return t == tx3
	}, fs)
	require.Equal(t, 0, len(mp.fees))
}

func TestMempoolItemsOrder(t *testing.T) {
	sender0 := util.Uint160{1, 2, 3}
	balance := big.NewInt(10000000)

	tx1 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx1.NetworkFee = new(big.Int).Div(balance, big.NewInt(8)).Int64()
	tx1.Signers = []transaction.Signer{{Account: sender0}}
	tx1.Attributes = []transaction.Attribute{{Type: transaction.HighPriority}}
	item1 := item{txn: tx1}

	tx2 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx2.NetworkFee = new(big.Int).Div(balance, big.NewInt(16)).Int64()
	tx2.Signers = []transaction.Signer{{Account: sender0}}
	tx2.Attributes = []transaction.Attribute{{Type: transaction.HighPriority}}
	item2 := item{txn: tx2}

	tx3 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx3.NetworkFee = new(big.Int).Div(balance, big.NewInt(2)).Int64()
	tx3.Signers = []transaction.Signer{{Account: sender0}}
	item3 := item{txn: tx3}

	tx4 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx4.NetworkFee = new(big.Int).Div(balance, big.NewInt(4)).Int64()
	tx4.Signers = []transaction.Signer{{Account: sender0}}
	item4 := item{txn: tx4}

	require.True(t, item1.Compare(item2) > 0)
	require.True(t, item2.Compare(item1) < 0)
	require.True(t, item1.Compare(item3) > 0)
	require.True(t, item3.Compare(item1) < 0)
	require.True(t, item1.Compare(item4) > 0)
	require.True(t, item4.Compare(item1) < 0)
	require.True(t, item2.Compare(item3) > 0)
	require.True(t, item3.Compare(item2) < 0)
	require.True(t, item2.Compare(item4) > 0)
	require.True(t, item4.Compare(item2) < 0)
	require.True(t, item3.Compare(item4) > 0)
	require.True(t, item4.Compare(item3) < 0)
}

func TestMempoolAddRemoveOracleResponse(t *testing.T) {
	mp := New(3, 0, false, nil)
	nonce := uint32(0)
	fs := &FeerStub{balance: 10000}
	newTx := func(netFee int64, id uint64) *transaction.Transaction {
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		tx.NetworkFee = netFee
		tx.Nonce = nonce
		nonce++
		tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3}}}
		tx.Attributes = []transaction.Attribute{{
			Type:  transaction.OracleResponseT,
			Value: &transaction.OracleResponse{ID: id},
		}}
		// sanity check
		_, ok := mp.TryGetValue(tx.Hash())
		require.False(t, ok)
		return tx
	}

	tx1 := newTx(10, 1)
	require.NoError(t, mp.Add(tx1, fs))

	// smaller network fee
	tx2 := newTx(5, 1)
	err := mp.Add(tx2, fs)
	require.ErrorIs(t, err, ErrOracleResponse)

	// ok if old tx is removed
	mp.Remove(tx1.Hash())
	require.NoError(t, mp.Add(tx2, fs))

	// higher network fee
	tx3 := newTx(6, 1)
	require.NoError(t, mp.Add(tx3, fs))
	_, ok := mp.TryGetValue(tx2.Hash())
	require.False(t, ok)
	_, ok = mp.TryGetValue(tx3.Hash())
	require.True(t, ok)

	// another oracle response ID
	tx4 := newTx(4, 2)
	require.NoError(t, mp.Add(tx4, fs))

	mp.RemoveStale(func(tx *transaction.Transaction) bool {
		return tx.Hash() != tx4.Hash()
	}, fs)

	// check that oracle id was removed.
	tx5 := newTx(3, 2)
	require.NoError(t, mp.Add(tx5, fs))

	// another oracle response ID with high net fee
	tx6 := newTx(6, 3)
	require.NoError(t, mp.Add(tx6, fs))
	// check respIds
	for _, i := range []uint64{1, 2, 3} {
		_, ok := mp.oracleResp[i]
		require.True(t, ok)
	}
	// reach capacity, check that response ID is removed together with tx5
	tx7 := newTx(6, 4)
	require.NoError(t, mp.Add(tx7, fs))
	for _, i := range []uint64{1, 4, 3} {
		_, ok := mp.oracleResp[i]
		require.True(t, ok)
	}
}

func TestMempoolAddRemoveConflicts(t *testing.T) {
	var (
		capacity        = 6
		mp              = New(capacity, 0, false, nil)
		sender          = transaction.Signer{Account: util.Uint160{1, 2, 3}}
		maliciousSender = transaction.Signer{Account: util.Uint160{4, 5, 6}}
	)

	var (
		fs           = &FeerStub{p2pSigExt: true, balance: 100000}
		nonce uint32 = 1
	)
	getTx := func(netFee int64, sender transaction.Signer, hashes ...util.Uint256) *transaction.Transaction {
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		tx.NetworkFee = netFee
		tx.Nonce = nonce
		nonce++
		tx.Signers = []transaction.Signer{sender}
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
	getConflictsTx := func(netFee int64, hashes ...util.Uint256) *transaction.Transaction {
		return getTx(netFee, sender, hashes...)
	}
	getMaliciousTx := func(netFee int64, hashes ...util.Uint256) *transaction.Transaction {
		return getTx(netFee, maliciousSender, hashes...)
	}

	// tx1 in mempool and does not conflicts with anyone
	smallNetFee := int64(3)
	tx1 := getConflictsTx(smallNetFee)
	require.NoError(t, mp.Add(tx1, fs))

	// tx2 conflicts with tx1 and has smaller netfee (Step 2, negative)
	tx2 := getConflictsTx(smallNetFee-1, tx1.Hash())
	require.ErrorIs(t, mp.Add(tx2, fs), ErrConflictsAttribute)

	// tx3 conflicts with mempooled tx1 and has larger netfee => tx1 should be replaced by tx3 (Step 2, positive)
	tx3 := getConflictsTx(smallNetFee+1, tx1.Hash())
	require.NoError(t, mp.Add(tx3, fs))
	assert.Equal(t, 1, mp.Count())
	assert.Equal(t, 1, len(mp.conflicts))
	assert.Equal(t, []util.Uint256{tx3.Hash()}, mp.conflicts[tx1.Hash()])

	// tx1 still does not conflicts with anyone, but tx3 is mempooled, conflicts with tx1
	// and has larger netfee => tx1 shouldn't be added again (Step 1, negative)
	require.ErrorIs(t, mp.Add(tx1, fs), ErrConflictsAttribute)

	// tx2 can now safely be added because conflicting tx1 is not in mempool => we
	// cannot check that tx2 is signed by tx1.Sender
	require.NoError(t, mp.Add(tx2, fs))
	assert.Equal(t, 1, len(mp.conflicts))
	assert.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// mempooled tx4 conflicts with tx5, but tx4 has smaller netfee => tx4 should be replaced by tx5 (Step 1, positive)
	tx5 := getConflictsTx(smallNetFee + 1)
	tx4 := getConflictsTx(smallNetFee, tx5.Hash())
	require.NoError(t, mp.Add(tx4, fs)) // unverified
	assert.Equal(t, 2, len(mp.conflicts))
	assert.Equal(t, []util.Uint256{tx4.Hash()}, mp.conflicts[tx5.Hash()])
	require.NoError(t, mp.Add(tx5, fs))
	// tx5 does not conflict with anyone
	assert.Equal(t, 1, len(mp.conflicts))

	// multiple conflicts in attributes of single transaction
	tx6 := getConflictsTx(smallNetFee)
	tx7 := getConflictsTx(smallNetFee)
	tx8 := getConflictsTx(smallNetFee)
	// need small network fee later
	tx9 := getConflictsTx(smallNetFee-2, tx6.Hash(), tx7.Hash(), tx8.Hash())
	require.NoError(t, mp.Add(tx9, fs))
	assert.Equal(t, 4, len(mp.conflicts))
	assert.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx6.Hash()])
	assert.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx7.Hash()])
	assert.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx8.Hash()])
	assert.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// multiple conflicts in attributes of multiple transactions
	tx10 := getConflictsTx(smallNetFee, tx6.Hash())
	tx11 := getConflictsTx(smallNetFee, tx6.Hash())
	require.NoError(t, mp.Add(tx10, fs)) // unverified, because tx6 is not in the pool
	require.NoError(t, mp.Add(tx11, fs)) // unverified, because tx6 is not in the pool
	assert.Equal(t, 4, len(mp.conflicts))
	assert.Equal(t, []util.Uint256{tx9.Hash(), tx10.Hash(), tx11.Hash()}, mp.conflicts[tx6.Hash()])
	assert.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx7.Hash()])
	assert.Equal(t, []util.Uint256{tx9.Hash()}, mp.conflicts[tx8.Hash()])
	assert.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// reach capacity, remove less prioritised tx9 with its multiple conflicts
	require.Equal(t, capacity, len(mp.verifiedTxes))
	tx12 := getConflictsTx(smallNetFee + 2)
	require.NoError(t, mp.Add(tx12, fs))
	assert.Equal(t, 2, len(mp.conflicts))
	assert.Equal(t, []util.Uint256{tx10.Hash(), tx11.Hash()}, mp.conflicts[tx6.Hash()])
	assert.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// manually remove tx11 with its single conflict
	mp.Remove(tx11.Hash())
	assert.Equal(t, 2, len(mp.conflicts))
	assert.Equal(t, []util.Uint256{tx10.Hash()}, mp.conflicts[tx6.Hash()])

	// manually remove last tx which conflicts with tx6 => mp.conflicts[tx6] should also be deleted
	mp.Remove(tx10.Hash())
	assert.Equal(t, 1, len(mp.conflicts))
	assert.Equal(t, []util.Uint256{tx3.Hash(), tx2.Hash()}, mp.conflicts[tx1.Hash()])

	// tx13 conflicts with tx2, but is not signed by tx2.Sender
	tx13 := getMaliciousTx(smallNetFee, tx2.Hash())
	_, ok := mp.TryGetValue(tx13.Hash())
	require.Equal(t, false, ok)
	require.ErrorIs(t, mp.Add(tx13, fs), ErrConflictsAttribute)

	// tx15 conflicts with tx14, but added firstly and has the same network fee => tx14 must not be added.
	tx14 := getConflictsTx(smallNetFee)
	tx15 := getConflictsTx(smallNetFee, tx14.Hash())
	require.NoError(t, mp.Add(tx15, fs))
	err := mp.Add(tx14, fs)
	require.Error(t, err)

	require.True(t, strings.Contains(err.Error(), fmt.Sprintf("conflicting transactions have bigger or equal network fee: %d vs %d", smallNetFee, smallNetFee)))

	check := func(t *testing.T, mainFee int64, fail bool) {
		// Clear mempool.
		mp.RemoveStale(func(t *transaction.Transaction) bool {
			return false
		}, fs)

		// mempooled tx17, tx18, tx19 conflict with tx16
		tx16 := getConflictsTx(mainFee)
		tx17 := getConflictsTx(smallNetFee, tx16.Hash())
		tx18 := getConflictsTx(smallNetFee, tx16.Hash())
		tx19 := getMaliciousTx(smallNetFee, tx16.Hash()) // malicious, thus, doesn't take into account during fee evaluation
		require.NoError(t, mp.Add(tx17, fs))
		require.NoError(t, mp.Add(tx18, fs))
		require.NoError(t, mp.Add(tx19, fs))
		if fail {
			require.Error(t, mp.Add(tx16, fs))
			_, ok = mp.TryGetValue(tx17.Hash())
			require.True(t, ok)
			_, ok = mp.TryGetValue(tx18.Hash())
			require.True(t, ok)
			_, ok = mp.TryGetValue(tx19.Hash())
			require.True(t, ok)
		} else {
			require.NoError(t, mp.Add(tx16, fs))
			_, ok = mp.TryGetValue(tx17.Hash())
			require.False(t, ok)
			_, ok = mp.TryGetValue(tx18.Hash())
			require.False(t, ok)
			_, ok = mp.TryGetValue(tx19.Hash())
			require.False(t, ok)
		}
	}
	check(t, smallNetFee*2, true)
	check(t, smallNetFee*2+1, false)

	check = func(t *testing.T, mainFee int64, fail bool) {
		// Clear mempool.
		mp.RemoveStale(func(t *transaction.Transaction) bool {
			return false
		}, fs)

		// mempooled tx20, tx21, tx22 don't conflict with anyone, but tx23 conflicts with them
		tx20 := getConflictsTx(smallNetFee)
		tx21 := getConflictsTx(smallNetFee)
		tx22 := getConflictsTx(smallNetFee)
		tx23 := getConflictsTx(mainFee, tx20.Hash(), tx21.Hash(), tx22.Hash())
		require.NoError(t, mp.Add(tx20, fs))
		require.NoError(t, mp.Add(tx21, fs))
		require.NoError(t, mp.Add(tx22, fs))
		if fail {
			require.Error(t, mp.Add(tx23, fs))
			_, ok = mp.TryGetData(tx20.Hash())
			require.True(t, ok)
			_, ok = mp.TryGetData(tx21.Hash())
			require.True(t, ok)
			_, ok = mp.TryGetData(tx22.Hash())
			require.True(t, ok)
		} else {
			require.NoError(t, mp.Add(tx23, fs))
			_, ok = mp.TryGetData(tx20.Hash())
			require.False(t, ok)
			_, ok = mp.TryGetData(tx21.Hash())
			require.False(t, ok)
			_, ok = mp.TryGetData(tx22.Hash())
			require.False(t, ok)
		}
	}
	check(t, smallNetFee*3, true)
	check(t, smallNetFee*3+1, false)
}

func TestMempoolAddWithDataGetData(t *testing.T) {
	var (
		smallNetFee int64 = 3
		nonce       uint32
	)
	fs := &FeerStub{
		feePerByte:  0,
		p2pSigExt:   true,
		blockHeight: 5,
		balance:     100,
	}
	mp := New(10, 1, false, nil)

	// bad, insufficient deposit
	r1 := &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(fs.balance+1, &nonce),
	}
	require.ErrorIs(t, mp.Add(r1.FallbackTransaction, fs, r1), ErrInsufficientFunds)

	// good
	r2 := &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee, &nonce),
	}
	require.NoError(t, mp.Add(r2.FallbackTransaction, fs, r2))
	require.True(t, mp.ContainsKey(r2.FallbackTransaction.Hash()))
	data, ok := mp.TryGetData(r2.FallbackTransaction.Hash())
	require.True(t, ok)
	require.Equal(t, r2, data)

	// bad, already in pool
	require.ErrorIs(t, mp.Add(r2.FallbackTransaction, fs, r2), ErrDup)

	// good, higher priority than r2. The resulting mp.verifiedTxes: [r3, r2]
	r3 := &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee+1, &nonce),
	}
	require.NoError(t, mp.Add(r3.FallbackTransaction, fs, r3))
	require.True(t, mp.ContainsKey(r3.FallbackTransaction.Hash()))
	data, ok = mp.TryGetData(r3.FallbackTransaction.Hash())
	require.True(t, ok)
	require.Equal(t, r3, data)

	// good, same priority as r2. The resulting mp.verifiedTxes: [r3, r2, r4]
	r4 := &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee, &nonce),
	}
	require.NoError(t, mp.Add(r4.FallbackTransaction, fs, r4))
	require.True(t, mp.ContainsKey(r4.FallbackTransaction.Hash()))
	data, ok = mp.TryGetData(r4.FallbackTransaction.Hash())
	require.True(t, ok)
	require.Equal(t, r4, data)

	// good, same priority as r2. The resulting mp.verifiedTxes: [r3, r2, r4, r5]
	r5 := &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee, &nonce),
	}
	require.NoError(t, mp.Add(r5.FallbackTransaction, fs, r5))
	require.True(t, mp.ContainsKey(r5.FallbackTransaction.Hash()))
	data, ok = mp.TryGetData(r5.FallbackTransaction.Hash())
	require.True(t, ok)
	require.Equal(t, r5, data)

	// and both r2's and r4's data should still be reachable
	data, ok = mp.TryGetData(r2.FallbackTransaction.Hash())
	require.True(t, ok)
	require.Equal(t, r2, data)
	data, ok = mp.TryGetData(r4.FallbackTransaction.Hash())
	require.True(t, ok)
	require.Equal(t, r4, data)

	// should fail to get unexisting data
	_, ok = mp.TryGetData(util.Uint256{0, 0, 0})
	require.False(t, ok)

	// but getting nil data is OK. The resulting mp.verifiedTxes: [r3, r2, r4, r5, r6]
	r6 := mkTwoSignersTx(smallNetFee, &nonce)
	require.NoError(t, mp.Add(r6, fs, nil))
	require.True(t, mp.ContainsKey(r6.Hash()))
	data, ok = mp.TryGetData(r6.Hash())
	require.True(t, ok)
	require.Nil(t, data)

	// getting data: item is in verifiedMap, but not in verifiedTxes
	r7 := &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee, &nonce),
	}
	require.NoError(t, mp.Add(r7.FallbackTransaction, fs, r4))
	require.True(t, mp.ContainsKey(r7.FallbackTransaction.Hash()))
	r8 := &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee-1, &nonce),
	}
	require.NoError(t, mp.Add(r8.FallbackTransaction, fs, r4))
	require.True(t, mp.ContainsKey(r8.FallbackTransaction.Hash()))
	mp.verifiedTxes = append(mp.verifiedTxes[:len(mp.verifiedTxes)-2], mp.verifiedTxes[len(mp.verifiedTxes)-1])
	_, ok = mp.TryGetData(r7.FallbackTransaction.Hash())
	require.False(t, ok)
}

func mkTwoSignersTx(netFee int64, nonce *uint32) *transaction.Transaction {
	tx := transaction.New([]byte{byte(opcode.RET)}, 0)
	tx.Signers = []transaction.Signer{{}, {}}
	tx.NetworkFee = netFee
	*nonce++
	tx.Nonce = *nonce
	return tx
}

func TestMempoolIterateVerifiedTransactions(t *testing.T) {
	var (
		smallNetFee        int64 = 3
		nonce              uint32
		r1, r2, r3, r4, r5 *payload.P2PNotaryRequest
	)
	fs := &FeerStub{
		feePerByte:  0,
		p2pSigExt:   true,
		blockHeight: 5,
		balance:     100,
	}
	mp := New(10, 1, false, nil)

	checkRequestsOrder := func(orderedRequests []*payload.P2PNotaryRequest) {
		var pooledRequests []*payload.P2PNotaryRequest
		mp.IterateVerifiedTransactions(func(tx *transaction.Transaction, data any) bool {
			d := data.(*payload.P2PNotaryRequest)
			pooledRequests = append(pooledRequests, d)
			return true
		})
		require.Equal(t, orderedRequests, pooledRequests)
	}

	r1 = &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee, &nonce),
	}
	require.NoError(t, mp.Add(r1.FallbackTransaction, fs, r1))
	checkRequestsOrder([]*payload.P2PNotaryRequest{r1})

	// r2 has higher priority than r1. The resulting mp.verifiedTxes: [r2, r1]
	r2 = &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee+1, &nonce),
	}
	require.NoError(t, mp.Add(r2.FallbackTransaction, fs, r2))
	checkRequestsOrder([]*payload.P2PNotaryRequest{r2, r1})

	// r3 has the same priority as r1. The resulting mp.verifiedTxes: [r2, r1, r3]
	r3 = &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee, &nonce),
	}
	require.NoError(t, mp.Add(r3.FallbackTransaction, fs, r3))
	checkRequestsOrder([]*payload.P2PNotaryRequest{r2, r1, r3})

	// r4 has the same priority as r1. The resulting mp.verifiedTxes: [r2, r1, r3, r4]
	r4 = &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee, &nonce),
	}
	require.NoError(t, mp.Add(r4.FallbackTransaction, fs, r4))
	checkRequestsOrder([]*payload.P2PNotaryRequest{r2, r1, r3, r4})

	checkPooledRequest := func(t *testing.T, r *payload.P2PNotaryRequest, isPooled bool) {
		cont := true
		notaryRequest := &payload.P2PNotaryRequest{}
		mp.IterateVerifiedTransactions(func(tx *transaction.Transaction, data any) bool {
			if data != nil {
				notaryRequest = data.(*payload.P2PNotaryRequest)
				if notaryRequest.MainTransaction.Hash() == r.MainTransaction.Hash() {
					cont = false
				}
			}
			return cont
		})

		if isPooled {
			require.Equal(t, false, cont)
			require.Equal(t, r, notaryRequest)
		} else {
			require.Equal(t, true, cont)
		}
	}
	checkPooledRequest(t, r1, true)
	checkPooledRequest(t, r2, true)
	checkPooledRequest(t, r3, true)
	checkPooledRequest(t, r4, true)

	r5 = &payload.P2PNotaryRequest{
		MainTransaction:     mkTwoSignersTx(0, &nonce),
		FallbackTransaction: mkTwoSignersTx(smallNetFee, &nonce),
	}
	checkPooledRequest(t, r5, false)
}
