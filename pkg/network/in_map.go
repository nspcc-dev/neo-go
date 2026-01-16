package network

import (
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// inMap manages incoming hashable items (transactions) that are enqueued by
// the server to be added to the underlying memory pool.
type inMap[T hash.Hashable] struct {
	lock     sync.RWMutex
	in       chan T
	m        map[util.Uint256]struct{}
	isInPool func(h util.Uint256) bool
}

// newInMap creates a new instance of inMap.
func newInMap[T hash.Hashable](capacity int, isInPool func(h util.Uint256) bool) *inMap[T] {
	return &inMap[T]{
		in:       make(chan T, capacity),
		m:        make(map[util.Uint256]struct{}),
		isInPool: isInPool,
	}
}

// In returns a channel that emits items added to inMap. It's the caller's duty
// to drain and close this channel once the work with inMap is done.
func (m *inMap[T]) In() <-chan T {
	return m.in
}

// Contains returns whether inMap or corresponding memory pool contains an item
// with the given hash.
func (m *inMap[T]) Contains(h util.Uint256) bool {
	m.lock.RLock()
	_, ok := m.m[h]
	m.lock.RUnlock()

	return ok || m.isInPool(h)
}

// Add atomically adds the item to inMap if it's not yet there or in the
// corresponding mempool.
func (m *inMap[T]) Add(item T) {
	// It's OK for it to fail for various reasons like tx/notary request already existing
	// in the pool, so return silently without error.
	m.lock.Lock()
	_, ok := m.m[item.Hash()]
	if ok || m.isInPool(item.Hash()) {
		m.lock.Unlock()
		return
	}
	m.m[item.Hash()] = struct{}{}
	m.lock.Unlock()

	m.in <- item
}

// Remove atomically removes the item from inMap.
func (m *inMap[T]) Remove(h util.Uint256) {
	m.lock.Lock()
	delete(m.m, h)
	m.lock.Unlock()
}
