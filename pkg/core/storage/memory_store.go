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
	vcopy := make([]byte, len(v))
	copy(vcopy, v)
	kcopy := make([]byte, len(k))
	copy(kcopy, k)
	b.m[&kcopy] = vcopy
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

// Put implements the Store interface. Never returns an error.
func (s *MemoryStore) Put(key, value []byte) error {
	s.Lock()
	s.mem[makeKey(key)] = value
	s.Unlock()
	return nil
}

// PutBatch implements the Store interface. Never returns an error.
func (s *MemoryStore) PutBatch(batch Batch) error {
	b := batch.(*MemoryBatch)
	for k, v := range b.m {
		_ = s.Put(*k, v)
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
	return newMemoryBatch()
}

// newMemoryBatch returns new memory batch.
func newMemoryBatch() *MemoryBatch {
	return &MemoryBatch{
		m: make(map[*[]byte][]byte),
	}
}

// Persist flushes all the MemoryStore contents into the (supposedly) persistent
// store provided via parameter.
func (s *MemoryStore) Persist(ps Store) (int, error) {
	s.Lock()
	defer s.Unlock()
	batch := ps.Batch()
	keys := 0
	for k, v := range s.mem {
		kb, _ := hex.DecodeString(k)
		batch.Put(kb, v)
		keys++
	}
	var err error
	if keys != 0 {
		err = ps.PutBatch(batch)
	}
	if err == nil {
		s.mem = make(map[string][]byte)
	}
	return keys, err
}

// Close implements Store interface and clears up memory. Never returns an
// error.
func (s *MemoryStore) Close() error {
	s.Lock()
	s.mem = nil
	s.Unlock()
	return nil
}

func makeKey(k []byte) string {
	return hex.EncodeToString(k)
}
