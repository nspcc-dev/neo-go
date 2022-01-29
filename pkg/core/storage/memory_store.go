package storage

import (
	"bytes"
	"sort"
	"strings"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// MemoryStore is an in-memory implementation of a Store, mainly
// used for testing. Do not use MemoryStore in production.
type MemoryStore struct {
	mut sync.RWMutex
	mem map[string][]byte
}

// MemoryBatch is an in-memory batch compatible with MemoryStore.
type MemoryBatch struct {
	MemoryStore
}

// Put implements the Batch interface.
func (b *MemoryBatch) Put(k, v []byte) {
	b.MemoryStore.put(string(k), slice.Copy(v))
}

// Delete implements Batch interface.
func (b *MemoryBatch) Delete(k []byte) {
	b.MemoryStore.drop(string(k))
}

// NewMemoryStore creates a new MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		mem: make(map[string][]byte),
	}
}

// Get implements the Store interface.
func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	if val, ok := s.mem[string(key)]; ok && val != nil {
		return val, nil
	}
	return nil, ErrKeyNotFound
}

// put puts a key-value pair into the store, it's supposed to be called
// with mutex locked.
func (s *MemoryStore) put(key string, value []byte) {
	s.mem[key] = value
}

// Put implements the Store interface. Never returns an error.
func (s *MemoryStore) Put(key, value []byte) error {
	newKey := string(key)
	vcopy := slice.Copy(value)
	s.mut.Lock()
	s.put(newKey, vcopy)
	s.mut.Unlock()
	return nil
}

// drop deletes a key-value pair from the store, it's supposed to be called
// with mutex locked.
func (s *MemoryStore) drop(key string) {
	s.mem[key] = nil
}

// Delete implements Store interface. Never returns an error.
func (s *MemoryStore) Delete(key []byte) error {
	newKey := string(key)
	s.mut.Lock()
	s.drop(newKey)
	s.mut.Unlock()
	return nil
}

// PutBatch implements the Store interface. Never returns an error.
func (s *MemoryStore) PutBatch(batch Batch) error {
	b := batch.(*MemoryBatch)
	return s.PutChangeSet(b.mem)
}

// PutChangeSet implements the Store interface. Never returns an error.
func (s *MemoryStore) PutChangeSet(puts map[string][]byte) error {
	s.mut.Lock()
	for k := range puts {
		s.put(k, puts[k])
	}
	s.mut.Unlock()
	return nil
}

// Seek implements the Store interface.
func (s *MemoryStore) Seek(rng SeekRange, f func(k, v []byte) bool) {
	s.mut.RLock()
	s.seek(rng, f)
	s.mut.RUnlock()
}

// SeekAll is like seek but also iterates over deleted items.
func (s *MemoryStore) SeekAll(key []byte, f func(k, v []byte)) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	sk := string(key)
	for k, v := range s.mem {
		if strings.HasPrefix(k, sk) {
			f([]byte(k), v)
		}
	}
}

// seek is an internal unlocked implementation of Seek. `start` denotes whether
// seeking starting from the provided prefix should be performed. Backwards
// seeking from some point is supported with corresponding SeekRange field set.
func (s *MemoryStore) seek(rng SeekRange, f func(k, v []byte) bool) {
	sPrefix := string(rng.Prefix)
	lPrefix := len(sPrefix)
	sStart := string(rng.Start)
	lStart := len(sStart)
	var memList []KeyValue

	isKeyOK := func(key string) bool {
		return strings.HasPrefix(key, sPrefix) && (lStart == 0 || strings.Compare(key[lPrefix:], sStart) >= 0)
	}
	if rng.Backwards {
		isKeyOK = func(key string) bool {
			return strings.HasPrefix(key, sPrefix) && (lStart == 0 || strings.Compare(key[lPrefix:], sStart) <= 0)
		}
	}
	less := func(k1, k2 []byte) bool {
		res := bytes.Compare(k1, k2)
		return res != 0 && rng.Backwards == (res > 0)
	}

	for k, v := range s.mem {
		if v != nil && isKeyOK(k) {
			memList = append(memList, KeyValue{
				Key:   []byte(k),
				Value: v,
			})
		}
	}
	sort.Slice(memList, func(i, j int) bool {
		return less(memList[i].Key, memList[j].Key)
	})
	for _, kv := range memList {
		if !f(kv.Key, kv.Value) {
			break
		}
	}
}

// Batch implements the Batch interface and returns a compatible Batch.
func (s *MemoryStore) Batch() Batch {
	return newMemoryBatch()
}

// newMemoryBatch returns new memory batch.
func newMemoryBatch() *MemoryBatch {
	return &MemoryBatch{MemoryStore: *NewMemoryStore()}
}

// Close implements Store interface and clears up memory. Never returns an
// error.
func (s *MemoryStore) Close() error {
	s.mut.Lock()
	s.mem = nil
	s.mut.Unlock()
	return nil
}
