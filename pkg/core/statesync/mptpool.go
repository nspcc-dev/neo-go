package statesync

import (
	"bytes"
	"slices"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Pool stores unknown MPT nodes along with the corresponding paths (single node is
// allowed to have multiple MPT paths).
type Pool struct {
	lock   sync.RWMutex
	hashes map[util.Uint256][][]byte
}

// NewPool returns new MPT node hashes pool.
func NewPool() *Pool {
	return &Pool{
		hashes: make(map[util.Uint256][][]byte),
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
	// need to copy here, because we can modify existing array of paths inside the pool.
	return slices.Clone(paths), ok
}

// GetAll returns all MPT nodes with the corresponding paths from the pool.
func (mp *Pool) GetAll() map[util.Uint256][][]byte {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	return mp.hashes
}

// GetBatch returns set of unknown MPT nodes hashes (`limit` at max).
func (mp *Pool) GetBatch(limit int) []util.Uint256 {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	count := min(len(mp.hashes), limit)
	result := make([]util.Uint256, 0, limit)
	for h := range mp.hashes {
		if count == 0 {
			break
		}
		result = append(result, h)
		count--
	}
	return result
}

// Remove removes MPT node from the pool by the specified hash.
func (mp *Pool) Remove(hash util.Uint256) {
	mp.lock.Lock()
	defer mp.lock.Unlock()

	delete(mp.hashes, hash)
}

// Add adds path to the set of paths for the specified node.
func (mp *Pool) Add(hash util.Uint256, path []byte) {
	mp.lock.Lock()
	defer mp.lock.Unlock()

	mp.addPaths(hash, [][]byte{path})
}

// Update is an atomic operation and removes/adds specified nodes from/to the pool.
func (mp *Pool) Update(remove map[util.Uint256][][]byte, add map[util.Uint256][][]byte) {
	mp.lock.Lock()
	defer mp.lock.Unlock()

	for h, paths := range remove {
		old := mp.hashes[h]
		for _, path := range paths {
			i, found := slices.BinarySearchFunc(old, path, bytes.Compare)
			if found {
				old = slices.Delete(old, i, i+1)
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
}

// addPaths adds set of the specified node paths to the pool.
func (mp *Pool) addPaths(nodeHash util.Uint256, paths [][]byte) {
	old := mp.hashes[nodeHash]
	for _, path := range paths {
		i, found := slices.BinarySearchFunc(old, path, bytes.Compare)
		if found {
			// then path is already added
			continue
		}
		old = slices.Insert(old, i, path)
	}
	mp.hashes[nodeHash] = old
}

// Count returns the number of nodes in the pool.
func (mp *Pool) Count() int {
	mp.lock.RLock()
	defer mp.lock.RUnlock()

	return len(mp.hashes)
}
