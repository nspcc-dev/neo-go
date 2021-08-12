package storage

import (
	"strings"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// MemoryStore is an in-memory implementation of a Store, mainly
// used for testing. Do not use MemoryStore in production.
type MemoryStore struct {
	mut sync.RWMutex
	mem map[string][]byte
	// A map, not a slice, to avoid duplicates.
	del map[string]bool
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
		del: make(map[string]bool),
	}
}

// Get implements the Store interface.
func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	if val, ok := s.mem[string(key)]; ok {
		return val, nil
	}
	return nil, ErrKeyNotFound
}

// put puts a key-value pair into the store, it's supposed to be called
// with mutex locked.
func (s *MemoryStore) put(key string, value []byte) {
	s.mem[key] = value
	delete(s.del, key)
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
	s.del[key] = true
	delete(s.mem, key)
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
	return s.PutChangeSet(b.mem, b.del)
}

// PutChangeSet implements the Store interface. Never returns an error.
func (s *MemoryStore) PutChangeSet(puts map[string][]byte, dels map[string]bool) error {
	s.mut.Lock()
	for k := range puts {
		s.put(k, puts[k])
	}
	for k := range dels {
		s.drop(k)
	}
	s.mut.Unlock()
	return nil
}

// Seek implements the Store interface.
func (s *MemoryStore) Seek(key []byte, f func(k, v []byte)) {
	s.mut.RLock()
	s.seek(key, f)
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
	for k := range s.del {
		if strings.HasPrefix(k, sk) {
			f([]byte(k), nil)
		}
	}
}

// seek is an internal unlocked implementation of Seek.
func (s *MemoryStore) seek(key []byte, f func(k, v []byte)) {
	sk := string(key)
	for k, v := range s.mem {
		if strings.HasPrefix(k, sk) {
			f([]byte(k), v)
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
	s.del = nil
	s.mem = nil
	s.mut.Unlock()
	return nil
}
