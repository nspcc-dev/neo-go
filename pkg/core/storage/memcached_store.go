package storage

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

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
	// Create memory store `mem` and `del` snapshot not to hold the lock.
	var memRes []KeyValueExists
	sPrefix := string(rng.Prefix)
	lPrefix := len(sPrefix)
	sStart := string(rng.Start)
	lStart := len(sStart)
	isKeyOK := func(key string) bool {
		return strings.HasPrefix(key, sPrefix) && (lStart == 0 || strings.Compare(key[lPrefix:], sStart) >= 0)
	}
	if rng.Backwards {
		isKeyOK = func(key string) bool {
			return strings.HasPrefix(key, sPrefix) && (lStart == 0 || strings.Compare(key[lPrefix:], sStart) <= 0)
		}
	}
	s.mut.RLock()
	for k, v := range s.MemoryStore.mem {
		if isKeyOK(k) {
			memRes = append(memRes, KeyValueExists{
				KeyValue: KeyValue{
					Key:   []byte(k),
					Value: v,
				},
				Exists: true,
			})
		}
	}
	for k := range s.MemoryStore.del {
		if isKeyOK(k) {
			memRes = append(memRes, KeyValueExists{
				KeyValue: KeyValue{
					Key: []byte(k),
				},
			})
		}
	}
	ps := s.ps
	s.mut.RUnlock()

	less := func(k1, k2 []byte) bool {
		res := bytes.Compare(k1, k2)
		return res != 0 && rng.Backwards == (res > 0)
	}
	// Sort memRes items for further comparison with ps items.
	sort.Slice(memRes, func(i, j int) bool {
		return less(memRes[i].Key, memRes[j].Key)
	})

	var (
		done    bool
		iMem    int
		kvMem   KeyValueExists
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
					if kvMem.Exists {
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
				if kvMem.Exists {
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
	s.mem = make(map[string][]byte, len(s.mem))
	s.del = make(map[string]bool, len(s.del))
	if !isSync {
		s.mut.Unlock()
	}

	err = tempstore.ps.PutChangeSet(tempstore.mem, tempstore.del)

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
