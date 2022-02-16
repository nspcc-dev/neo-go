package storage

import (
	"bytes"
	"sort"
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
	for k := range puts {
		put(s.mem, k, puts[k])
	}
	for k := range stores {
		put(s.stor, k, stores[k])
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

// SeekGC implements the Store interface.
func (s *MemoryStore) SeekGC(rng SeekRange, keep func(k, v []byte) bool) error {
	s.mut.Lock()
	// We still need to perform normal seek, some GC operations can be
	// sensitive to the order of KV pairs.
	s.seek(rng, func(k, v []byte) bool {
		if !keep(k, v) {
			delete(s.chooseMap(k), string(k))
		}
		return true
	})
	s.mut.Unlock()
	return nil
}

// SeekAll is like seek but also iterates over deleted items.
func (s *MemoryStore) SeekAll(key []byte, f func(k, v []byte)) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	sk := string(key)
	m := s.chooseMap(key)
	for k, v := range m {
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

	m := s.chooseMap(rng.Prefix)
	for k, v := range m {
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

// Close implements Store interface and clears up memory. Never returns an
// error.
func (s *MemoryStore) Close() error {
	s.mut.Lock()
	s.mem = nil
	s.stor = nil
	s.mut.Unlock()
	return nil
}
