package storage

import (
	"encoding/hex"
	"strings"
	"sync"
)

// MemoryStore is an in-memory implementation of a Store, mainly
// used for testing. Do not use MemoryStore in production.
type MemoryStore struct {
	*sync.RWMutex
	mem map[string][]byte
}

// MemoryBatch a in-memory batch compatible with MemoryStore.
type MemoryBatch struct {
	m map[*[]byte][]byte
}

// Put implements the Batch interface.
func (b *MemoryBatch) Put(k, v []byte) {
	key := &k
	b.m[key] = v
}

// Len implements the Batch interface.
func (b *MemoryBatch) Len() int {
	return len(b.m)
}

// NewMemoryStore creates a new MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		RWMutex: new(sync.RWMutex),
		mem:     make(map[string][]byte),
	}
}

// Get implements the Store interface.
func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	s.RLock()
	defer s.RUnlock()
	if val, ok := s.mem[makeKey(key)]; ok {
		return val, nil
	}
	return nil, ErrKeyNotFound
}

// Put implements the Store interface.
func (s *MemoryStore) Put(key, value []byte) error {
	s.Lock()
	s.mem[makeKey(key)] = value
	s.Unlock()
	return nil
}

// PutBatch implements the Store interface.
func (s *MemoryStore) PutBatch(batch Batch) error {
	b := batch.(*MemoryBatch)
	for k, v := range b.m {
		if err := s.Put(*k, v); err != nil {
			return err
		}
	}
	return nil
}

// Seek implements the Store interface.
func (s *MemoryStore) Seek(key []byte, f func(k, v []byte)) {
	for k, v := range s.mem {
		if strings.Contains(k, hex.EncodeToString(key)) {
			decodeString, _ := hex.DecodeString(k)
			f(decodeString, v)
		}
	}
}

// Batch implements the Batch interface and returns a compatible Batch.
func (s *MemoryStore) Batch() Batch {
	return &MemoryBatch{
		m: make(map[*[]byte][]byte),
	}
}

// Close implements Store interface and clears up memory.
func (s *MemoryStore) Close() error {
	s.Lock()
	s.mem = nil
	s.Unlock()
	return nil
}

func makeKey(k []byte) string {
	return hex.EncodeToString(k)
}
