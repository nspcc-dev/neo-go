package mempool

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
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

// TxWithFee combines transaction and its precalculated network fee.
type TxWithFee struct {
	Tx  *transaction.Transaction
	Fee util.Fixed8
}

// Pool stores the unconfirms transactions.
type Pool struct {
	lock         sync.RWMutex
	verifiedMap  map[util.Uint256]*item
	verifiedTxes items
	inputs       []*transaction.Input
	claims       []*transaction.Input

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
	return len(mp.verifiedTxes)
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

	return false
}

// findIndexForInput finds an index in a sorted Input pointers slice that is
// appropriate to place this input into (or which contains an identical Input).
func findIndexForInput(slice []*transaction.Input, input *transaction.Input) int {
	return sort.Search(len(slice), func(n int) bool {
		return input.Cmp(slice[n]) <= 0
	})
}

// pushInputToSortedSlice pushes new Input into the given slice.
func pushInputToSortedSlice(slice *[]*transaction.Input, input *transaction.Input) {
	n := findIndexForInput(*slice, input)
	*slice = append(*slice, input)
	if n != len(*slice)-1 {
		copy((*slice)[n+1:], (*slice)[n:])
		(*slice)[n] = input
	}
}

// dropInputFromSortedSlice removes given input from the given slice.
func dropInputFromSortedSlice(slice *[]*transaction.Input, input *transaction.Input) {
	n := findIndexForInput(*slice, input)
	if n == len(*slice) || *input != *(*slice)[n] {
		// Not present.
		return
	}
	copy((*slice)[n:], (*slice)[n+1:])
	*slice = (*slice)[:len(*slice)-1]
}

// Add tries to add given transaction to the Pool.
func (mp *Pool) Add(t *transaction.Transaction, fee Feer) error {
	var pItem = &item{
		txn:        t,
		timeStamp:  time.Now().UTC(),
		perByteFee: fee.FeePerByte(t),
		netFee:     fee.NetworkFee(t),
	}
	pItem.isLowPrio = fee.IsLowPriority(pItem.netFee)
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

	// We've reached our capacity already.
	if len(mp.verifiedTxes) == mp.capacity {
		// Less prioritized than the least prioritized we already have, won't fit.
		if n == len(mp.verifiedTxes) {
			mp.lock.Unlock()
			return ErrOOM
		}
		// Ditch the last one.
		unlucky := mp.verifiedTxes[len(mp.verifiedTxes)-1]
		delete(mp.verifiedMap, unlucky.txn.Hash())
		mp.verifiedTxes[len(mp.verifiedTxes)-1] = pItem
	} else {
		mp.verifiedTxes = append(mp.verifiedTxes, pItem)
	}
	if n != len(mp.verifiedTxes)-1 {
		copy(mp.verifiedTxes[n+1:], mp.verifiedTxes[n:])
		mp.verifiedTxes[n] = pItem
	}

	// For lots of inputs it might be easier to push them all and sort
	// afterwards, but that requires benchmarking.
	for i := range t.Inputs {
		pushInputToSortedSlice(&mp.inputs, &t.Inputs[i])
	}
	if t.Type == transaction.ClaimType {
		claim := t.Data.(*transaction.ClaimTX)
		for i := range claim.Claims {
			pushInputToSortedSlice(&mp.claims, &claim.Claims[i])
		}
	}

	updateMempoolMetrics(len(mp.verifiedTxes))
	mp.lock.Unlock()
	return nil
}

// Remove removes an item from the mempool, if it exists there (and does
// nothing if it doesn't).
func (mp *Pool) Remove(hash util.Uint256) {
	mp.lock.Lock()
	if it, ok := mp.verifiedMap[hash]; ok {
		var num int
		delete(mp.verifiedMap, hash)
		for num = range mp.verifiedTxes {
			if hash.Equals(mp.verifiedTxes[num].txn.Hash()) {
				break
			}
		}
		if num < len(mp.verifiedTxes)-1 {
			mp.verifiedTxes = append(mp.verifiedTxes[:num], mp.verifiedTxes[num+1:]...)
		} else if num == len(mp.verifiedTxes)-1 {
			mp.verifiedTxes = mp.verifiedTxes[:num]
		}
		for i := range it.txn.Inputs {
			dropInputFromSortedSlice(&mp.inputs, &it.txn.Inputs[i])
		}
		if it.txn.Type == transaction.ClaimType {
			claim := it.txn.Data.(*transaction.ClaimTX)
			for i := range claim.Claims {
				dropInputFromSortedSlice(&mp.claims, &claim.Claims[i])
			}
		}
	}
	updateMempoolMetrics(len(mp.verifiedTxes))
	mp.lock.Unlock()
}

// RemoveStale filters verified transactions through the given function keeping
// only the transactions for which it returns a true result. It's used to quickly
// drop part of the mempool that is now invalid after the block acceptance.
func (mp *Pool) RemoveStale(isOK func(*transaction.Transaction) bool) {
	mp.lock.Lock()
	// We can reuse already allocated slice
	// because items are iterated one-by-one in increasing order.
	newVerifiedTxes := mp.verifiedTxes[:0]
	newInputs := mp.inputs[:0]
	newClaims := mp.claims[:0]
	for _, itm := range mp.verifiedTxes {
		if isOK(itm.txn) {
			newVerifiedTxes = append(newVerifiedTxes, itm)
			for i := range itm.txn.Inputs {
				newInputs = append(newInputs, &itm.txn.Inputs[i])
			}
			if itm.txn.Type == transaction.ClaimType {
				claim := itm.txn.Data.(*transaction.ClaimTX)
				for i := range claim.Claims {
					newClaims = append(newClaims, &claim.Claims[i])
				}
			}
		} else {
			delete(mp.verifiedMap, itm.txn.Hash())
		}
	}
	sort.Slice(newInputs, func(i, j int) bool {
		return newInputs[i].Cmp(newInputs[j]) < 0
	})
	sort.Slice(newClaims, func(i, j int) bool {
		return newClaims[i].Cmp(newClaims[j]) < 0
	})
	mp.verifiedTxes = newVerifiedTxes
	mp.inputs = newInputs
	mp.claims = newClaims
	mp.lock.Unlock()
}

// NewMemPool returns a new Pool struct.
func NewMemPool(capacity int) Pool {
	return Pool{
		verifiedMap:  make(map[util.Uint256]*item),
		verifiedTxes: make([]*item, 0, capacity),
		capacity:     capacity,
	}
}

// TryGetValue returns a transaction and its fee if it exists in the memory pool.
func (mp *Pool) TryGetValue(hash util.Uint256) (*transaction.Transaction, util.Fixed8, bool) {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	if pItem, ok := mp.verifiedMap[hash]; ok {
		return pItem.txn, pItem.netFee, ok
	}

	return nil, 0, false
}

// GetVerifiedTransactions returns a slice of Input from all the transactions in the memory pool
// whose hash is not included in excludedHashes.
func (mp *Pool) GetVerifiedTransactions() []TxWithFee {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	var t = make([]TxWithFee, len(mp.verifiedTxes))

	for i := range mp.verifiedTxes {
		t[i].Tx = mp.verifiedTxes[i].txn
		t[i].Fee = mp.verifiedTxes[i].netFee
	}

	return t
}

// verifyInputs is an internal unprotected version of Verify.
func (mp *Pool) verifyInputs(tx *transaction.Transaction) bool {
	for i := range tx.Inputs {
		n := findIndexForInput(mp.inputs, &tx.Inputs[i])
		if n < len(mp.inputs) && *mp.inputs[n] == tx.Inputs[i] {
			return false
		}
	}
	if tx.Type == transaction.ClaimType {
		claim := tx.Data.(*transaction.ClaimTX)
		for i := range claim.Claims {
			n := findIndexForInput(mp.claims, &claim.Claims[i])
			if n < len(mp.claims) && *mp.claims[n] == claim.Claims[i] {
				return false
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
