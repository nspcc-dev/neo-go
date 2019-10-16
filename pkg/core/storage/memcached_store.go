package storage

// MemCachedStore is a wrapper around persistent store that caches all changes
// being made for them to be later flushed in one batch.
type MemCachedStore struct {
	MemoryStore

	// Persistent Store.
	ps Store
}

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

// Seek implements the Store interface.
func (s *MemCachedStore) Seek(key []byte, f func(k, v []byte)) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	s.MemoryStore.Seek(key, f)
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
	s.mut.Lock()
	defer s.mut.Unlock()
	batch := s.ps.Batch()
	keys, dkeys := 0, 0
	for k, v := range s.mem {
		batch.Put([]byte(k), v)
		keys++
	}
	for k := range s.del {
		batch.Delete([]byte(k))
		dkeys++
	}
	var err error
	if keys != 0 || dkeys != 0 {
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
