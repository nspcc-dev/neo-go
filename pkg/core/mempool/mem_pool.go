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
	lock           sync.RWMutex
	verifiedMap    map[util.Uint256]*item
	unverifiedMap  map[util.Uint256]*item
	verifiedTxes   items
	unverifiedTxes items

	capacity int
}

func (p items) Len() int           { return len(p) }
func (p items) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p items) Less(i, j int) bool { return p[i].CompareTo(p[j]) < 0 }

// CompareTo returns the difference between two items.
// difference < 0 implies p < otherP.
// difference = 0 implies p = otherP.
// difference > 0 implies p > otherP.
func (p *item) CompareTo(otherP *item) int {
	if otherP == nil {
		return 1
	}

	if !p.isLowPrio && otherP.isLowPrio {
		return 1
	}

	if p.isLowPrio && !otherP.isLowPrio {
		return -1
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
	return mp.count()
}

// count is an internal unlocked version of Count.
func (mp *Pool) count() int {
	return len(mp.verifiedTxes) + len(mp.unverifiedTxes)
}

// ContainsKey checks if a transactions hash is in the Pool.
func (mp *Pool) ContainsKey(hash util.Uint256) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	return mp.containsKey(hash)
}

// containsKey is an internal unlocked version of ContainsKey.
func (mp *Pool) containsKey(hash util.Uint256) bool {
	if _, ok := mp.verifiedMap[hash]; ok {
		return true
	}

	if _, ok := mp.unverifiedMap[hash]; ok {
		return true
	}

	return false
}

// Add tries to add given transaction to the Pool.
func (mp *Pool) Add(t *transaction.Transaction, fee Feer) error {
	var pItem = &item{
		txn:        t,
		timeStamp:  time.Now().UTC(),
		perByteFee: fee.FeePerByte(t),
		netFee:     fee.NetworkFee(t),
		isLowPrio:  fee.IsLowPriority(t),
	}
	mp.lock.Lock()
	if !mp.verifyInputs(t) {
		mp.lock.Unlock()
		return ErrConflict
	}
	if mp.containsKey(t.Hash()) {
		mp.lock.Unlock()
		return ErrDup
	}

	mp.verifiedMap[t.Hash()] = pItem
	// Insert into sorted array (from max to min, that could also be done
	// using sort.Sort(sort.Reverse()), but it incurs more overhead. Notice
	// also that we're searching for position that is strictly more
	// prioritized than our new item because we do expect a lot of
	// transactions with the same priority and appending to the end of the
	// slice is always more efficient.
	n := sort.Search(len(mp.verifiedTxes), func(n int) bool {
		return pItem.CompareTo(mp.verifiedTxes[n]) > 0
	})
	mp.verifiedTxes = append(mp.verifiedTxes, pItem)
	if n != len(mp.verifiedTxes) {
		copy(mp.verifiedTxes[n+1:], mp.verifiedTxes[n:])
		mp.verifiedTxes[n] = pItem
	}
	mp.lock.Unlock()

	if mp.Count() > mp.capacity {
		mp.RemoveOverCapacity()
	}
	mp.lock.RLock()
	_, ok := mp.verifiedMap[t.Hash()]
	updateMempoolMetrics(len(mp.verifiedTxes), len(mp.unverifiedTxes))
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
		txMap  map[util.Uint256]*item
		txPool *items
	}{
		{txMap: mp.verifiedMap, txPool: &mp.verifiedTxes},
		{txMap: mp.unverifiedMap, txPool: &mp.unverifiedTxes},
	}
	mp.lock.Lock()
	for _, mapAndPool := range mapAndPools {
		if _, ok := mapAndPool.txMap[hash]; ok {
			var num int
			delete(mapAndPool.txMap, hash)
			for num := range *mapAndPool.txPool {
				if hash.Equals((*mapAndPool.txPool)[num].txn.Hash()) {
					break
				}
			}
			if num < len(*mapAndPool.txPool)-1 {
				*mapAndPool.txPool = append((*mapAndPool.txPool)[:num], (*mapAndPool.txPool)[num+1:]...)
			} else if num == len(*mapAndPool.txPool)-1 {
				*mapAndPool.txPool = (*mapAndPool.txPool)[:num]
			}
		}
	}
	updateMempoolMetrics(len(mp.verifiedTxes), len(mp.unverifiedTxes))
	mp.lock.Unlock()
}

// RemoveOverCapacity removes transactions with lowest fees until the total number of transactions
// in the Pool is within the capacity of the Pool.
func (mp *Pool) RemoveOverCapacity() {
	mp.lock.Lock()
	for mp.count()-mp.capacity > 0 {
		minItem, argPosition := getLowestFeeTransaction(mp.verifiedTxes, mp.unverifiedTxes)
		if argPosition == 1 {
			// minItem belongs to the mp.sortedLowPrioTxn slice.
			// The corresponding unsorted pool is is mp.unsortedTxn.
			delete(mp.verifiedMap, minItem.txn.Hash())
			mp.verifiedTxes = mp.verifiedTxes[:len(mp.verifiedTxes)-1]
		} else {
			// minItem belongs to the mp.unverifiedSortedLowPrioTxn slice.
			// The corresponding unsorted pool is is mp.unverifiedTxn.
			delete(mp.unverifiedMap, minItem.txn.Hash())
			mp.unverifiedTxes = mp.unverifiedTxes[:len(mp.unverifiedTxes)-1]
		}
		updateMempoolMetrics(len(mp.verifiedTxes), len(mp.unverifiedTxes))
	}
	mp.lock.Unlock()
}

// NewMemPool returns a new Pool struct.
func NewMemPool(capacity int) Pool {
	return Pool{
		verifiedMap:    make(map[util.Uint256]*item),
		unverifiedMap:  make(map[util.Uint256]*item),
		verifiedTxes:   make([]*item, 0, capacity),
		unverifiedTxes: make([]*item, 0, capacity),
		capacity:       capacity,
	}
}

// TryGetValue returns a transaction if it exists in the memory pool.
func (mp *Pool) TryGetValue(hash util.Uint256) (*transaction.Transaction, bool) {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	if pItem, ok := mp.verifiedMap[hash]; ok {
		return pItem.txn, ok
	}

	if pItem, ok := mp.unverifiedMap[hash]; ok {
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
	return sortedPool[len(sortedPool)-1]
}

// GetVerifiedTransactions returns a slice of Input from all the transactions in the memory pool
// whose hash is not included in excludedHashes.
func (mp *Pool) GetVerifiedTransactions() []*transaction.Transaction {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	var t = make([]*transaction.Transaction, len(mp.verifiedMap))
	var i int

	for _, p := range mp.verifiedMap {
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
	for num := range mp.verifiedTxes {
		txn := mp.verifiedTxes[num].txn
		for i := range txn.Inputs {
			for j := 0; j < len(tx.Inputs); j++ {
				if txn.Inputs[i] == tx.Inputs[j] {
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
