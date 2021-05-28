package mempool

import (
	"errors"
	"fmt"
	"math/big"
	"math/bits"
	"sort"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/atomic"
)

var (
	// ErrInsufficientFunds is returned when Sender is not able to pay for
	// transaction being added irrespective of the other contents of the
	// pool.
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrConflict is returned when transaction being added is incompatible
	// with the contents of the memory pool (Sender doesn't have enough GAS
	// to pay for all transactions in the pool).
	ErrConflict = errors.New("conflicts with the memory pool")
	// ErrDup is returned when transaction being added is already present
	// in the memory pool.
	ErrDup = errors.New("already in the memory pool")
	// ErrOOM is returned when transaction just doesn't fit in the memory
	// pool because of its capacity constraints.
	ErrOOM = errors.New("out of memory")
	// ErrConflictsAttribute is returned when transaction conflicts with other transactions
	// due to its (or theirs) Conflicts attributes.
	ErrConflictsAttribute = errors.New("conflicts with memory pool due to Conflicts attribute")
	// ErrOracleResponse is returned when mempool already contains transaction
	// with the same oracle response ID and higher network fee.
	ErrOracleResponse = errors.New("conflicts with memory pool due to OracleResponse attribute")
)

// item represents a transaction in the the Memory pool.
type item struct {
	txn        *transaction.Transaction
	blockStamp uint32
	data       interface{}
}

// items is a slice of item.
type items []item

// utilityBalanceAndFees stores sender's balance and overall fees of
// sender's transactions which are currently in mempool.
type utilityBalanceAndFees struct {
	balance *big.Int
	feeSum  *big.Int
}

// Pool stores the unconfirms transactions.
type Pool struct {
	lock         sync.RWMutex
	verifiedMap  map[util.Uint256]*transaction.Transaction
	verifiedTxes items
	fees         map[util.Uint160]utilityBalanceAndFees
	// conflicts is a map of hashes of transactions which are conflicting with the mempooled ones.
	conflicts map[util.Uint256][]util.Uint256
	// oracleResp contains ids of oracle responses for tx in pool.
	oracleResp map[uint64]util.Uint256

	capacity   int
	feePerByte int64
	payerIndex int

	resendThreshold uint32
	resendFunc      func(*transaction.Transaction, interface{})

	// subscriptions for mempool events
	subscriptionsEnabled bool
	subscriptionsOn      atomic.Bool
	stopCh               chan struct{}
	events               chan mempoolevent.Event
	subCh                chan chan<- mempoolevent.Event // there are no other events in mempool except Event, so no need in generic subscribers type
	unsubCh              chan chan<- mempoolevent.Event
}

func (p items) Len() int           { return len(p) }
func (p items) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p items) Less(i, j int) bool { return p[i].CompareTo(p[j]) < 0 }

// CompareTo returns the difference between two items.
// difference < 0 implies p < otherP.
// difference = 0 implies p = otherP.
// difference > 0 implies p > otherP.
func (p item) CompareTo(otherP item) int {
	pHigh := p.txn.HasAttribute(transaction.HighPriority)
	otherHigh := otherP.txn.HasAttribute(transaction.HighPriority)
	if pHigh && !otherHigh {
		return 1
	} else if !pHigh && otherHigh {
		return -1
	}

	// Fees sorted ascending.
	if ret := int(p.txn.FeePerByte() - otherP.txn.FeePerByte()); ret != 0 {
		return ret
	}

	return int(p.txn.NetworkFee - otherP.txn.NetworkFee)
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

// HasConflicts returns true if transaction is already in pool or in the Conflicts attributes
// of pooled transactions or has Conflicts attributes for pooled transactions.
func (mp *Pool) HasConflicts(t *transaction.Transaction, fee Feer) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	if mp.containsKey(t.Hash()) {
		return true
	}
	if fee.P2PSigExtensionsEnabled() {
		// do not check sender's signature and fee
		if _, ok := mp.conflicts[t.Hash()]; ok {
			return true
		}
		for _, attr := range t.GetAttributes(transaction.ConflictsT) {
			if mp.containsKey(attr.Value.(*transaction.Conflicts).Hash) {
				return true
			}
		}
	}
	return false
}

// tryAddSendersFee tries to add system fee and network fee to the total sender`s fee in mempool
// and returns false if both balance check is required and sender has not enough GAS to pay.
func (mp *Pool) tryAddSendersFee(tx *transaction.Transaction, feer Feer, needCheck bool) bool {
	payer := tx.Signers[mp.payerIndex].Account
	senderFee, ok := mp.fees[payer]
	if !ok {
		senderFee.balance = feer.GetUtilityTokenBalance(payer)
		senderFee.feeSum = big.NewInt(0)
		mp.fees[payer] = senderFee
	}
	if needCheck {
		newFeeSum, err := checkBalance(tx, senderFee)
		if err != nil {
			return false
		}
		senderFee.feeSum.Set(newFeeSum)
	} else {
		senderFee.feeSum.Add(senderFee.feeSum, big.NewInt(tx.SystemFee+tx.NetworkFee))
	}
	return true
}

// checkBalance returns new cumulative fee balance for account or an error in
// case sender doesn't have enough GAS to pay for the transaction.
func checkBalance(tx *transaction.Transaction, balance utilityBalanceAndFees) (*big.Int, error) {
	txFee := big.NewInt(tx.SystemFee + tx.NetworkFee)
	if balance.balance.Cmp(txFee) < 0 {
		return nil, ErrInsufficientFunds
	}
	txFee.Add(txFee, balance.feeSum)
	if balance.balance.Cmp(txFee) < 0 {
		return nil, ErrConflict
	}
	return txFee, nil
}

// Add tries to add given transaction to the Pool.
func (mp *Pool) Add(t *transaction.Transaction, fee Feer, data ...interface{}) error {
	var pItem = item{
		txn:        t,
		blockStamp: fee.BlockHeight(),
	}
	if data != nil {
		pItem.data = data[0]
	}
	mp.lock.Lock()
	if mp.containsKey(t.Hash()) {
		mp.lock.Unlock()
		return ErrDup
	}
	conflictsToBeRemoved, err := mp.checkTxConflicts(t, fee)
	if err != nil {
		mp.lock.Unlock()
		return err
	}
	if attrs := t.GetAttributes(transaction.OracleResponseT); len(attrs) != 0 {
		id := attrs[0].Value.(*transaction.OracleResponse).ID
		h, ok := mp.oracleResp[id]
		if ok {
			if mp.verifiedMap[h].NetworkFee >= t.NetworkFee {
				mp.lock.Unlock()
				return ErrOracleResponse
			}
			mp.removeInternal(h, fee)
		}
		mp.oracleResp[id] = t.Hash()
	}

	if fee.P2PSigExtensionsEnabled() {
		// Remove conflicting transactions.
		for _, conflictingTx := range conflictsToBeRemoved {
			mp.removeInternal(conflictingTx.Hash(), fee)
		}
	}
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
		if fee.P2PSigExtensionsEnabled() {
			mp.removeConflictsOf(unlucky.txn)
		}
		if attrs := unlucky.txn.GetAttributes(transaction.OracleResponseT); len(attrs) != 0 {
			delete(mp.oracleResp, attrs[0].Value.(*transaction.OracleResponse).ID)
		}
		mp.verifiedTxes[len(mp.verifiedTxes)-1] = pItem
		if mp.subscriptionsOn.Load() {
			mp.events <- mempoolevent.Event{
				Type: mempoolevent.TransactionRemoved,
				Tx:   unlucky.txn,
				Data: unlucky.data,
			}
		}
	} else {
		mp.verifiedTxes = append(mp.verifiedTxes, pItem)
	}
	if n != len(mp.verifiedTxes)-1 {
		copy(mp.verifiedTxes[n+1:], mp.verifiedTxes[n:])
		mp.verifiedTxes[n] = pItem
	}
	mp.verifiedMap[t.Hash()] = t
	if fee.P2PSigExtensionsEnabled() {
		// Add conflicting hashes to the mp.conflicts list.
		for _, attr := range t.GetAttributes(transaction.ConflictsT) {
			hash := attr.Value.(*transaction.Conflicts).Hash
			mp.conflicts[hash] = append(mp.conflicts[hash], t.Hash())
		}
	}
	// we already checked balance in checkTxConflicts, so don't need to check again
	mp.tryAddSendersFee(pItem.txn, fee, false)

	updateMempoolMetrics(len(mp.verifiedTxes))
	mp.lock.Unlock()

	if mp.subscriptionsOn.Load() {
		mp.events <- mempoolevent.Event{
			Type: mempoolevent.TransactionAdded,
			Tx:   pItem.txn,
			Data: pItem.data,
		}
	}
	return nil
}

// Remove removes an item from the mempool, if it exists there (and does
// nothing if it doesn't).
func (mp *Pool) Remove(hash util.Uint256, feer Feer) {
	mp.lock.Lock()
	mp.removeInternal(hash, feer)
	mp.lock.Unlock()
}

// removeInternal is an internal unlocked representation of Remove.
func (mp *Pool) removeInternal(hash util.Uint256, feer Feer) {
	if tx, ok := mp.verifiedMap[hash]; ok {
		var num int
		delete(mp.verifiedMap, hash)
		for num = range mp.verifiedTxes {
			if hash.Equals(mp.verifiedTxes[num].txn.Hash()) {
				break
			}
		}
		itm := mp.verifiedTxes[num]
		if num < len(mp.verifiedTxes)-1 {
			mp.verifiedTxes = append(mp.verifiedTxes[:num], mp.verifiedTxes[num+1:]...)
		} else if num == len(mp.verifiedTxes)-1 {
			mp.verifiedTxes = mp.verifiedTxes[:num]
		}
		payer := itm.txn.Signers[mp.payerIndex].Account
		senderFee := mp.fees[payer]
		senderFee.feeSum.Sub(senderFee.feeSum, big.NewInt(tx.SystemFee+tx.NetworkFee))
		mp.fees[payer] = senderFee
		if feer.P2PSigExtensionsEnabled() {
			// remove all conflicting hashes from mp.conflicts list
			mp.removeConflictsOf(tx)
		}
		if attrs := tx.GetAttributes(transaction.OracleResponseT); len(attrs) != 0 {
			delete(mp.oracleResp, attrs[0].Value.(*transaction.OracleResponse).ID)
		}
		if mp.subscriptionsOn.Load() {
			mp.events <- mempoolevent.Event{
				Type: mempoolevent.TransactionRemoved,
				Tx:   itm.txn,
				Data: itm.data,
			}
		}
	}
	updateMempoolMetrics(len(mp.verifiedTxes))
}

// RemoveStale filters verified transactions through the given function keeping
// only the transactions for which it returns a true result. It's used to quickly
// drop part of the mempool that is now invalid after the block acceptance.
func (mp *Pool) RemoveStale(isOK func(*transaction.Transaction) bool, feer Feer) {
	mp.lock.Lock()
	policyChanged := mp.loadPolicy(feer)
	// We can reuse already allocated slice
	// because items are iterated one-by-one in increasing order.
	newVerifiedTxes := mp.verifiedTxes[:0]
	mp.fees = make(map[util.Uint160]utilityBalanceAndFees) // it'd be nice to reuse existing map, but we can't easily clear it
	if feer.P2PSigExtensionsEnabled() {
		mp.conflicts = make(map[util.Uint256][]util.Uint256)
	}
	height := feer.BlockHeight()
	var (
		staleItems []item
	)
	for _, itm := range mp.verifiedTxes {
		if isOK(itm.txn) && mp.checkPolicy(itm.txn, policyChanged) && mp.tryAddSendersFee(itm.txn, feer, true) {
			newVerifiedTxes = append(newVerifiedTxes, itm)
			if feer.P2PSigExtensionsEnabled() {
				for _, attr := range itm.txn.GetAttributes(transaction.ConflictsT) {
					hash := attr.Value.(*transaction.Conflicts).Hash
					mp.conflicts[hash] = append(mp.conflicts[hash], itm.txn.Hash())
				}
			}
			if mp.resendThreshold != 0 {
				// item is resend at resendThreshold, 2*resendThreshold, 4*resendThreshold ...
				// so quotient must be a power of two.
				diff := (height - itm.blockStamp)
				if diff%mp.resendThreshold == 0 && bits.OnesCount32(diff/mp.resendThreshold) == 1 {
					staleItems = append(staleItems, itm)
				}
			}
		} else {
			delete(mp.verifiedMap, itm.txn.Hash())
			if attrs := itm.txn.GetAttributes(transaction.OracleResponseT); len(attrs) != 0 {
				delete(mp.oracleResp, attrs[0].Value.(*transaction.OracleResponse).ID)
			}
			if mp.subscriptionsOn.Load() {
				mp.events <- mempoolevent.Event{
					Type: mempoolevent.TransactionRemoved,
					Tx:   itm.txn,
					Data: itm.data,
				}
			}
		}
	}
	if len(staleItems) != 0 {
		go mp.resendStaleItems(staleItems)
	}
	mp.verifiedTxes = newVerifiedTxes
	mp.lock.Unlock()
}

// loadPolicy updates feePerByte field and returns whether policy has been
// changed.
func (mp *Pool) loadPolicy(feer Feer) bool {
	newFeePerByte := feer.FeePerByte()
	if newFeePerByte > mp.feePerByte {
		mp.feePerByte = newFeePerByte
		return true
	}
	return false
}

// checkPolicy checks whether transaction fits policy.
func (mp *Pool) checkPolicy(tx *transaction.Transaction, policyChanged bool) bool {
	if !policyChanged || tx.FeePerByte() >= mp.feePerByte {
		return true
	}
	return false
}

// New returns a new Pool struct.
func New(capacity int, payerIndex int, enableSubscriptions bool) *Pool {
	mp := &Pool{
		verifiedMap:          make(map[util.Uint256]*transaction.Transaction),
		verifiedTxes:         make([]item, 0, capacity),
		capacity:             capacity,
		payerIndex:           payerIndex,
		fees:                 make(map[util.Uint160]utilityBalanceAndFees),
		conflicts:            make(map[util.Uint256][]util.Uint256),
		oracleResp:           make(map[uint64]util.Uint256),
		subscriptionsEnabled: enableSubscriptions,
		stopCh:               make(chan struct{}),
		events:               make(chan mempoolevent.Event),
		subCh:                make(chan chan<- mempoolevent.Event),
		unsubCh:              make(chan chan<- mempoolevent.Event),
	}
	mp.subscriptionsOn.Store(false)
	return mp
}

// SetResendThreshold sets threshold after which transaction will be considered stale
// and returned for retransmission by `GetStaleTransactions`.
func (mp *Pool) SetResendThreshold(h uint32, f func(*transaction.Transaction, interface{})) {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	mp.resendThreshold = h
	mp.resendFunc = f
}

func (mp *Pool) resendStaleItems(items []item) {
	for i := range items {
		mp.resendFunc(items[i].txn, items[i].data)
	}
}

// TryGetValue returns a transaction and its fee if it exists in the memory pool.
func (mp *Pool) TryGetValue(hash util.Uint256) (*transaction.Transaction, bool) {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	if tx, ok := mp.verifiedMap[hash]; ok {
		return tx, ok
	}

	return nil, false
}

// TryGetData returns data associated with the specified transaction if it exists in the memory pool.
func (mp *Pool) TryGetData(hash util.Uint256) (interface{}, bool) {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	if tx, ok := mp.verifiedMap[hash]; ok {
		itm := item{txn: tx}
		n := sort.Search(len(mp.verifiedTxes), func(n int) bool {
			return itm.CompareTo(mp.verifiedTxes[n]) >= 0
		})
		if n < len(mp.verifiedTxes) {
			for i := n; i < len(mp.verifiedTxes); i++ { // items may have equal priority, so `n` is the left bound of the items which are as prioritized as the desired `itm`.
				if mp.verifiedTxes[i].txn.Hash() == hash {
					return mp.verifiedTxes[i].data, ok
				}
				if itm.CompareTo(mp.verifiedTxes[i]) != 0 {
					break
				}
			}
		}
	}

	return nil, false
}

// GetVerifiedTransactions returns a slice of transactions with their fees.
func (mp *Pool) GetVerifiedTransactions() []*transaction.Transaction {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	var t = make([]*transaction.Transaction, len(mp.verifiedTxes))

	for i := range mp.verifiedTxes {
		t[i] = mp.verifiedTxes[i].txn
	}

	return t
}

// checkTxConflicts is an internal unprotected version of Verify. It takes into
// consideration conflicting transactions which are about to be removed from mempool.
func (mp *Pool) checkTxConflicts(tx *transaction.Transaction, fee Feer) ([]*transaction.Transaction, error) {
	payer := tx.Signers[mp.payerIndex].Account
	actualSenderFee, ok := mp.fees[payer]
	if !ok {
		actualSenderFee.balance = fee.GetUtilityTokenBalance(payer)
		actualSenderFee.feeSum = big.NewInt(0)
	}

	var expectedSenderFee utilityBalanceAndFees
	// Check Conflicts attributes.
	var conflictsToBeRemoved []*transaction.Transaction
	if fee.P2PSigExtensionsEnabled() {
		// Step 1: check if `tx` was in attributes of mempooled transactions.
		if conflictingHashes, ok := mp.conflicts[tx.Hash()]; ok {
			for _, hash := range conflictingHashes {
				existingTx := mp.verifiedMap[hash]
				if existingTx.HasSigner(payer) && existingTx.NetworkFee > tx.NetworkFee {
					return nil, fmt.Errorf("%w: conflicting transaction %s has bigger network fee", ErrConflictsAttribute, existingTx.Hash().StringBE())
				}
				conflictsToBeRemoved = append(conflictsToBeRemoved, existingTx)
			}
		}
		// Step 2: check if mempooled transactions were in `tx`'s attributes.
		for _, attr := range tx.GetAttributes(transaction.ConflictsT) {
			hash := attr.Value.(*transaction.Conflicts).Hash
			existingTx, ok := mp.verifiedMap[hash]
			if !ok {
				continue
			}
			if !tx.HasSigner(existingTx.Signers[mp.payerIndex].Account) {
				return nil, fmt.Errorf("%w: not signed by the sender of conflicting transaction %s", ErrConflictsAttribute, existingTx.Hash().StringBE())
			}
			if existingTx.NetworkFee >= tx.NetworkFee {
				return nil, fmt.Errorf("%w: conflicting transaction %s has bigger or equal network fee", ErrConflictsAttribute, existingTx.Hash().StringBE())
			}
			conflictsToBeRemoved = append(conflictsToBeRemoved, existingTx)
		}
		// Step 3: take into account sender's conflicting transactions before balance check.
		expectedSenderFee = utilityBalanceAndFees{
			balance: new(big.Int).Set(actualSenderFee.balance),
			feeSum:  new(big.Int).Set(actualSenderFee.feeSum),
		}
		for _, conflictingTx := range conflictsToBeRemoved {
			if conflictingTx.Signers[mp.payerIndex].Account.Equals(payer) {
				expectedSenderFee.feeSum.Sub(expectedSenderFee.feeSum, big.NewInt(conflictingTx.SystemFee+conflictingTx.NetworkFee))
			}
		}
	} else {
		expectedSenderFee = actualSenderFee
	}
	_, err := checkBalance(tx, expectedSenderFee)
	return conflictsToBeRemoved, err
}

// Verify checks if a Sender of tx is able to pay for it (and all the other
// transactions in the pool). If yes, the transaction tx is a valid
// transaction and the function returns true. If no, the transaction tx is
// considered to be invalid the function returns false.
func (mp *Pool) Verify(tx *transaction.Transaction, feer Feer) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()
	_, err := mp.checkTxConflicts(tx, feer)
	return err == nil
}

// removeConflictsOf removes hash of the given transaction from the conflicts list
// for each Conflicts attribute.
func (mp *Pool) removeConflictsOf(tx *transaction.Transaction) {
	// remove all conflicting hashes from mp.conflicts list
	for _, attr := range tx.GetAttributes(transaction.ConflictsT) {
		conflictsHash := attr.Value.(*transaction.Conflicts).Hash
		if len(mp.conflicts[conflictsHash]) == 1 {
			delete(mp.conflicts, conflictsHash)
			continue
		}
		for i, existingHash := range mp.conflicts[conflictsHash] {
			if existingHash == tx.Hash() {
				// tx.Hash can occur in the conflicting hashes array only once, because we can't add the same transaction to the mempol twice
				mp.conflicts[conflictsHash] = append(mp.conflicts[conflictsHash][:i], mp.conflicts[conflictsHash][i+1:]...)
				break
			}
		}
	}
}
