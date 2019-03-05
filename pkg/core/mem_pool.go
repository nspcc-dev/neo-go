package core

import (
	"sort"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// PoolItem represents a transaction in the the Memory pool.
type PoolItem struct {
	txn       *transaction.Transaction
	timeStamp time.Time
	fee       Feer
}

// PoolItems slice of PoolItem
type PoolItems []*PoolItem

// MemPool stores the unconfirms transactions.
type MemPool struct {
	unsortedTxn                 map[util.Uint256]*PoolItem
	unverifiedTxn               map[util.Uint256]*PoolItem
	sortedHighPrioTxn           PoolItems
	sortedLowPrioTxn            PoolItems
	unverifiedSortedHighPrioTxn PoolItems
	unverifiedSortedLowPrioTxn  PoolItems

	capacity int
}

func (p PoolItems) Len() int           { return len(p) }
func (p PoolItems) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PoolItems) Less(i, j int) bool { return (p[i].CompareTo(p[j]) < 0) }

// CompareTo returns the difference between two PoolItems.
// difference < 0 implies p < otherP.
// difference = 0 implies p = otherP.
// difference > 0 implies p > otherP.
func (p *PoolItem) CompareTo(otherP *PoolItem) int {
	if otherP == nil {
		return 1
	}

	if p.fee.IsLowPriority(p.txn) && p.fee.IsLowPriority(otherP.txn) {
		thisIsClaimTx := p.txn.Type == transaction.ClaimType
		otherIsClaimTx := otherP.txn.Type == transaction.ClaimType

		if thisIsClaimTx != otherIsClaimTx {
			// This is a claim Tx and other isn't.
			if thisIsClaimTx {
				return 1
			}
			// The other is claim Tx and this isn't.
			return -1
		}
	}

	// Fees sorted ascending
	pFPB := p.fee.FeePerByte(p.txn)
	otherFPB := p.fee.FeePerByte(otherP.txn)
	if ret := pFPB.CompareTo(otherFPB); ret != 0 {
		return ret
	}

	pNF := p.fee.NetworkFee(p.txn)
	otherNF := p.fee.NetworkFee(otherP.txn)
	if ret := pNF.CompareTo(otherNF); ret != 0 {
		return ret
	}

	// Transaction hash sorted descending
	return otherP.txn.Hash().CompareTo(p.txn.Hash())
}

// Count returns the total number of uncofirm transactions.
func (mp MemPool) Count() int {
	return len(mp.unsortedTxn) + len(mp.unverifiedTxn)
}

// ContainsKey checks if a transactions hash is in the MemPool.
func (mp MemPool) ContainsKey(hash util.Uint256) bool {
	if _, ok := mp.unsortedTxn[hash]; ok {
		return true
	}

	if _, ok := mp.unverifiedTxn[hash]; ok {
		return true
	}

	return false
}

// TryAdd try to add the PoolItem to the MemPool.
func (mp MemPool) TryAdd(hash util.Uint256, pItem *PoolItem) bool {
	var pool PoolItems

	if _, ok := mp.unsortedTxn[hash]; ok {
		return false
	}
	mp.unsortedTxn[hash] = pItem

	if pItem.fee.IsLowPriority(pItem.txn) {
		pool = mp.sortedLowPrioTxn
	} else {
		pool = mp.sortedHighPrioTxn
	}
	pool = append(pool, pItem)
	sort.Sort(pool)

	if mp.Count() > mp.capacity {

	}

	_, ok := mp.unsortedTxn[hash]
	return ok
}

// RemoveOverCapacity removes ...
func (mp MemPool) RemoveOverCapacity(pool map[util.Uint256]PoolItem, time time.Time) {
	for mp.Count()-mp.capacity > 0 {
		if minItem, argPosition := getLowestFeeTransaction(mp.sortedLowPrioTxn, mp.unverifiedSortedLowPrioTxn); minItem != nil {
			if argPosition == 1 {
				// minItem belongs to the mp.sortedLowPrioTxn slice.
				// The corresponding unsorted pool is is mp.unsortedTxn.
				delete(mp.unsortedTxn, minItem.txn.Hash())
				mp.sortedLowPrioTxn = append(mp.sortedLowPrioTxn[:0], mp.sortedLowPrioTxn[1:]...)
			} else {
				// minItem belongs to the mp.unverifiedSortedLowPrioTxn slice.
				// The corresponding unsorted pool is is mp.unverifiedTxn.
				delete(mp.unverifiedTxn, minItem.txn.Hash())
				mp.unverifiedSortedLowPrioTxn = append(mp.unverifiedSortedLowPrioTxn[:0], mp.unverifiedSortedLowPrioTxn[1:]...)

			}
		} else if minItem, argPosition := getLowestFeeTransaction(mp.sortedHighPrioTxn, mp.unverifiedSortedHighPrioTxn); minItem != nil {
			if argPosition == 1 {
				// minItem belongs to the mp.sortedHighPrioTxn slice.
				// The corresponding unsorted pool is is mp.unsortedTxn.
				delete(mp.unsortedTxn, minItem.txn.Hash())
				mp.sortedHighPrioTxn = append(mp.sortedHighPrioTxn[:0], mp.sortedHighPrioTxn[1:]...)
			} else {
				// minItem belongs to the mp.unverifiedSortedHighPrioTxn slice.
				// The corresponding unsorted pool is is mp.unverifiedTxn.
				delete(mp.unverifiedTxn, minItem.txn.Hash())
				mp.unverifiedSortedHighPrioTxn = append(mp.unverifiedSortedHighPrioTxn[:0], mp.unverifiedSortedHighPrioTxn[1:]...)

			}
		}
	}

}

// NewPoolItem returns a new PoolItem.
func NewPoolItem(t *transaction.Transaction, fee Feer) *PoolItem {
	return &PoolItem{
		txn:       t,
		timeStamp: time.Now().UTC(),
		fee:       fee,
	}
}

// NewMemPool returns a new MemPool struct.
func NewMemPool(capacity int) MemPool {
	return MemPool{
		unsortedTxn:   make(map[util.Uint256]*PoolItem),
		unverifiedTxn: make(map[util.Uint256]*PoolItem),
		capacity:      capacity,
	}
}

// TryGetValue returns a transactions if it esists in the memory pool.
func (mp MemPool) TryGetValue(hash util.Uint256) (*transaction.Transaction, bool) {
	if pItem, ok := mp.unsortedTxn[hash]; ok {
		return pItem.txn, ok
	}

	if pItem, ok := mp.unverifiedTxn[hash]; ok {
		return pItem.txn, ok
	}

	return nil, false
}

// getLowestFeeTransaction returns the PoolItem with the lowest fee amongst the "verifiedTxnSorted"
// and "unverifiedTxnSorted" PoolItems along with a integer. The integer can assume two values, 1 and 2 which indicate
// that the PoolItem with the lowest fee was found in "verifiedTxnSorted" respectively in "unverifiedTxnSorted".
// "verifiedTxnSorted" and "unverifiedTxnSorted" are sorted slice order by transaction fee ascending. This means that
// the transaction with lowest fee start at index 0.
// Reference: GetLowestFeeTransaction method in C# (https://github.com/neo-project/neo/blob/master/neo/Ledger/MemoryPool.cs)
func getLowestFeeTransaction(verifiedTxnSorted PoolItems, unverifiedTxnSorted PoolItems) (*PoolItem, int) {
	minItem := min(unverifiedTxnSorted)
	verifiedMin := min(verifiedTxnSorted)
	if verifiedMin == nil || (minItem != nil && verifiedMin.CompareTo(minItem) >= 0) {
		return minItem, 2
	}

	minItem = verifiedMin
	return minItem, 1

}

// min return the minimum item in a ascending sorted slice of pool items.
// The  function can't be applied to unsorted slice!
func min(sortedPool PoolItems) *PoolItem {
	var minItem *PoolItem
	if len(sortedPool) > 0 {
		minItem = sortedPool[0]
	} else {
		minItem = nil
	}
	return minItem
}

// GetVerifiedTransactions returns a slice of Input from all the transactions in the memory pool
// whose hash is not included in excludedHashes.
func (mp *MemPool) GetVerifiedTransactions() []*transaction.Transaction {
	var t []*transaction.Transaction
	for _, p := range mp.unsortedTxn {
		t = append(t, p.txn)
	}

	return t
}

// Verify verifies if the inputs of a transaction tx are already used in any other transaction in the memory pool.
// If yes, the transaction tx is not a valid transaction and the function return false.
// If no, the transaction tx is a valid transaction and the function return true.
func (mp MemPool) Verify(tx *transaction.Transaction) bool {
	var mpInputs []transaction.Input

	mpTxn := mp.GetVerifiedTransactions()
	for _, t := range mpTxn {
		if t.Hash() != tx.Hash() {
			for _, in := range t.Inputs {
				mpInputs = append(mpInputs, *in)
			}
		}
	}

	var txInputs []transaction.Input
	for _, in := range tx.Inputs {
		txInputs = append(txInputs, *in)
	}

	if i := transaction.InputIntersection(mpInputs, txInputs); len(i) > 0 {
		return false
	}
	return true
}
