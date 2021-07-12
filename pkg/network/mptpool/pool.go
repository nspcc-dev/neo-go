package mptpool

import (
	"bytes"
	"sort"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/atomic"
)

// Pool stores unknown MPT nodes along with the corresponding paths (single node is
// allowed to have multiple MPT paths).
type Pool struct {
	lock   sync.RWMutex
	hashes map[util.Uint256][][]byte

	resendThreshold time.Duration
	resendFunc      func(map[util.Uint256]bool)

	batchCh          chan struct{}
	retransmissionOn *atomic.Bool
}

// New returns new MPT node hashes pool.
func New() *Pool {
	return &Pool{
		hashes:           make(map[util.Uint256][][]byte),
		batchCh:          make(chan struct{}),
		retransmissionOn: atomic.NewBool(false),
	}
}

// ContainsKey checks if MPT node with the specified hash is in the Pool.
func (mp *Pool) ContainsKey(hash util.Uint256) bool {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	_, ok := mp.hashes[hash]
	return ok
}

// TryGet returns a set of MPT paths for the specified HashNode.
func (mp *Pool) TryGet(hash util.Uint256) ([][]byte, bool) {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	paths, ok := mp.hashes[hash]
	return paths, ok
}

// GetAll returns all MPT nodes with the corresponding paths from the pool.
func (mp *Pool) GetAll() map[util.Uint256][][]byte {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	return mp.hashes
}

// Remove removes MPT node from the pool by the specified hash.
func (mp *Pool) Remove(hash util.Uint256) {
	mp.lock.Lock()
	defer mp.lock.Unlock()

	delete(mp.hashes, hash)
}

// Add adds path to the set of paths for the specified node.
func (mp *Pool) Add(hash util.Uint256, path []byte) {
	// TODO: we may want to copy `item` here, but this pool is for our internal purposes, so maybe
	// we don't need to.
	mp.lock.Lock()
	defer mp.lock.Unlock()

	mp.addPaths(hash, [][]byte{path})
}

// Update is an atomic operation and removes/adds specified nodes from/to the pool.
func (mp *Pool) Update(remove map[util.Uint256][][]byte, add map[util.Uint256][][]byte) {
	mp.lock.Lock()

	for h, paths := range remove {
		old := mp.hashes[h]
		for _, path := range paths {
			i := sort.Search(len(old), func(i int) bool {
				return bytes.Compare(old[i], path) >= 0
			})
			if i < len(old) && bytes.Equal(old[i], path) {
				old = append(old[:i], old[i+1:]...)
			}
		}
		if len(old) == 0 {
			delete(mp.hashes, h)
		} else {
			mp.hashes[h] = old
		}
	}
	for h, paths := range add {
		mp.addPaths(h, paths)
	}
	mp.lock.Unlock()

	if mp.retransmissionOn.Load() {
		mp.batchCh <- struct{}{}
	}
}

// addPaths adds set of the specified node paths to the pool.
func (mp *Pool) addPaths(nodeHash util.Uint256, paths [][]byte) {
	old := mp.hashes[nodeHash]
	for _, path := range paths {
		i := sort.Search(len(old), func(i int) bool {
			return bytes.Compare(old[i], path) >= 0
		})
		if i < len(old) && bytes.Equal(old[i], path) {
			// then path is already added
			continue
		}
		old = append(old, path)
		if i != len(old)-1 {
			copy(old[i+1:], old[i:])
			old[i] = path
		}
	}
	mp.hashes[nodeHash] = old
}

// Count returns the number of nodes in the pool.
func (mp *Pool) Count() int {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	return len(mp.hashes)
}

// SetResendThreshold sets threshold after which MPT data requests will be considered
// stale and retransmitted by `ResendStaleItems` routine.
func (mp *Pool) SetResendThreshold(t time.Duration, f func(map[util.Uint256]bool)) {
	mp.lock.Lock()
	defer mp.lock.Unlock()
	mp.resendThreshold = t
	mp.resendFunc = f
}

// ResendStaleItems starts cycle that manages stale MPT data requests (must be run
// in a separate go-routine). The cycle will be automatically exited after MPT sync
// process is ended, i.e. when no nodes are left in pool after Update.
func (mp *Pool) ResendStaleItems() {
	if !mp.retransmissionOn.CAS(false, true) {
		return
	}
	timer := time.NewTimer(mp.resendThreshold)
	for {
		select {
		case <-timer.C:
			stale := make(map[util.Uint256]bool)
			for h, _ := range mp.GetAll() {
				stale[h] = true
			}
			mp.resendFunc(stale)
			timer.Reset(mp.resendThreshold)
		case <-mp.batchCh:
			if mp.Count() == 0 {
				mp.retransmissionOn.Store(false)
				return
			}
			// new batch is firstly requested by server, no need to duplicate requests
			timer.Reset(mp.resendThreshold)
		}
	}
}
