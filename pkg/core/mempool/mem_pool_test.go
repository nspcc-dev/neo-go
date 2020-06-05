package mempool

import (
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FeerStub struct {
	lowPriority bool
	feePerByte  util.Fixed8
}

func (fs *FeerStub) IsLowPriority(util.Fixed8) bool {
	return fs.lowPriority
}

func (fs *FeerStub) FeePerByte() util.Fixed8 {
	return fs.feePerByte
}

func (fs *FeerStub) GetUtilityTokenBalance(uint160 util.Uint160) util.Fixed8 {
	return util.Fixed8FromInt64(10000)
}

func testMemPoolAddRemoveWithFeer(t *testing.T, fs Feer) {
	mp := NewMemPool(10)
	tx := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = 0
	_, _, ok := mp.TryGetValue(tx.Hash())
	require.Equal(t, false, ok)
	require.NoError(t, mp.Add(tx, fs))
	// Re-adding should fail.
	require.Error(t, mp.Add(tx, fs))
	tx2, _, ok := mp.TryGetValue(tx.Hash())
	require.Equal(t, true, ok)
	require.Equal(t, tx, tx2)
	mp.Remove(tx.Hash())
	_, _, ok = mp.TryGetValue(tx.Hash())
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

func TestMemPoolAddRemoveWithInputs(t *testing.T) {
	mp := NewMemPool(50)
	hash1, err := util.Uint256DecodeStringBE("a83ba6ede918a501558d3170a124324aedc89909e64c4ff2c6f863094f980b25")
	require.NoError(t, err)
	hash2, err := util.Uint256DecodeStringBE("629397158f852e838077bb2715b13a2e29b0a51c2157e5466321b70ed7904ce9")
	require.NoError(t, err)
	mpLessInputs := func(i, j int) bool {
		return mp.inputs[i].Cmp(mp.inputs[j]) < 0
	}
	txm1 := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
	txm1.Nonce = 1
	for i := 0; i < 5; i++ {
		txm1.Inputs = append(txm1.Inputs, transaction.Input{PrevHash: hash1, PrevIndex: uint16(100 - i)})
	}
	require.NoError(t, mp.Add(txm1, &FeerStub{}))
	// Look inside.
	assert.Equal(t, len(txm1.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))

	txm2 := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
	txm2.Nonce = 1
	for i := 0; i < 10; i++ {
		txm2.Inputs = append(txm2.Inputs, transaction.Input{PrevHash: hash2, PrevIndex: uint16(i)})
	}
	require.NoError(t, mp.Add(txm2, &FeerStub{}))
	assert.Equal(t, len(txm1.Inputs)+len(txm2.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))

	mp.Remove(txm1.Hash())
	assert.Equal(t, len(txm2.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))

	require.NoError(t, mp.Add(txm1, &FeerStub{}))
	assert.Equal(t, len(txm1.Inputs)+len(txm2.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))

	mp.RemoveStale(func(t *transaction.Transaction) bool {
		if t.Hash() == txm2.Hash() {
			return false
		}
		return true
	}, &FeerStub{})
	assert.Equal(t, len(txm1.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))
}

func TestMemPoolVerifyInputs(t *testing.T) {
	mp := NewMemPool(10)
	tx := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = 1
	inhash1 := random.Uint256()
	tx.Inputs = append(tx.Inputs, transaction.Input{PrevHash: inhash1, PrevIndex: 0})
	require.Equal(t, true, mp.Verify(tx, &FeerStub{}))
	require.NoError(t, mp.Add(tx, &FeerStub{}))

	tx2 := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
	tx2.Nonce = 2
	inhash2 := random.Uint256()
	tx2.Inputs = append(tx2.Inputs, transaction.Input{PrevHash: inhash2, PrevIndex: 0})
	require.Equal(t, true, mp.Verify(tx2, &FeerStub{}))
	require.NoError(t, mp.Add(tx2, &FeerStub{}))

	tx3 := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
	tx3.Nonce = 3
	// Different index number, but the same PrevHash as in tx1.
	tx3.Inputs = append(tx3.Inputs, transaction.Input{PrevHash: inhash1, PrevIndex: 1})
	require.Equal(t, true, mp.Verify(tx3, &FeerStub{}))
	// The same input as in tx2.
	tx3.Inputs = append(tx3.Inputs, transaction.Input{PrevHash: inhash2, PrevIndex: 0})
	require.Equal(t, false, mp.Verify(tx3, &FeerStub{}))
	require.Error(t, mp.Add(tx3, &FeerStub{}))
}

func TestMemPoolVerifyIssue(t *testing.T) {
	mp := NewMemPool(50)
	tx1 := newIssueTX()
	require.Equal(t, true, mp.Verify(tx1, &FeerStub{}))
	require.NoError(t, mp.Add(tx1, &FeerStub{}))

	tx2 := newIssueTX()
	require.Equal(t, false, mp.Verify(tx2, &FeerStub{}))
	require.Error(t, mp.Add(tx2, &FeerStub{}))
}

func newIssueTX() *transaction.Transaction {
	tx := transaction.NewIssueTX()
	tx.Outputs = []transaction.Output{
		{
			AssetID:    random.Uint256(),
			Amount:     util.Fixed8FromInt64(42),
			ScriptHash: random.Uint160(),
		},
	}
	return tx
}

func TestOverCapacity(t *testing.T) {
	var fs = &FeerStub{lowPriority: true}
	const mempoolSize = 10
	mp := NewMemPool(mempoolSize)

	for i := 0; i < mempoolSize; i++ {
		tx := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = uint32(i)
		require.NoError(t, mp.Add(tx, fs))
	}
	txcnt := uint32(mempoolSize)
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// Fees are also prioritized.
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
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
	tx := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
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
	tx = transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = txcnt
	tx.NetworkFee = util.Fixed8FromFloat(0.00007)
	txcnt++
	// size is 51 (no attributes), networkFee is 0.00007 (<0.0001)
	// => feePerByte is 0.00000137 (>0.00000119)
	require.NoError(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// High priority always wins over low priority.
	fs.lowPriority = false
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = txcnt
		txcnt++
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
	}
	// Good luck with low priority now.
	fs.lowPriority = true
	tx = transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
	tx.Nonce = txcnt
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
		tx := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = uint32(i)
		txes = append(txes, tx)
		require.NoError(t, mp.Add(tx, fs))
	}
	require.Equal(t, mempoolSize, mp.Count())
	verTxes := mp.GetVerifiedTransactions()
	require.Equal(t, mempoolSize, len(verTxes))
	for _, txf := range verTxes {
		require.Contains(t, txes, txf.Tx)
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
		tx := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
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
		require.NotContains(t, txes1, txf.Tx)
		require.Contains(t, txes2, txf.Tx)
	}
}

func TestMemPoolFees(t *testing.T) {
	mp := NewMemPool(10)
	sender0 := util.Uint160{1, 2, 3}
	tx0 := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
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
	tx1 := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
	tx1.NetworkFee = util.Fixed8FromInt64(7000)
	tx1.Sender = sender0
	require.NoError(t, mp.Add(tx1, &FeerStub{}))
	require.Equal(t, 1, len(mp.fees))
	require.Equal(t, utilityBalanceAndFees{
		balance: util.Fixed8FromInt64(10000),
		feeSum:  util.Fixed8FromInt64(7000),
	}, mp.fees[sender0])

	// balance shouldn't change after adding one more transaction
	tx2 := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
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
	tx3 := transaction.NewInvocationTX([]byte{byte(opcode.PUSH1)}, 0)
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
