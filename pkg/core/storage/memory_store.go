package storage

import (
	"encoding/hex"
)

// MemoryStore is an in-memory implementation of a Store, mainly
// used for testing. Do not use MemoryStore in production.
type MemoryStore struct {
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
		mem: make(map[string][]byte),
	}
}

// Get implements the Store interface.
func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	if val, ok := s.mem[makeKey(key)]; ok {
		return val, nil
	}
	return nil, ErrKeyNotFound
}

// Put implementes the Store interface.
func (s *MemoryStore) Put(key, value []byte) error {
	s.mem[makeKey(key)] = value
	return nil
}

// PutBatch implementes the Store interface.
func (s *MemoryStore) PutBatch(batch Batch) error {
	b := batch.(*MemoryBatch)
	for k, v := range b.m {
		s.Put(*k, v)
	}
	return nil
}

// Seek implementes the Store interface.
func (s *MemoryStore) Seek(key []byte, f func(k, v []byte)) {
}

// Batch implements the Batch interface and returns a compatible Batch.
func (s *MemoryStore) Batch() Batch {
	return &MemoryBatch{
		m: make(map[*[]byte][]byte),
	}
}

func makeKey(k []byte) string {
	return hex.EncodeToString(k)
}
