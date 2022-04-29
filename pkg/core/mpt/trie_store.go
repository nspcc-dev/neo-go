package mpt

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// TrieStore is an MPT-based storage implementation for storing and retrieving
// historic blockchain data. TrieStore is supposed to be used within transaction
// script invocations only, thus only contract storage related operations are
// supported. All storage-related operations are being performed using historical
// storage data retrieved from MPT state. TrieStore is read-only and does not
// support put-related operations, thus, it should always be wrapped into
// MemCachedStore for proper puts handling. TrieStore never changes the provided
// backend store.
type TrieStore struct {
	trie *Trie
}

// ErrForbiddenTrieStoreOperation is returned when operation is not supposed to
// be performed over MPT-based Store.
var ErrForbiddenTrieStoreOperation = errors.New("operation is not allowed to be performed over TrieStore")

// NewTrieStore returns a new ready to use MPT-backed storage.
func NewTrieStore(root util.Uint256, mode TrieMode, backed storage.Store) *TrieStore {
	cache, ok := backed.(*storage.MemCachedStore)
	if !ok {
		cache = storage.NewMemCachedStore(backed)
	}
	tr := NewTrie(NewHashNode(root), mode, cache)
	return &TrieStore{
		trie: tr,
	}
}

// Get implements the Store interface.
func (m *TrieStore) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("%w: Get is supported only for contract storage items", ErrForbiddenTrieStoreOperation)
	}
	switch storage.KeyPrefix(key[0]) {
	case storage.STStorage, storage.STTempStorage:
		res, err := m.trie.Get(key[1:])
		if err != nil && errors.Is(err, ErrNotFound) {
			// Mimic the real storage behaviour.
			return nil, storage.ErrKeyNotFound
		}
		return res, err
	default:
		return nil, fmt.Errorf("%w: Get is supported only for contract storage items", ErrForbiddenTrieStoreOperation)
	}
}

// PutChangeSet implements the Store interface.
func (m *TrieStore) PutChangeSet(puts map[string][]byte, stor map[string][]byte) error {
	// Only Get and Seek should be supported, as TrieStore is read-only and is always
	// should be wrapped by MemCachedStore to properly support put operations (if any).
	return fmt.Errorf("%w: PutChangeSet is not supported", ErrForbiddenTrieStoreOperation)
}

// Seek implements the Store interface.
func (m *TrieStore) Seek(rng storage.SeekRange, f func(k, v []byte) bool) {
	prefix := storage.KeyPrefix(rng.Prefix[0])
	if prefix != storage.STStorage && prefix != storage.STTempStorage { // Prefix is always non-empty.
		panic(fmt.Errorf("%w: Seek is supported only for contract storage items", ErrForbiddenTrieStoreOperation))
	}
	prefixP := toNibbles(rng.Prefix[1:])
	fromP := []byte{}
	if len(rng.Start) > 0 {
		fromP = toNibbles(rng.Start)
	}
	_, start, path, err := m.trie.getWithPath(m.trie.root, prefixP, false)
	if err != nil {
		// Failed to determine the start node => no matching items.
		return
	}
	path = path[len(prefixP):]

	if len(fromP) > 0 {
		if len(path) <= len(fromP) && bytes.HasPrefix(fromP, path) {
			fromP = fromP[len(path):]
		} else if len(path) > len(fromP) && bytes.HasPrefix(path, fromP) {
			fromP = []byte{}
		} else {
			cmp := bytes.Compare(path, fromP)
			if cmp < 0 == rng.Backwards {
				// No matching items.
				return
			}
			fromP = []byte{}
		}
	}

	b := NewBillet(m.trie.root.Hash(), m.trie.mode, 0, m.trie.Store)
	process := func(pathToNode []byte, node Node, _ []byte) bool {
		if leaf, ok := node.(*LeafNode); ok {
			// (*Billet).traverse includes `from` path into the result if so. It's OK for Seek, so shouldn't be filtered out.
			kv := storage.KeyValue{
				Key:   append(slice.Copy(rng.Prefix), pathToNode...), // Do not cut prefix.
				Value: slice.Copy(leaf.value),
			}
			return !f(kv.Key, kv.Value) // Should return whether to stop.
		}
		return false
	}
	_, err = b.traverse(start, path, fromP, process, false, rng.Backwards)
	if err != nil && !errors.Is(err, errStop) {
		panic(fmt.Errorf("failed to perform Seek operation on TrieStore: %w", err))
	}
}

// SeekGC implements the Store interface.
func (m *TrieStore) SeekGC(rng storage.SeekRange, keep func(k, v []byte) bool) error {
	return fmt.Errorf("%w: SeekGC is not supported", ErrForbiddenTrieStoreOperation)
}

// Close implements the Store interface.
func (m *TrieStore) Close() error {
	m.trie = nil
	return nil
}
