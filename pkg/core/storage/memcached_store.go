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

	private bool
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

// NewPrivateMemCachedStore creates a new private (unlocked) MemCachedStore object.
// Private cached stores are closed after Persist.
func NewPrivateMemCachedStore(lower Store) *MemCachedStore {
	return &MemCachedStore{
		MemoryStore: *NewMemoryStore(),
		private:     true,
		ps:          lower,
	}
}

// lock write-locks non-private store.
func (s *MemCachedStore) lock() {
	if !s.private {
		s.mut.Lock()
	}
}

// unlock unlocks non-private store.
func (s *MemCachedStore) unlock() {
	if !s.private {
		s.mut.Unlock()
	}
}

// rlock read-locks non-private store.
func (s *MemCachedStore) rlock() {
	if !s.private {
		s.mut.RLock()
	}
}

// runlock drops read lock for non-private stores.
func (s *MemCachedStore) runlock() {
	if !s.private {
		s.mut.RUnlock()
	}
}

// Get implements the Store interface.
func (s *MemCachedStore) Get(key []byte) ([]byte, error) {
	s.rlock()
	defer s.runlock()
	m := s.chooseMap(key)
	if val, ok := m[string(key)]; ok {
		if val == nil {
			return nil, ErrKeyNotFound
		}
		return val, nil
	}
	return s.ps.Get(key)
}

// Put puts new KV pair into the store.
func (s *MemCachedStore) Put(key, value []byte) {
	newKey := string(key)
	vcopy := slice.Copy(value)
	s.lock()
	put(s.chooseMap(key), newKey, vcopy)
	s.unlock()
}

// Delete drops KV pair from the store. Never returns an error.
func (s *MemCachedStore) Delete(key []byte) {
	newKey := string(key)
	s.lock()
	put(s.chooseMap(key), newKey, nil)
	s.unlock()
}

// GetBatch returns currently accumulated changeset.
func (s *MemCachedStore) GetBatch() *MemBatch {
	s.rlock()
	defer s.runlock()
	var b MemBatch

	b.Put = make([]KeyValueExists, 0, len(s.mem)+len(s.stor))
	b.Deleted = make([]KeyValueExists, 0)
	for _, m := range []map[string][]byte{s.mem, s.stor} {
		for k, v := range m {
			key := []byte(k)
			_, err := s.ps.Get(key)
			if v == nil {
				b.Deleted = append(b.Deleted, KeyValueExists{KeyValue: KeyValue{Key: key}, Exists: err == nil})
			} else {
				b.Put = append(b.Put, KeyValueExists{KeyValue: KeyValue{Key: key, Value: v}, Exists: err == nil})
			}
		}
	}
	return &b
}

// PutChangeSet implements the Store interface. Never returns an error.
func (s *MemCachedStore) PutChangeSet(puts map[string][]byte, stores map[string][]byte) error {
	s.lock()
	s.MemoryStore.putChangeSet(puts, stores)
	s.unlock()
	return nil
}

// Seek implements the Store interface.
func (s *MemCachedStore) Seek(rng SeekRange, f func(k, v []byte) bool) {
	ps, memRes := s.prepareSeekMemSnapshot(rng)
	performSeek(context.Background(), ps, memRes, rng, false, f)
}

// GetStorageChanges returns all current storage changes. It can only be done for private
// MemCachedStore.
func (s *MemCachedStore) GetStorageChanges() map[string][]byte {
	if !s.private {
		panic("GetStorageChanges called on shared MemCachedStore")
	}
	return s.stor
}

// SeekAsync returns non-buffered channel with matching KeyValue pairs. Key and
// value slices may not be copied and may be modified. SeekAsync can guarantee
// that key-value items are sorted by key in ascending way.
func (s *MemCachedStore) SeekAsync(ctx context.Context, rng SeekRange, cutPrefix bool) chan KeyValue {
	res := make(chan KeyValue)
	ps, memRes := s.prepareSeekMemSnapshot(rng)
	go func() {
		performSeek(ctx, ps, memRes, rng, cutPrefix, func(k, v []byte) bool {
			select {
			case <-ctx.Done():
				return false
			case res <- KeyValue{Key: k, Value: v}:
				return true
			}
		})
		close(res)
	}()

	return res
}

// prepareSeekMemSnapshot prepares memory store snapshot of `stor`/`mem` in order
// not to hold the lock over MemCachedStore throughout the whole Seek operation.
// The results of prepareSeekMemSnapshot can be safely used as performSeek arguments.
func (s *MemCachedStore) prepareSeekMemSnapshot(rng SeekRange) (Store, []KeyValueExists) {
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
	s.rlock()
	m := s.MemoryStore.chooseMap(rng.Prefix)
	for k, v := range m {
		if isKeyOK(k) {
			memRes = append(memRes, KeyValueExists{
				KeyValue: KeyValue{
					Key:   []byte(k),
					Value: v,
				},
				Exists: v != nil,
			})
		}
	}
	ps := s.ps
	s.runlock()
	return ps, memRes
}

// performSeek is internal representations of Seek* capable of seeking for the given key
// and supporting early stop using provided context. `ps` is a captured underlying
// persistent storage. `memRes` is a snapshot of suitable cached items prepared
// by prepareSeekMemSnapshot.
//
// `cutPrefix` denotes whether provided key needs to be cut off the resulting keys.
// `rng` specifies prefix items must match and point to start seeking from. Backwards
// seeking from some point is supported with corresponding `rng` field set.
func performSeek(ctx context.Context, ps Store, memRes []KeyValueExists, rng SeekRange, cutPrefix bool, f func(k, v []byte) bool) {
	lPrefix := len(string(rng.Prefix))
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
	if rng.SearchDepth == 0 || rng.SearchDepth > 1 {
		if rng.SearchDepth > 1 {
			rng.SearchDepth--
		}
		ps.Seek(rng, mergeFunc)
	}

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
	var keys int

	if s.private {
		keys = len(s.mem) + len(s.stor)
		if keys == 0 {
			return 0, nil
		}
		err = s.ps.PutChangeSet(s.mem, s.stor)
		if err != nil {
			return 0, err
		}
		s.mem = nil
		s.stor = nil
		return keys, nil
	}

	s.plock.Lock()
	defer s.plock.Unlock()
	s.mut.Lock()

	keys = len(s.mem) + len(s.stor)
	if keys == 0 {
		s.mut.Unlock()
		return 0, nil
	}

	// tempstore technically copies current s in lower layer while real s
	// starts using fresh new maps. This tempstore is only known here and
	// nothing ever changes it, therefore accesses to it (reads) can go
	// unprotected while writes are handled by s proper.
	var tempstore = &MemCachedStore{MemoryStore: MemoryStore{mem: s.mem, stor: s.stor}, ps: s.ps}
	s.ps = tempstore
	s.mem = make(map[string][]byte, len(s.mem))
	s.stor = make(map[string][]byte, len(s.stor))
	if !isSync {
		s.mut.Unlock()
	}
	err = tempstore.ps.PutChangeSet(tempstore.mem, tempstore.stor)

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
			put(tempstore.mem, k, s.mem[k])
		}
		for k := range s.stor {
			put(tempstore.stor, k, s.stor[k])
		}
		s.ps = tempstore.ps
		s.mem = tempstore.mem
		s.stor = tempstore.stor
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
