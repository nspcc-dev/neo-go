package storage

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// MemCachedStore is a wrapper around persistent store that caches all changes
// being made for them to be later flushed in one batch.
type MemCachedStore struct {
	MemoryStore

	nativeCacheLock sync.RWMutex
	nativeCache     map[int32]NativeContractCache

	private bool
	// plock protects Persist from double entrance.
	plock sync.Mutex
	// Persistent Store.
	ps Store
}

// NativeContractCache is an interface representing cache for a native contract.
// Cache can be copied to create a wrapper around current DAO layer. Wrapped cache
// can be persisted to the underlying DAO native cache.
type NativeContractCache interface {
	// Copy returns a copy of native cache item that can safely be changed within
	// the subsequent DAO operations.
	Copy() NativeContractCache
	// Persist persists changes from upper native cache wrapper to the underlying
	// native cache `ps`. The resulting up-to-date cache and an error are returned.
	Persist(ps NativeContractCache) (NativeContractCache, error)
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
	// Do not copy cache from ps; instead should create clear map: GetRWCache and
	// GetROCache will retrieve cache from the underlying nativeCache if requested.
	cache := make(map[int32]NativeContractCache)
	return &MemCachedStore{
		MemoryStore: *NewMemoryStore(),
		nativeCache: cache,
		ps:          lower,
	}
}

// NewPrivateMemCachedStore creates a new private (unlocked) MemCachedStore object.
// Private cached stores are closed after Persist.
func NewPrivateMemCachedStore(lower Store) *MemCachedStore {
	// Do not copy cache from ps; instead should create clear map: GetRWCache and
	// GetROCache will retrieve cache from the underlying nativeCache if requested.
	// The lowest underlying store MUST have its native cache initialized, otherwise
	// GetROCache and GetRWCache won't work properly.
	cache := make(map[int32]NativeContractCache)
	return &MemCachedStore{
		MemoryStore: *NewMemoryStore(),
		nativeCache: cache,
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
	s.seek(context.Background(), rng, false, f)
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
	go func() {
		s.seek(ctx, rng, cutPrefix, func(k, v []byte) bool {
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
		if cached, ok := s.ps.(*MemCachedStore); ok {
			for id, nativeCache := range s.nativeCache {
				updatedCache, err := nativeCache.Persist(cached.nativeCache[id])
				if err != nil {
					return 0, fmt.Errorf("failed to persist native cache changes for private MemCachedStore: %w", err)
				}
				cached.nativeCache[id] = updatedCache
			}
			s.nativeCache = nil
		}
		return keys, nil
	}

	s.plock.Lock()
	defer s.plock.Unlock()
	s.mut.Lock()
	s.nativeCacheLock.Lock()

	keys = len(s.mem) + len(s.stor)
	if keys == 0 {
		s.nativeCacheLock.Unlock()
		s.mut.Unlock()
		return 0, nil
	}

	// tempstore technically copies current s in lower layer while real s
	// starts using fresh new maps. This tempstore is only known here and
	// nothing ever changes it, therefore accesses to it (reads) can go
	// unprotected while writes are handled by s proper.
	var tempstore = &MemCachedStore{MemoryStore: MemoryStore{mem: s.mem, stor: s.stor}, ps: s.ps, nativeCache: s.nativeCache}
	s.ps = tempstore
	s.mem = make(map[string][]byte, len(s.mem))
	s.stor = make(map[string][]byte, len(s.stor))
	cached, isPSCached := tempstore.ps.(*MemCachedStore)
	if isPSCached {
		s.nativeCache = make(map[int32]NativeContractCache)
	}
	if !isSync {
		s.nativeCacheLock.Unlock()
		s.mut.Unlock()
	}
	if isPSCached {
		cached.nativeCacheLock.Lock()
		for id, nativeCache := range tempstore.nativeCache {
			updatedCache, err := nativeCache.Persist(cached.nativeCache[id])
			if err != nil {
				cached.nativeCacheLock.Unlock()
				return 0, fmt.Errorf("failed to persist native cache changes: %w", err)
			}
			cached.nativeCache[id] = updatedCache
		}
		cached.nativeCacheLock.Unlock()
	}
	err = tempstore.ps.PutChangeSet(tempstore.mem, tempstore.stor)

	if !isSync {
		s.mut.Lock()
		s.nativeCacheLock.Lock()
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
		if isPSCached {
			for id, nativeCache := range s.nativeCache {
				updatedCache, err := nativeCache.Persist(tempstore.nativeCache[id])
				if err != nil {
					return 0, fmt.Errorf("failed to persist native cache changes: %w", err)
				}
				tempstore.nativeCache[id] = updatedCache
			}
			s.nativeCache = tempstore.nativeCache
		}
		s.ps = tempstore.ps
		s.mem = tempstore.mem
		s.stor = tempstore.stor
	}
	s.nativeCacheLock.Unlock()
	s.mut.Unlock()
	return keys, err
}

// GetROCache returns native contact cache. The cache CAN NOT be modified by
// the caller. It's the caller's duty to keep it unmodified.
func (s *MemCachedStore) GetROCache(id int32) NativeContractCache {
	s.nativeCacheLock.RLock()
	defer s.nativeCacheLock.RUnlock()

	return s.getCache(id, true)
}

// GetRWCache returns native contact cache. The cache CAN BE safely modified
// by the caller.
func (s *MemCachedStore) GetRWCache(k int32) NativeContractCache {
	s.nativeCacheLock.Lock()
	defer s.nativeCacheLock.Unlock()

	return s.getCache(k, false)
}

func (s *MemCachedStore) getCache(k int32, ro bool) NativeContractCache {
	if itm, ok := s.nativeCache[k]; ok {
		// Don't need to create itm copy, because its value was already copied
		// the first time it was retrieved from loser ps.
		return itm
	}

	if cached, ok := s.ps.(*MemCachedStore); ok {
		if ro {
			return cached.GetROCache(k)
		}
		v := cached.GetRWCache(k)
		if v != nil {
			// Create a copy here in order not to modify the existing cache.
			cp := v.Copy()
			s.nativeCache[k] = cp
			return cp
		}
	}

	return nil
}

func (s *MemCachedStore) SetCache(k int32, v NativeContractCache) {
	s.nativeCacheLock.Lock()
	defer s.nativeCacheLock.Unlock()

	s.nativeCache[k] = v
}

// Close implements Store interface, clears up memory and closes the lower layer
// Store.
func (s *MemCachedStore) Close() error {
	// It's always successful.
	_ = s.MemoryStore.Close()
	return s.ps.Close()
}
