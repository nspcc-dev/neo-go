package storage

// MemCachedStore is a wrapper around persistent store that caches all changes
// being made for them to be later flushed in one batch.
type MemCachedStore struct {
	MemoryStore

	// Persistent Store.
	ps Store
}

type (
	// KeyValue represents key-value pair.
	KeyValue struct {
		Key   []byte
		Value []byte

		Exists bool
	}

	// MemBatch represents a changeset to be persisted.
	MemBatch struct {
		Put     []KeyValue
		Deleted []KeyValue
	}
)

// NewMemCachedStore creates a new MemCachedStore object.
func NewMemCachedStore(lower Store) *MemCachedStore {
	return &MemCachedStore{
		MemoryStore: *NewMemoryStore(),
		ps:          lower,
	}
}

// Get implements the Store interface.
func (s *MemCachedStore) Get(key []byte) ([]byte, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	k := string(key)
	if val, ok := s.mem[k]; ok {
		return val, nil
	}
	if _, ok := s.del[k]; ok {
		return nil, ErrKeyNotFound
	}
	return s.ps.Get(key)
}

// GetBatch returns currently accumulated changeset.
func (s *MemCachedStore) GetBatch() *MemBatch {
	s.mut.RLock()
	defer s.mut.RUnlock()

	var b MemBatch

	b.Put = make([]KeyValue, 0, len(s.mem))
	for k, v := range s.mem {
		key := []byte(k)
		_, err := s.ps.Get(key)
		b.Put = append(b.Put, KeyValue{Key: key, Value: v, Exists: err == nil})
	}

	b.Deleted = make([]KeyValue, 0, len(s.del))
	for k := range s.del {
		key := []byte(k)
		_, err := s.ps.Get(key)
		b.Deleted = append(b.Deleted, KeyValue{Key: key, Exists: err == nil})
	}

	return &b
}

// Seek implements the Store interface.
func (s *MemCachedStore) Seek(key []byte, f func(k, v []byte)) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	s.MemoryStore.seek(key, f)
	s.ps.Seek(key, func(k, v []byte) {
		elem := string(k)
		// If it's in mem, we already called f() for it in MemoryStore.Seek().
		_, present := s.mem[elem]
		if !present {
			// If it's in del, we shouldn't be calling f() anyway.
			_, present = s.del[elem]
		}
		if !present {
			f(k, v)
		}
	})
}

// Persist flushes all the MemoryStore contents into the (supposedly) persistent
// store ps.
func (s *MemCachedStore) Persist() (int, error) {
	var err error
	var keys, dkeys int

	s.mut.Lock()
	defer s.mut.Unlock()

	keys = len(s.mem)
	dkeys = len(s.del)
	if keys == 0 && dkeys == 0 {
		return 0, nil
	}

	memStore, ok := s.ps.(*MemoryStore)
	if !ok {
		memCachedStore, ok := s.ps.(*MemCachedStore)
		if ok {
			memStore = &memCachedStore.MemoryStore
		}
	}
	if memStore != nil {
		memStore.mut.Lock()
		for k := range s.mem {
			memStore.put(k, s.mem[k])
		}
		for k := range s.del {
			memStore.drop(k)
		}
		memStore.mut.Unlock()
	} else {
		batch := s.ps.Batch()
		for k := range s.mem {
			batch.Put([]byte(k), s.mem[k])
		}
		for k := range s.del {
			batch.Delete([]byte(k))
		}
		err = s.ps.PutBatch(batch)
	}
	if err == nil {
		s.mem = make(map[string][]byte)
		s.del = make(map[string]bool)
	}
	return keys, err
}

// Close implements Store interface, clears up memory and closes the lower layer
// Store.
func (s *MemCachedStore) Close() error {
	// It's always successful.
	_ = s.MemoryStore.Close()
	return s.ps.Close()
}
