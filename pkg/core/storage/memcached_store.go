package storage

import "sync"

// MemCachedStore is a wrapper around persistent store that caches all changes
// being made for them to be later flushed in one batch.
type MemCachedStore struct {
	MemoryStore

	// plock protects Persist from double entrance.
	plock sync.Mutex
	// Persistent Store.
	ps Store
}

type (
	// KeyValue represents key-value pair.
	KeyValue struct {
		Key   []byte
		Value []byte
	}

	// KeyValueExists represents key-value pair with indicator whether the item
	// exists in the persistent storage.
	KeyValueExists struct {
		KeyValue

		Exists bool
	}

	// MemBatch represents a changeset to be persisted.
	MemBatch struct {
		Put     []KeyValueExists
		Deleted []KeyValueExists
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

	b.Put = make([]KeyValueExists, 0, len(s.mem))
	for k, v := range s.mem {
		key := []byte(k)
		_, err := s.ps.Get(key)
		b.Put = append(b.Put, KeyValueExists{KeyValue: KeyValue{Key: key, Value: v}, Exists: err == nil})
	}

	b.Deleted = make([]KeyValueExists, 0, len(s.del))
	for k := range s.del {
		key := []byte(k)
		_, err := s.ps.Get(key)
		b.Deleted = append(b.Deleted, KeyValueExists{KeyValue: KeyValue{Key: key}, Exists: err == nil})
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

	s.plock.Lock()
	defer s.plock.Unlock()
	s.mut.Lock()

	keys = len(s.mem)
	dkeys = len(s.del)
	if keys == 0 && dkeys == 0 {
		s.mut.Unlock()
		return 0, nil
	}

	// tempstore technically copies current s in lower layer while real s
	// starts using fresh new maps. This tempstore is only known here and
	// nothing ever changes it, therefore accesses to it (reads) can go
	// unprotected while writes are handled by s proper.
	var tempstore = &MemCachedStore{MemoryStore: MemoryStore{mem: s.mem, del: s.del}, ps: s.ps}
	s.ps = tempstore
	s.mem = make(map[string][]byte)
	s.del = make(map[string]bool)
	s.mut.Unlock()

	err = tempstore.ps.PutChangeSet(tempstore.mem, tempstore.del)

	s.mut.Lock()
	if err == nil {
		// tempstore.mem and tempstore.del are completely flushed now
		// to tempstore.ps, so all KV pairs are the same and this
		// substitution has no visible effects.
		s.ps = tempstore.ps
	} else {
		// We're toast. We'll try to still keep proper state, but OOM
		// killer will get to us eventually.
		for k := range s.mem {
			tempstore.put(k, s.mem[k])
		}
		for k := range s.del {
			tempstore.drop(k)
		}
		s.ps = tempstore.ps
		s.mem = tempstore.mem
		s.del = tempstore.del
	}
	s.mut.Unlock()
	return keys, err
}

// Close implements Store interface, clears up memory and closes the lower layer
// Store.
func (s *MemCachedStore) Close() error {
	// It's always successful.
	_ = s.MemoryStore.Close()
	return s.ps.Close()
}
