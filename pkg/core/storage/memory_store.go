package storage

import (
	"bytes"
	"sync"

	"github.com/google/btree"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// MemoryStore is an in-memory implementation of a Store, mainly
// used for testing. Do not use MemoryStore in production.
type MemoryStore struct {
	mut sync.RWMutex
	mem btree.BTree
}

// MemoryBatch is an in-memory batch compatible with MemoryStore.
type MemoryBatch struct {
	MemoryStore
}

// Put implements the Batch interface.
func (b *MemoryBatch) Put(k, v []byte) {
	b.MemoryStore.put(dupKV(k, v))
}

// Delete implements Batch interface.
func (b *MemoryBatch) Delete(k []byte) {
	b.MemoryStore.drop(slice.Copy(k))
}

// NewMemoryStore creates a new MemoryStore object.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		mem: *btree.New(2),
	}
}

// Get implements the Store interface.
func (s *MemoryStore) Get(key []byte) ([]byte, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	itm := s.mem.Get(KeyValue{Key: key})
	if itm != nil {
		kv := itm.(KeyValue)
		if kv.Value != nil {
			return kv.Value, nil
		}
	}
	return nil, ErrKeyNotFound
}

// put puts a key-value pair into the store, it's supposed to be called
// with mutex locked.
func (s *MemoryStore) put(kv KeyValue) {
	_ = s.mem.ReplaceOrInsert(kv)
}

// Put implements the Store interface. Never returns an error.
func (s *MemoryStore) Put(key, value []byte) error {
	kv := dupKV(key, value)
	s.mut.Lock()
	s.put(kv)
	s.mut.Unlock()
	return nil
}

// drop deletes a key-value pair from the store, it's supposed to be called
// with mutex locked.
func (s *MemoryStore) drop(key []byte) {
	s.put(KeyValue{Key: key})
}

// Delete implements Store interface. Never returns an error.
func (s *MemoryStore) Delete(key []byte) error {
	newKey := slice.Copy(key)
	s.mut.Lock()
	s.drop(newKey)
	s.mut.Unlock()
	return nil
}

// PutBatch implements the Store interface. Never returns an error.
func (s *MemoryStore) PutBatch(batch Batch) error {
	b := batch.(*MemoryBatch)
	return s.PutChangeSet(&b.mem)
}

// PutChangeSet implements the Store interface. Never returns an error.
func (s *MemoryStore) PutChangeSet(puts *btree.BTree) error {
	s.mut.Lock()
	puts.Ascend(func(i btree.Item) bool {
		s.put(i.(KeyValue))
		return true
	})
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
	s.mem.AscendGreaterOrEqual(KeyValue{Key: key}, func(i btree.Item) bool {
		kv := i.(KeyValue)
		if !bytes.HasPrefix(kv.Key, key) {
			return false
		}
		f(kv.Key, kv.Value)
		return true
	})
}

// getSeekPairs returns KV pairs for current Seek.
func (s *MemoryStore) getSeekPairs(rng SeekRange) []KeyValue {
	lPrefix := len(rng.Prefix)
	lStart := len(rng.Start)

	var pivot KeyValue
	pivot.Key = make([]byte, lPrefix+lStart, lPrefix+lStart+1)
	copy(pivot.Key, rng.Prefix)
	if lStart != 0 {
		copy(pivot.Key[lPrefix:], rng.Start)
	}

	var memList []KeyValue
	var appender = func(i btree.Item) bool {
		kv := i.(KeyValue)
		if !bytes.HasPrefix(kv.Key, rng.Prefix) {
			return false
		}
		memList = append(memList, kv)
		return true
	}

	if !rng.Backwards {
		s.mem.AscendGreaterOrEqual(pivot, appender)
	} else {
		if lStart != 0 {
			pivot.Key = append(pivot.Key, 0) // Right after the start key.
			s.mem.AscendRange(KeyValue{Key: rng.Prefix}, pivot, appender)
		} else {
			s.mem.AscendGreaterOrEqual(KeyValue{Key: rng.Prefix}, appender)
		}
		for i, j := 0, len(memList)-1; i <= j; i, j = i+1, j-1 {
			memList[i], memList[j] = memList[j], memList[i]
		}
	}
	return memList
}

// seek is an internal unlocked implementation of Seek. `start` denotes whether
// seeking starting from the provided prefix should be performed. Backwards
// seeking from some point is supported with corresponding SeekRange field set.
func (s *MemoryStore) seek(rng SeekRange, f func(k, v []byte) bool) {
	for _, kv := range s.getSeekPairs(rng) {
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
	s.mem.Clear(false)
	s.mut.Unlock()
	return nil
}

func dupKV(key []byte, value []byte) KeyValue {
	var res KeyValue

	s := make([]byte, len(key)+len(value))
	copy(s, key)
	res.Key = s[:len(key)]
	if value != nil {
		copy(s[len(key):], value)
		res.Value = s[len(key):]
	}
	return res
}
