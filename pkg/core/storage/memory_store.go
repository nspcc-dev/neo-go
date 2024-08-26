package storage

import (
	"bytes"
	"slices"
	"strings"
	"sync"
)

// MemoryStore is an in-memory implementation of a Store, mainly
// used for testing. Do not use MemoryStore in production.
type MemoryStore struct {
	mut  sync.RWMutex
	mem  map[string][]byte
	stor map[string][]byte
}

// NewMemoryStore creates a new MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		mem:  make(map[string][]byte),
		stor: make(map[string][]byte),
	}
}

// Get implements the Store interface.
func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	m := s.chooseMap(key)
	if val, ok := m[string(key)]; ok && val != nil {
		return val, nil
	}
	return nil, ErrKeyNotFound
}

func (s *MemoryStore) chooseMap(key []byte) map[string][]byte {
	switch KeyPrefix(key[0]) {
	case STStorage, STTempStorage:
		return s.stor
	default:
		return s.mem
	}
}

// put puts a key-value pair into the store, it's supposed to be called
// with mutex locked.
func put(m map[string][]byte, key string, value []byte) {
	m[key] = value
}

// PutChangeSet implements the Store interface. Never returns an error.
func (s *MemoryStore) PutChangeSet(puts map[string][]byte, stores map[string][]byte) error {
	s.mut.Lock()
	s.putChangeSet(puts, stores)
	s.mut.Unlock()
	return nil
}

func (s *MemoryStore) putChangeSet(puts map[string][]byte, stores map[string][]byte) {
	for k := range puts {
		put(s.mem, k, puts[k])
	}
	for k := range stores {
		put(s.stor, k, stores[k])
	}
}

// Seek implements the Store interface.
func (s *MemoryStore) Seek(rng SeekRange, f func(k, v []byte) bool) {
	s.seek(rng, f, s.mut.RLock, s.mut.RUnlock)
}

// SeekGC implements the Store interface.
func (s *MemoryStore) SeekGC(rng SeekRange, keep func(k, v []byte) bool) error {
	noop := func() {}
	// Keep RW lock for the whole Seek time, state must be consistent across whole
	// operation and we call delete in the handler.
	s.mut.Lock()
	// We still need to perform normal seek, some GC operations can be
	// sensitive to the order of KV pairs.
	s.seek(rng, func(k, v []byte) bool {
		if !keep(k, v) {
			delete(s.chooseMap(k), string(k))
		}
		return true
	}, noop, noop)
	s.mut.Unlock()
	return nil
}

// seek is an internal unlocked implementation of Seek. `start` denotes whether
// seeking starting from the provided prefix should be performed. Backwards
// seeking from some point is supported with corresponding SeekRange field set.
func (s *MemoryStore) seek(rng SeekRange, f func(k, v []byte) bool, lock func(), unlock func()) {
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
	var cmpFunc = getCmpFunc(rng.Backwards)

	lock()
	m := s.chooseMap(rng.Prefix)
	for k, v := range m {
		if v != nil && isKeyOK(k) {
			memList = append(memList, KeyValue{
				Key:   []byte(k),
				Value: v,
			})
		}
	}
	unlock()
	slices.SortFunc(memList, func(a, b KeyValue) int {
		return cmpFunc(a.Key, b.Key)
	})
	for _, kv := range memList {
		if !f(kv.Key, kv.Value) {
			break
		}
	}
}

func getCmpFunc(backwards bool) func(a, b []byte) int {
	if !backwards {
		return bytes.Compare
	}
	return func(a, b []byte) int { return -bytes.Compare(a, b) }
}

// Close implements Store interface and clears up memory. Never returns an
// error.
func (s *MemoryStore) Close() error {
	s.mut.Lock()
	s.mem = nil
	s.stor = nil
	s.mut.Unlock()
	return nil
}
