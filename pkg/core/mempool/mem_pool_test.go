package mempool

import (
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/util"
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
	tx := transaction.NewContractTX()
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

func TestMemPoolAddRemoveWithInputsAndClaims(t *testing.T) {
	mp := NewMemPool(50)
	hash1, err := util.Uint256DecodeStringBE("a83ba6ede918a501558d3170a124324aedc89909e64c4ff2c6f863094f980b25")
	require.NoError(t, err)
	hash2, err := util.Uint256DecodeStringBE("629397158f852e838077bb2715b13a2e29b0a51c2157e5466321b70ed7904ce9")
	require.NoError(t, err)
	mpLessInputs := func(i, j int) bool {
		return mp.inputs[i].Cmp(mp.inputs[j]) < 0
	}
	mpLessClaims := func(i, j int) bool {
		return mp.claims[i].Cmp(mp.claims[j]) < 0
	}
	txm1 := transaction.NewContractTX()
	txm1.Nonce = 1
	txc1, claim1 := newClaimTX()
	for i := 0; i < 5; i++ {
		txm1.Inputs = append(txm1.Inputs, transaction.Input{PrevHash: hash1, PrevIndex: uint16(100 - i)})
		claim1.Claims = append(claim1.Claims, transaction.Input{PrevHash: hash1, PrevIndex: uint16(100 - i)})
	}
	require.NoError(t, mp.Add(txm1, &FeerStub{}))
	require.NoError(t, mp.Add(txc1, &FeerStub{}))
	// Look inside.
	assert.Equal(t, len(txm1.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))
	assert.Equal(t, len(claim1.Claims), len(mp.claims))
	assert.True(t, sort.SliceIsSorted(mp.claims, mpLessClaims))

	txm2 := transaction.NewContractTX()
	txm2.Nonce = 1
	txc2, claim2 := newClaimTX()
	for i := 0; i < 10; i++ {
		txm2.Inputs = append(txm2.Inputs, transaction.Input{PrevHash: hash2, PrevIndex: uint16(i)})
		claim2.Claims = append(claim2.Claims, transaction.Input{PrevHash: hash2, PrevIndex: uint16(i)})
	}
	require.NoError(t, mp.Add(txm2, &FeerStub{}))
	require.NoError(t, mp.Add(txc2, &FeerStub{}))
	assert.Equal(t, len(txm1.Inputs)+len(txm2.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))
	assert.Equal(t, len(claim1.Claims)+len(claim2.Claims), len(mp.claims))
	assert.True(t, sort.SliceIsSorted(mp.claims, mpLessClaims))

	mp.Remove(txm1.Hash())
	mp.Remove(txc2.Hash())
	assert.Equal(t, len(txm2.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))
	assert.Equal(t, len(claim1.Claims), len(mp.claims))
	assert.True(t, sort.SliceIsSorted(mp.claims, mpLessClaims))

	require.NoError(t, mp.Add(txm1, &FeerStub{}))
	require.NoError(t, mp.Add(txc2, &FeerStub{}))
	assert.Equal(t, len(txm1.Inputs)+len(txm2.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))
	assert.Equal(t, len(claim1.Claims)+len(claim2.Claims), len(mp.claims))
	assert.True(t, sort.SliceIsSorted(mp.claims, mpLessClaims))

	mp.RemoveStale(func(t *transaction.Transaction) bool {
		if t.Hash() == txc1.Hash() || t.Hash() == txm2.Hash() {
			return false
		}
		return true
	})
	assert.Equal(t, len(txm1.Inputs), len(mp.inputs))
	assert.True(t, sort.SliceIsSorted(mp.inputs, mpLessInputs))
	assert.Equal(t, len(claim2.Claims), len(mp.claims))
	assert.True(t, sort.SliceIsSorted(mp.claims, mpLessClaims))
}

func TestMemPoolVerifyInputs(t *testing.T) {
	mp := NewMemPool(10)
	tx := transaction.NewContractTX()
	tx.Nonce = 1
	inhash1 := random.Uint256()
	tx.Inputs = append(tx.Inputs, transaction.Input{PrevHash: inhash1, PrevIndex: 0})
	require.Equal(t, true, mp.Verify(tx))
	require.NoError(t, mp.Add(tx, &FeerStub{}))

	tx2 := transaction.NewContractTX()
	tx2.Nonce = 2
	inhash2 := random.Uint256()
	tx2.Inputs = append(tx2.Inputs, transaction.Input{PrevHash: inhash2, PrevIndex: 0})
	require.Equal(t, true, mp.Verify(tx2))
	require.NoError(t, mp.Add(tx2, &FeerStub{}))

	tx3 := transaction.NewContractTX()
	tx3.Nonce = 3
	// Different index number, but the same PrevHash as in tx1.
	tx3.Inputs = append(tx3.Inputs, transaction.Input{PrevHash: inhash1, PrevIndex: 1})
	require.Equal(t, true, mp.Verify(tx3))
	// The same input as in tx2.
	tx3.Inputs = append(tx3.Inputs, transaction.Input{PrevHash: inhash2, PrevIndex: 0})
	require.Equal(t, false, mp.Verify(tx3))
	require.Error(t, mp.Add(tx3, &FeerStub{}))
}

func TestMemPoolVerifyClaims(t *testing.T) {
	mp := NewMemPool(50)
	tx1, claim1 := newClaimTX()
	hash1, err := util.Uint256DecodeStringBE("a83ba6ede918a501558d3170a124324aedc89909e64c4ff2c6f863094f980b25")
	require.NoError(t, err)
	hash2, err := util.Uint256DecodeStringBE("629397158f852e838077bb2715b13a2e29b0a51c2157e5466321b70ed7904ce9")
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		claim1.Claims = append(claim1.Claims, transaction.Input{PrevHash: hash1, PrevIndex: uint16(i)})
		claim1.Claims = append(claim1.Claims, transaction.Input{PrevHash: hash2, PrevIndex: uint16(i)})
	}
	require.Equal(t, true, mp.Verify(tx1))
	require.NoError(t, mp.Add(tx1, &FeerStub{}))

	tx2, claim2 := newClaimTX()
	for i := 0; i < 10; i++ {
		claim2.Claims = append(claim2.Claims, transaction.Input{PrevHash: hash2, PrevIndex: uint16(i + 10)})
	}
	require.Equal(t, true, mp.Verify(tx2))
	require.NoError(t, mp.Add(tx2, &FeerStub{}))

	tx3, claim3 := newClaimTX()
	claim3.Claims = append(claim3.Claims, transaction.Input{PrevHash: hash1, PrevIndex: 0})
	require.Equal(t, false, mp.Verify(tx3))
	require.Error(t, mp.Add(tx3, &FeerStub{}))
}

func TestMemPoolVerifyIssue(t *testing.T) {
	mp := NewMemPool(50)
	tx1 := newIssueTX()
	require.Equal(t, true, mp.Verify(tx1))
	require.NoError(t, mp.Add(tx1, &FeerStub{}))

	tx2 := newIssueTX()
	require.Equal(t, false, mp.Verify(tx2))
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

func newClaimTX() (*transaction.Transaction, *transaction.ClaimTX) {
	cl := &transaction.ClaimTX{}
	return transaction.NewClaimTX(cl), cl
}

func TestOverCapacity(t *testing.T) {
	var fs = &FeerStub{lowPriority: true}
	const mempoolSize = 10
	mp := NewMemPool(mempoolSize)

	for i := 0; i < mempoolSize; i++ {
		tx := transaction.NewContractTX()
		tx.Nonce = uint32(i)
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
		tx := transaction.NewContractTX()
		tx.Nonce = txcnt
		txcnt++
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
	}
	// Less prioritized txes are not allowed anymore.
	fs.netFee = util.Fixed8FromFloat(0.00001)
	tx := transaction.NewContractTX()
	tx.Nonce = txcnt
	txcnt++
	require.Error(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// But claim tx should still be there.
	require.True(t, mp.ContainsKey(claim.Hash()))

	// Low net fee, but higher per-byte fee is still a better combination.
	fs.perByteFee = util.Fixed8FromFloat(0.001)
	tx = transaction.NewContractTX()
	tx.Nonce = txcnt
	txcnt++
	require.NoError(t, mp.Add(tx, fs))
	require.Equal(t, mempoolSize, mp.Count())
	require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))

	// High priority always wins over low priority.
	fs.lowPriority = false
	for i := 0; i < mempoolSize; i++ {
		tx := transaction.NewContractTX()
		tx.Nonce = txcnt
		txcnt++
		require.NoError(t, mp.Add(tx, fs))
		require.Equal(t, mempoolSize, mp.Count())
		require.Equal(t, true, sort.IsSorted(sort.Reverse(mp.verifiedTxes)))
	}
	// Good luck with low priority now.
	fs.lowPriority = true
	tx = transaction.NewContractTX()
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
		tx := transaction.NewContractTX()
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
		tx := transaction.NewContractTX()
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
	})
	require.Equal(t, mempoolSize/2, mp.Count())
	verTxes := mp.GetVerifiedTransactions()
	for _, txf := range verTxes {
		require.NotContains(t, txes1, txf.Tx)
		require.Contains(t, txes2, txf.Tx)
	}
}
