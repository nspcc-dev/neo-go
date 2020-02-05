package mempool

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

var (
	// ErrConflict is returned when transaction being added is incompatible
	// with the contents of the memory pool (using the same inputs as some
	// other transaction in the pool)
	ErrConflict = errors.New("conflicts with the memory pool")
	// ErrDup is returned when transaction being added is already present
	// in the memory pool.
	ErrDup = errors.New("already in the memory pool")
	// ErrOOM is returned when transaction just doesn't fit in the memory
	// pool because of its capacity constraints.
	ErrOOM = errors.New("out of memory")
)

// item represents a transaction in the the Memory pool.
type item struct {
	txn        *transaction.Transaction
	timeStamp  time.Time
	perByteFee util.Fixed8
	netFee     util.Fixed8
	isLowPrio  bool
}

// items is a slice of item.
type items []*item

// Pool stores the unconfirms transactions.
type Pool struct {
	lock                        sync.RWMutex
	unsortedTxn                 map[util.Uint256]*item
	unverifiedTxn               map[util.Uint256]*item
	sortedHighPrioTxn           items
	sortedLowPrioTxn            items
	unverifiedSortedHighPrioTxn items
	unverifiedSortedLowPrioTxn  items

	capacity int
}

func (p items) Len() int           { return len(p) }
func (p items) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p items) Less(i, j int) bool { return p[i].CompareTo(p[j]) < 0 }

// CompareTo returns the difference between two items.
// difference < 0 implies p < otherP.
// difference = 0 implies p = otherP.
// difference > 0 implies p > otherP.
func (p item) CompareTo(otherP *item) int {
	if otherP == nil {
		return 1
	}

	if p.isLowPrio && otherP.isLowPrio {
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

	// Fees sorted ascending.
	if ret := p.perByteFee.CompareTo(otherP.perByteFee); ret != 0 {
		return ret
	}

	if ret := p.netFee.CompareTo(otherP.netFee); ret != 0 {
		return ret
	}

	// Transaction hash sorted descending.
	return otherP.txn.Hash().CompareTo(p.txn.Hash())
}

// Count returns the total number of uncofirm transactions.
func (mp *Pool) Count() int {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	return len(mp.unsortedTxn) + len(mp.unverifiedTxn)
}

// ContainsKey checks if a transactions hash is in the Pool.
func (mp *Pool) ContainsKey(hash util.Uint256) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	if _, ok := mp.unsortedTxn[hash]; ok {
		return true
	}

	if _, ok := mp.unverifiedTxn[hash]; ok {
		return true
	}

	return false
}

// Add tries to add given transaction to the Pool.
func (mp *Pool) Add(t *transaction.Transaction, fee Feer) error {
	var pool *items
	var pItem = &item{
		txn:        t,
		timeStamp:  time.Now().UTC(),
		perByteFee: fee.FeePerByte(t),
		netFee:     fee.NetworkFee(t),
		isLowPrio:  fee.IsLowPriority(t),
	}

	if pItem.isLowPrio {
		pool = &mp.sortedLowPrioTxn
	} else {
		pool = &mp.sortedHighPrioTxn
	}

	mp.lock.Lock()
	if !mp.verifyInputs(pItem.txn) {
		mp.lock.Unlock()
		return ErrConflict
	}
	if _, ok := mp.unsortedTxn[t.Hash()]; ok {
		mp.lock.Unlock()
		return ErrDup
	}
	mp.unsortedTxn[t.Hash()] = pItem

	*pool = append(*pool, pItem)
	sort.Sort(pool)
	mp.lock.Unlock()

	if mp.Count() > mp.capacity {
		mp.RemoveOverCapacity()
	}
	mp.lock.RLock()
	_, ok := mp.unsortedTxn[t.Hash()]
	updateMempoolMetrics(len(mp.unsortedTxn), len(mp.unverifiedTxn))
	mp.lock.RUnlock()
	if !ok {
		return ErrOOM
	}
	return nil
}

// Remove removes an item from the mempool, if it exists there (and does
// nothing if it doesn't).
func (mp *Pool) Remove(hash util.Uint256) {
	var mapAndPools = []struct {
		unsortedMap map[util.Uint256]*item
		sortedPools []*items
	}{
		{unsortedMap: mp.unsortedTxn, sortedPools: []*items{&mp.sortedHighPrioTxn, &mp.sortedLowPrioTxn}},
		{unsortedMap: mp.unverifiedTxn, sortedPools: []*items{&mp.unverifiedSortedHighPrioTxn, &mp.unverifiedSortedLowPrioTxn}},
	}
	mp.lock.Lock()
	for _, mapAndPool := range mapAndPools {
		if _, ok := mapAndPool.unsortedMap[hash]; ok {
			delete(mapAndPool.unsortedMap, hash)
			for _, pool := range mapAndPool.sortedPools {
				var num int
				var item *item
				for num, item = range *pool {
					if hash.Equals(item.txn.Hash()) {
						break
					}
				}
				if num < len(*pool)-1 {
					*pool = append((*pool)[:num], (*pool)[num+1:]...)
				} else if num == len(*pool)-1 {
					*pool = (*pool)[:num]
				}
			}
		}
	}
	updateMempoolMetrics(len(mp.unsortedTxn), len(mp.unverifiedTxn))
	mp.lock.Unlock()
}

// RemoveOverCapacity removes transactions with lowest fees until the total number of transactions
// in the Pool is within the capacity of the Pool.
func (mp *Pool) RemoveOverCapacity() {
	for mp.Count()-mp.capacity > 0 {
		mp.lock.Lock()
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
		updateMempoolMetrics(len(mp.unsortedTxn), len(mp.unverifiedTxn))
		mp.lock.Unlock()
	}

}

// NewMemPool returns a new Pool struct.
func NewMemPool(capacity int) Pool {
	return Pool{
		unsortedTxn:   make(map[util.Uint256]*item),
		unverifiedTxn: make(map[util.Uint256]*item),
		capacity:      capacity,
	}
}

// TryGetValue returns a transaction if it exists in the memory pool.
func (mp *Pool) TryGetValue(hash util.Uint256) (*transaction.Transaction, bool) {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	if pItem, ok := mp.unsortedTxn[hash]; ok {
		return pItem.txn, ok
	}

	if pItem, ok := mp.unverifiedTxn[hash]; ok {
		return pItem.txn, ok
	}

	return nil, false
}

// getLowestFeeTransaction returns the item with the lowest fee amongst the "verifiedTxnSorted"
// and "unverifiedTxnSorted" items along with a integer. The integer can assume two values, 1 and 2 which indicate
// that the item with the lowest fee was found in "verifiedTxnSorted" respectively in "unverifiedTxnSorted".
// "verifiedTxnSorted" and "unverifiedTxnSorted" are sorted slice order by transaction fee ascending. This means that
// the transaction with lowest fee start at index 0.
// Reference: GetLowestFeeTransaction method in C# (https://github.com/neo-project/neo/blob/master/neo/Ledger/MemoryPool.cs)
func getLowestFeeTransaction(verifiedTxnSorted items, unverifiedTxnSorted items) (*item, int) {
	minItem := min(unverifiedTxnSorted)
	verifiedMin := min(verifiedTxnSorted)
	if verifiedMin == nil || (minItem != nil && verifiedMin.CompareTo(minItem) >= 0) {
		return minItem, 2
	}

	minItem = verifiedMin
	return minItem, 1

}

// min returns the minimum item in a ascending sorted slice of pool items.
// The function can't be applied to unsorted slice!
func min(sortedPool items) *item {
	if len(sortedPool) == 0 {
		return nil
	}
	return sortedPool[0]
}

// GetVerifiedTransactions returns a slice of Input from all the transactions in the memory pool
// whose hash is not included in excludedHashes.
func (mp *Pool) GetVerifiedTransactions() []*transaction.Transaction {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	var t = make([]*transaction.Transaction, len(mp.unsortedTxn))
	var i int

	for _, p := range mp.unsortedTxn {
		t[i] = p.txn
		i++
	}

	return t
}

// verifyInputs is an internal unprotected version of Verify.
func (mp *Pool) verifyInputs(tx *transaction.Transaction) bool {
	if len(tx.Inputs) == 0 {
		return true
	}
	for _, item := range mp.unsortedTxn {
		for i := range item.txn.Inputs {
			for j := 0; j < len(tx.Inputs); j++ {
				if item.txn.Inputs[i] == tx.Inputs[j] {
					return false
				}
			}
		}
	}

	return true
}

// Verify verifies if the inputs of a transaction tx are already used in any other transaction in the memory pool.
// If yes, the transaction tx is not a valid transaction and the function return false.
// If no, the transaction tx is a valid transaction and the function return true.
func (mp *Pool) Verify(tx *transaction.Transaction) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	return mp.verifyInputs(tx)
}
