package storage

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	// layers holds in-memory changesets that are either being persisted to ps
	// or have already been persisted to ps while older read views remain open.
	// Ordered oldest to newest; protected by mut. Backing arrays and individual
	// layers are immutable once published.
	layers []*memLayer
	// persistGen is a monotonically increasing counter incremented on each
	// persist attempt. Protected by mut.
	persistGen uint64
	// persisting is true while persist is executing ps.PutChangeSet.
	// Protected by mut.
	persisting bool
	// viewGens tracks active views opened via BeginView by their minimum
	// required layer generation (minGen -> count). Protected by mut.
	viewGens map[uint64]int
	// View holds an opened read-only transaction to the persistent Store in
	// case if created by the caller. It's the caller's responsibility to close
	// the View once the current instance of MemCachedStore is released.
	View IView
	// closeView denotes whether the attached View should be closed on
	// finalisation of this instance of MemCachedStore.
	closeView bool
}

type memLayer struct {
	mem  map[string][]byte
	stor map[string][]byte
	gen  uint64
}

func (l *memLayer) chooseMap(key []byte) map[string][]byte {
	switch KeyPrefix(key[0]) {
	case STStorage, STTempStorage:
		return l.stor
	default:
		return l.mem
	}
}

type memCachedView struct {
	s      *MemCachedStore
	lower  IView
	minGen uint64
	closed atomic.Bool
}

// Close implements the IView interface.
func (v *memCachedView) Close() error {
	if !v.closed.CompareAndSwap(false, true) {
		return nil
	}
	err := v.lower.Close()
	v.s.releaseView(v.minGen)
	return err
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

// Finalize releases RO DB view occupied by the current private instance of
// MemCachedStore.
func (s *MemCachedStore) Finalize() error {
	if s.closeView {
		err := s.View.Close()
		s.View = nil
		s.closeView = false
		if err != nil {
			return fmt.Errorf("failed to close DB view: %w", err)
		}
	}
	return nil
}

// NewPrivateMemCachedStore creates a new private (unlocked) MemCachedStore object.
// Private cached stores are closed after Persist. If lower is a MemCachedStore
// with an opened view, the view is shared with the new store (but is not owned
// by it).
func NewPrivateMemCachedStore(lower Store) *MemCachedStore {
	var view IView
	if cache, ok := lower.(*MemCachedStore); ok {
		view = cache.View
	}
	return &MemCachedStore{
		MemoryStore: *NewMemoryStore(),
		private:     true,
		ps:          lower,
		View:        view,
	}
}

// NewPrivateMemCachedStoreWithView is similar to NewPrivateMemCachedStore, but
// it opens a new read-only view of the lower persistent Store that is used for
// all read operations. The view must be released via Finalize.
func NewPrivateMemCachedStoreWithView(lower Store) *MemCachedStore {
	view, err := lower.BeginView()
	if err != nil {
		panic(fmt.Errorf("failed to initialize read transaction to lower persistent storage: %w", err))
	}
	s := NewPrivateMemCachedStore(lower)
	s.View = view
	s.closeView = true
	return s
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

func (s *MemCachedStore) acquireViewGen() uint64 {
	s.mut.Lock()
	defer s.mut.Unlock()
	minGen := s.persistGen + 1
	if s.persisting {
		minGen = s.persistGen
	}
	if s.viewGens == nil {
		s.viewGens = make(map[uint64]int)
	}
	s.viewGens[minGen]++
	return minGen
}

func (s *MemCachedStore) releaseView(minGen uint64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	if c := s.viewGens[minGen]; c <= 1 {
		delete(s.viewGens, minGen)
	} else {
		s.viewGens[minGen] = c - 1
	}
	s.pruneLayersLocked()
}

// pruneLayersLocked drops persisted layers that no active view can still need.
// Caller must hold s.mut.Lock().
func (s *MemCachedStore) pruneLayersLocked() {
	if len(s.layers) == 0 {
		return
	}
	if len(s.viewGens) == 0 {
		if s.persisting {
			if len(s.layers) > 1 {
				s.layers = []*memLayer{s.layers[len(s.layers)-1]}
			}
		} else {
			s.layers = nil
		}
		return
	}
	var minActiveGen uint64 = math.MaxUint64
	for g := range s.viewGens {
		if g < minActiveGen {
			minActiveGen = g
		}
	}
	cut := 0
	limit := len(s.layers)
	if s.persisting {
		limit--
	}
	for cut < limit && s.layers[cut].gen < minActiveGen {
		cut++
	}
	if cut == 0 {
		return
	}
	if cut == len(s.layers) {
		s.layers = nil
		return
	}
	s.layers = slices.Clone(s.layers[cut:])
}

func (s *MemCachedStore) unwrapView(view IView) IView {
	if v, ok := view.(*memCachedView); ok && v.s == s {
		return v.lower
	}
	return view
}

// BeginView implements the Store interface.
func (s *MemCachedStore) BeginView() (IView, error) {
	if s.private {
		return s.ps.BeginView()
	}
	minGen := s.acquireViewGen()
	lower, err := s.ps.BeginView()
	if err != nil {
		s.releaseView(minGen)
		return nil, err
	}
	return &memCachedView{
		s:      s,
		lower:  lower,
		minGen: minGen,
	}, nil
}

// Get offers get functionality over underlying DB.
func (s *MemCachedStore) Get(key []byte) ([]byte, error) {
	if s.View != nil {
		return s.GetWithView(s.View, key)
	}
	s.rlock()
	val, ok := s.getMemory(key)
	s.runlock()
	if ok {
		if val == nil {
			return nil, ErrKeyNotFound
		}
		return val, nil
	}
	if s.private {
		if lower, ok := s.ps.(*MemCachedStore); ok {
			return lower.Get(key)
		}
	}
	view, err := s.BeginView()
	if err != nil {
		return nil, err
	}
	defer view.Close()
	return s.GetWithView(view, key)
}

// GetWithView implements the Store interface. It is not designated for the
// external usage. Use Get instead.
func (s *MemCachedStore) GetWithView(view IView, key []byte) ([]byte, error) {
	s.rlock()
	val, ok := s.getMemory(key)
	s.runlock()
	if ok {
		if val == nil {
			return nil, ErrKeyNotFound
		}
		return val, nil
	}
	return s.ps.GetWithView(s.unwrapView(view), key)
}

// getMemory looks up key in s's own maps and retained persisted layers.
// Caller must hold at least s.rlock().
func (s *MemCachedStore) getMemory(key []byte) ([]byte, bool) {
	if val, ok := s.chooseMap(key)[string(key)]; ok {
		return val, true
	}
	return s.getFromLayers(key)
}

// getFromLayers looks up key in retained layers from newest to oldest.
// Caller must hold at least s.rlock().
func (s *MemCachedStore) getFromLayers(key []byte) ([]byte, bool) {
	for _, layer := range slices.Backward(s.layers) {
		if val, ok := layer.chooseMap(key)[string(key)]; ok {
			return val, true
		}
	}
	return nil, false
}

// Put puts new KV pair into the store.
func (s *MemCachedStore) Put(key, value []byte) {
	newKey := string(key)
	vcopy := bytes.Clone(value)
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
	view := s.View
	if view == nil {
		var err error
		view, err = s.BeginView()
		if err != nil {
			panic(err)
		}
		defer view.Close()
	}
	lowerView := s.unwrapView(view)

	s.rlock()
	defer s.runlock()
	var b MemBatch

	b.Put = make([]KeyValueExists, 0, len(s.mem)+len(s.stor))
	b.Deleted = make([]KeyValueExists, 0)
	for _, m := range []map[string][]byte{s.mem, s.stor} {
		for k, v := range m {
			key := []byte(k)
			var exists bool
			if lv, ok := s.getFromLayers(key); ok {
				exists = lv != nil
			} else {
				_, err := s.ps.GetWithView(lowerView, key)
				exists = err == nil
			}
			if v == nil {
				b.Deleted = append(b.Deleted, KeyValueExists{KeyValue: KeyValue{Key: key}, Exists: exists})
			} else {
				b.Put = append(b.Put, KeyValueExists{KeyValue: KeyValue{Key: key, Value: v}, Exists: exists})
			}
		}
	}
	return &b
}

// PutChangeSet implements the Store interface. Never returns an error.
func (s *MemCachedStore) PutChangeSet(puts map[string][]byte, stores map[string][]byte) error {
	s.lock()
	s.putChangeSet(puts, stores)
	s.unlock()
	return nil
}

// Seek offers the DB seek functionality. See Store comments for behaviour details.
func (s *MemCachedStore) Seek(rng SeekRange, cont func(k, v []byte) bool) {
	view := s.View
	if view == nil && rng.SearchDepth != 1 {
		var err error
		view, err = s.BeginView()
		if err != nil {
			panic(err)
		}
		defer view.Close()
	}
	s.SeekWithView(view, rng, cont)
}

// SeekWithView implements the Store interface. It is not designated for the
// external usage. Use Seek instead.
func (s *MemCachedStore) SeekWithView(view IView, rng SeekRange, cont func(k, v []byte) bool) {
	memRes := s.prepareSeekMemSnapshot(rng)
	performSeek(context.Background(), s.unwrapView(view), s.ps, memRes, rng, false, cont)
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
	var (
		view      = s.View
		closeView bool
		err       error
	)
	if view == nil && rng.SearchDepth != 1 {
		view, err = s.BeginView()
		if err != nil {
			panic(err)
		}
		closeView = true
	}
	memRes := s.prepareSeekMemSnapshot(rng)
	lowerView := s.unwrapView(view)
	go func() {
		if closeView {
			defer view.Close()
		}
		performSeek(ctx, lowerView, s.ps, memRes, rng, cutPrefix, func(k, v []byte) bool {
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

// prepareSeekMemSnapshot prepares memory store snapshot of `stor`/`mem` and any
// retained persisted layers in order not to hold the lock over MemCachedStore
// throughout the whole Seek operation.
// The results of prepareSeekMemSnapshot can be safely used as performSeek arguments.
func (s *MemCachedStore) prepareSeekMemSnapshot(rng SeekRange) []KeyValueExists {
	var memRes []KeyValueExists
	sPrefix := string(rng.Prefix)
	lPrefix := len(sPrefix)
	sStart := string(rng.Start)
	isKeyOK := func(key string) bool {
		return strings.HasPrefix(key, sPrefix) && (rng.Start == nil || cmp.Compare(key[lPrefix:], sStart) >= 0)
	}
	if rng.Backwards {
		isKeyOK = func(key string) bool {
			return strings.HasPrefix(key, sPrefix) && (rng.Start == nil || cmp.Compare(key[lPrefix:], sStart) <= 0)
		}
	}
	s.rlock()
	m := s.chooseMap(rng.Prefix)
	if len(s.layers) == 0 || rng.SearchDepth == 1 {
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
	} else {
		seen := make(map[string]struct{}, len(m))
		addMap := func(mm map[string][]byte) {
			for k, v := range mm {
				if !isKeyOK(k) {
					continue
				}
				if _, ok := seen[k]; ok {
					continue
				}
				seen[k] = struct{}{}
				memRes = append(memRes, KeyValueExists{
					KeyValue: KeyValue{
						Key:   []byte(k),
						Value: v,
					},
					Exists: v != nil,
				})
			}
		}
		addMap(m)
		for _, layer := range slices.Backward(s.layers) {
			addMap(layer.chooseMap(rng.Prefix))
		}
	}
	s.runlock()
	return memRes
}

// performSeek is internal representations of Seek* capable of seeking for the given key
// and supporting early stop using provided context. `ps` is a captured underlying
// persistent storage. `memRes` is a snapshot of suitable cached items prepared
// by prepareSeekMemSnapshot.
//
// `cutPrefix` denotes whether provided key needs to be cut off the resulting keys.
// `rng` specifies prefix items must match and point to start seeking from. Backwards
// seeking from some point is supported with corresponding `rng` field set.
func performSeek(ctx context.Context, view IView, ps Store, memRes []KeyValueExists, rng SeekRange, cutPrefix bool, cont func(k, v []byte) bool) {
	lPrefix := len(string(rng.Prefix))
	var cmpFunc = getCmpFunc(rng.Backwards)

	// Sort memRes items for further comparison with ps items.
	slices.SortFunc(memRes, func(a, b KeyValueExists) int {
		return cmpFunc(a.Key, b.Key)
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
			Key:   bytes.Clone(k),
			Value: bytes.Clone(v),
		}
		for {
			select {
			case <-ctx.Done():
				done = true
				return false
			default:
				var isMem = haveMem && cmpFunc(kvMem.Key, kvPs.Key) < 0
				if isMem {
					if kvMem.Exists {
						if cutPrefix {
							kvMem.Key = kvMem.Key[lPrefix:]
						}
						if !cont(kvMem.Key, kvMem.Value) {
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
						if !cont(kvPs.Key, kvPs.Value) {
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
		ps.SeekWithView(view, rng, mergeFunc)
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
					if !cont(kvMem.Key, kvMem.Value) {
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

// PersistPrivate flushes the content of every private into MemCachedStore s
// supposedly that s is an underlying persistent store of every private.
// MemCachedStore remains accessible for the most part of this action
// (any new changes will be cached in memory) as far as every private.
// However, PersistPrivate relies on fact that every private is a private
// wrapper around s and every private will be disposed after persist, so further
// usages of private is no-op.
func (s *MemCachedStore) PersistPrivate(private ...*MemCachedStore) int {
	var keys int
	for i, p := range private {
		if !p.private {
			panic(fmt.Sprintf("memcached store %d is not private", i)) // program bug
		}
		if p.ps.(*MemCachedStore) != s {
			panic(fmt.Sprintf("persistent layer mismatch for memcached store %d", i)) // program bug
		}
		keys += len(p.mem) + len(p.stor)
	}
	// TODO: move under lock?
	for _, p := range private {
		_ = p.Finalize() // TODO: err
	}
	if keys == 0 {
		return 0
	}
	s.lock()
	for _, p := range private {
		s.putChangeSet(p.mem, p.stor)
		p.mem = nil
		p.stor = nil
	}
	s.unlock()

	return keys
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

	// Freeze current maps into an immutable layer while s starts using fresh
	// new maps. The layer stays visible to readers during PutChangeSet and
	// remains retained afterward until every view that may still depend on it
	// is closed.
	s.persistGen++
	layer := &memLayer{
		mem:  s.mem,
		stor: s.stor,
		gen:  s.persistGen,
	}
	nextLayers := make([]*memLayer, len(s.layers)+1)
	copy(nextLayers, s.layers)
	nextLayers[len(s.layers)] = layer
	s.layers = nextLayers
	s.persisting = true
	s.mem = make(map[string][]byte, len(s.mem))
	s.stor = make(map[string][]byte, len(s.stor))
	if !isSync {
		s.mut.Unlock()
	}
	err = s.ps.PutChangeSet(layer.mem, layer.stor)

	if !isSync {
		s.mut.Lock()
	}
	s.persisting = false
	if err == nil {
		// layer.mem and layer.stor are completely flushed now to s.ps; drop
		// any layers that no active read view can still need.
		s.pruneLayersLocked()
	} else {
		// We're toast. We'll try to still keep proper state, but OOM
		// killer will get to us eventually.
		mergedMem := make(map[string][]byte, len(layer.mem)+len(s.mem))
		maps.Copy(mergedMem, layer.mem)
		maps.Copy(mergedMem, s.mem)
		mergedStor := make(map[string][]byte, len(layer.stor)+len(s.stor))
		maps.Copy(mergedStor, layer.stor)
		maps.Copy(mergedStor, s.stor)
		s.mem = mergedMem
		s.stor = mergedStor
		if len(s.layers) == 1 {
			s.layers = nil
		} else {
			s.layers = slices.Clone(s.layers[:len(s.layers)-1])
		}
		s.pruneLayersLocked()
	}
	s.mut.Unlock()
	return keys, err
}

// Close implements Store interface, clears up memory and closes the lower layer
// Store.
func (s *MemCachedStore) Close() error {
	// It's always successful.
	_ = s.MemoryStore.Close()
	s.lock()
	s.layers = nil
	s.unlock()
	return s.ps.Close()
}
