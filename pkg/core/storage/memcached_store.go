package storage

import (
	"bytes"
	"context"
	"sync"

	"github.com/google/btree"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// MemCachedStore is a wrapper around persistent store that caches all changes
// being made for them to be later flushed in one batch.
type MemCachedStore struct {
	MemoryStore

	// lowerTrie stores lower level MemCachedStore trie for cloned MemCachedStore,
	// which allows for much more efficient Persist.
	lowerTrie *btree.BTree
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

// Less implements btree.Item interface.
func (kv KeyValue) Less(other btree.Item) bool {
	return bytes.Compare(kv.Key, other.(KeyValue).Key) < 0
}

// NewMemCachedStore creates a new MemCachedStore object.
func NewMemCachedStore(lower Store) *MemCachedStore {
	return &MemCachedStore{
		MemoryStore: *NewMemoryStore(),
		ps:          lower,
	}
}

// NewClonedMemCachedStore creates a cloned MemCachedStore which shares the trie
// with another MemCachedStore (until you write into it).
func (s *MemCachedStore) Clone() *MemCachedStore {
	return &MemCachedStore{
		MemoryStore: MemoryStore{mem: *s.mem.Clone()}, // Shared COW trie.
		lowerTrie:   &s.mem,
		ps:          s.ps, // But the same PS.
	}
}

// Get implements the Store interface.
func (s *MemCachedStore) Get(key []byte) ([]byte, error) {
	s.mut.RLock()
	defer s.mut.RUnlock()
	itm := s.mem.Get(KeyValue{Key: key})
	if itm != nil {
		kv := itm.(KeyValue)
		if kv.Value == nil {
			return nil, ErrKeyNotFound
		}
		return kv.Value, nil
	}
	return s.ps.Get(key)
}

// GetBatch returns currently accumulated changeset.
func (s *MemCachedStore) GetBatch() *MemBatch {
	s.mut.RLock()
	defer s.mut.RUnlock()

	var b MemBatch

	b.Put = make([]KeyValueExists, 0, s.mem.Len())
	b.Deleted = make([]KeyValueExists, 0)
	s.mem.Ascend(func(i btree.Item) bool {
		kv := i.(KeyValue)
		_, err := s.ps.Get(kv.Key)
		if kv.Value == nil {
			b.Deleted = append(b.Deleted, KeyValueExists{KeyValue: kv, Exists: err == nil})
		} else {
			b.Put = append(b.Put, KeyValueExists{KeyValue: kv, Exists: err == nil})
		}
		return true
	})
	return &b
}

// Seek implements the Store interface.
func (s *MemCachedStore) Seek(rng SeekRange, f func(k, v []byte) bool) {
	s.seek(context.Background(), rng, false, f)
}

// SeekAsync returns non-buffered channel with matching KeyValue pairs. Key and
// value slices may not be copied and may be modified. SeekAsync can guarantee
// that key-value items are sorted by key in ascending way.
func (s *MemCachedStore) SeekAsync(ctx context.Context, rng SeekRange, cutPrefix bool) chan KeyValue {
	res := make(chan KeyValue)
	go func() {
		s.seek(ctx, rng, cutPrefix, func(k, v []byte) bool {
			res <- KeyValue{
				Key:   k,
				Value: v,
			}
			return true // always continue, we have context for early stop.
		})
		close(res)
	}()

	return res
}

// seek is internal representations of Seek* capable of seeking for the given key
// and supporting early stop using provided context. `cutPrefix` denotes whether provided
// key needs to be cut off the resulting keys. `rng` specifies prefix items must match
// and point to start seeking from. Backwards seeking from some point is supported
// with corresponding `rng` field set.
func (s *MemCachedStore) seek(ctx context.Context, rng SeekRange, cutPrefix bool, f func(k, v []byte) bool) {
	lPrefix := len(rng.Prefix)

	s.mut.RLock()
	var memRes = s.getSeekPairs(rng)
	ps := s.ps
	s.mut.RUnlock()

	less := func(k1, k2 []byte) bool {
		res := bytes.Compare(k1, k2)
		return res != 0 && rng.Backwards == (res > 0)
	}

	var (
		done    bool
		iMem    int
		kvMem   KeyValue
		haveMem bool
	)
	if iMem < len(memRes) {
		kvMem = memRes[iMem]
		haveMem = true
		iMem++
	}
	// Merge results of seek operations in ascending order. It returns whether iterating
	// should be continued.
	mergeFunc := func(k, v []byte) bool {
		if done {
			return false
		}
		kvPs := KeyValue{
			Key:   slice.Copy(k),
			Value: slice.Copy(v),
		}
		for {
			select {
			case <-ctx.Done():
				done = true
				return false
			default:
				var isMem = haveMem && less(kvMem.Key, kvPs.Key)
				if isMem {
					if kvMem.Value != nil {
						if cutPrefix {
							kvMem.Key = kvMem.Key[lPrefix:]
						}
						if !f(kvMem.Key, kvMem.Value) {
							done = true
							return false
						}
					}
					if iMem < len(memRes) {
						kvMem = memRes[iMem]
						haveMem = true
						iMem++
					} else {
						haveMem = false
					}
				} else {
					if !bytes.Equal(kvMem.Key, kvPs.Key) {
						if cutPrefix {
							kvPs.Key = kvPs.Key[lPrefix:]
						}
						if !f(kvPs.Key, kvPs.Value) {
							done = true
							return false
						}
					}
					return true
				}
			}
		}
	}
	ps.Seek(rng, mergeFunc)

	if !done && haveMem {
	loop:
		for i := iMem - 1; i < len(memRes); i++ {
			select {
			case <-ctx.Done():
				break loop
			default:
				kvMem = memRes[i]
				if kvMem.Value != nil {
					if cutPrefix {
						kvMem.Key = kvMem.Key[lPrefix:]
					}
					if !f(kvMem.Key, kvMem.Value) {
						break loop
					}
				}
			}
		}
	}
}

// Persist flushes all the MemoryStore contents into the (supposedly) persistent
// store ps. MemCachedStore remains accessible for the most part of this action
// (any new changes will be cached in memory).
func (s *MemCachedStore) Persist() (int, error) {
	return s.persist(false)
}

// PersistSync flushes all the MemoryStore contents into the (supposedly) persistent
// store ps. It's different from Persist in that it blocks MemCachedStore completely
// while flushing things from memory to persistent store.
func (s *MemCachedStore) PersistSync() (int, error) {
	return s.persist(true)
}

func (s *MemCachedStore) persist(isSync bool) (int, error) {
	var err error
	var keys int

	s.plock.Lock()
	defer s.plock.Unlock()
	s.mut.Lock()

	if s.lowerTrie != nil {
		keys = s.mem.Len() - s.lowerTrie.Len()
		*s.lowerTrie = s.mem
		s.mut.Unlock()
		return keys, nil
	}

	keys = s.mem.Len()
	if keys == 0 {
		s.mut.Unlock()
		return 0, nil
	}

	// tempstore technically copies current s in lower layer while real s
	// starts using fresh new maps. This tempstore is only known here and
	// nothing ever changes it, therefore accesses to it (reads) can go
	// unprotected while writes are handled by s proper.
	var tempstore = &MemCachedStore{MemoryStore: MemoryStore{mem: s.mem}, ps: s.ps}
	s.ps = tempstore
	s.mem = *btree.New(8)
	if !isSync {
		s.mut.Unlock()
	}

	err = tempstore.ps.PutChangeSet(&tempstore.mem)

	if !isSync {
		s.mut.Lock()
	}
	if err == nil {
		// tempstore.mem and tempstore.del are completely flushed now
		// to tempstore.ps, so all KV pairs are the same and this
		// substitution has no visible effects.
		s.ps = tempstore.ps
	} else {
		// We're toast. We'll try to still keep proper state, but OOM
		// killer will get to us eventually.
		s.mem.Ascend(func(i btree.Item) bool {
			tempstore.put(i.(KeyValue))
			return true
		})
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
