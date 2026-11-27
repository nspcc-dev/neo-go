package storage_test

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/neotest/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemCachedPutGetDelete(t *testing.T) {
	ps := storage.NewMemoryStore()
	s := storage.NewMemCachedStore(ps)
	key := []byte("foo")
	value := []byte("bar")

	s.Put(key, value)

	require.Equal(t, 1, s.Len())
	result, err := s.Get(key)
	assert.Nil(t, err)
	require.Equal(t, value, result)

	s.Delete(key)

	require.Equal(t, 1, s.Len()) // deletion marker
	_, err = s.Get(key)
	assert.NotNil(t, err)
	assert.Equal(t, err, storage.ErrKeyNotFound)

	// Double delete.
	s.Delete(key)

	_, err = s.Get(key)
	assert.NotNil(t, err)
	assert.Equal(t, err, storage.ErrKeyNotFound)

	// Nonexistent.
	key = []byte("sparse")
	s.Delete(key)

	_, err = s.Get(key)
	assert.NotNil(t, err)
	assert.Equal(t, err, storage.ErrKeyNotFound)
}

func getFromStore(t *testing.T, ps storage.Store, key []byte) ([]byte, error) {
	view, err := ps.BeginView()
	require.NoError(t, err)
	v, err := ps.GetWithView(view, key)
	require.NoError(t, view.Close())
	return v, err
}

func testMemCachedStorePersist(t *testing.T, ps storage.Store) {
	// cached Store
	ts := storage.NewMemCachedStore(ps)
	// persisting nothing should do nothing
	c, err := ts.Persist()
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
	// persisting one key should result in one key in ps and nothing in ts
	ts.Put([]byte("key"), []byte("value"))
	checkBatch(t, ts, []storage.KeyValueExists{{KeyValue: storage.KeyValue{Key: []byte("key"), Value: []byte("value")}}}, nil)
	c, err = ts.Persist()
	checkBatch(t, ts, nil, nil)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, c)
	v, err := getFromStore(t, ps, []byte("key"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("value"), v)
	v, err = ts.MemoryStore.GetWithView(nil, []byte("key"))
	assert.Equal(t, storage.ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	// now we overwrite the previous `key` contents and also add `key2`,
	ts.Put([]byte("key"), []byte("newvalue"))
	ts.Put([]byte("key2"), []byte("value2"))
	// this is to check that now key is written into the ps before we do
	// persist
	v, err = getFromStore(t, ps, []byte("key2"))
	assert.Equal(t, storage.ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	checkBatch(t, ts, []storage.KeyValueExists{
		{KeyValue: storage.KeyValue{Key: []byte("key"), Value: []byte("newvalue")}, Exists: true},
		{KeyValue: storage.KeyValue{Key: []byte("key2"), Value: []byte("value2")}},
	}, nil)
	// two keys should be persisted (one overwritten and one new) and
	// available in the ps
	c, err = ts.Persist()
	checkBatch(t, ts, nil, nil)
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, c)
	v, err = ts.MemoryStore.GetWithView(nil, []byte("key"))
	assert.Equal(t, storage.ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	v, err = ts.MemoryStore.GetWithView(nil, []byte("key2"))
	assert.Equal(t, storage.ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	v, err = getFromStore(t, ps, []byte("key"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("newvalue"), v)
	v, err = getFromStore(t, ps, []byte("key2"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("value2"), v)
	checkBatch(t, ts, nil, nil)
	// we've persisted some values, make sure successive persist is a no-op
	c, err = ts.Persist()
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
	// test persisting deletions
	ts.Delete([]byte("key"))
	checkBatch(t, ts, nil, []storage.KeyValueExists{{KeyValue: storage.KeyValue{Key: []byte("key")}, Exists: true}})
	c, err = ts.Persist()
	checkBatch(t, ts, nil, nil)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, c)
	v, err = getFromStore(t, ps, []byte("key"))
	assert.Equal(t, storage.ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	v, err = getFromStore(t, ps, []byte("key2"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("value2"), v)
}

func checkBatch(t *testing.T, ts *storage.MemCachedStore, put []storage.KeyValueExists, del []storage.KeyValueExists) {
	b := ts.GetBatch()
	assert.Equal(t, len(put), len(b.Put), "wrong number of put elements in a batch")
	assert.Equal(t, len(del), len(b.Deleted), "wrong number of deleted elements in a batch")

	for i := range put {
		assert.Contains(t, b.Put, put[i])
	}

	for i := range del {
		assert.Contains(t, b.Deleted, del[i])
	}
}

func TestMemCachedPersist(t *testing.T) {
	t.Run("MemoryStore", func(t *testing.T) {
		ps := storage.NewMemoryStore()
		testMemCachedStorePersist(t, ps)
	})
	t.Run("MemoryCachedStore", func(t *testing.T) {
		ps1 := storage.NewMemoryStore()
		ps2 := storage.NewMemCachedStore(ps1)
		testMemCachedStorePersist(t, ps2)
	})
	t.Run("BoltDBStore", func(t *testing.T) {
		ps := store.NewBoltDBForTesting(t)
		t.Cleanup(func() {
			err := ps.Close()
			require.NoError(t, err)
		})
		testMemCachedStorePersist(t, ps)
	})
}

func TestCachedGetFromPersistent(t *testing.T) {
	key := []byte("key")
	value := []byte("value")
	ps := storage.NewMemoryStore()
	ts := storage.NewMemCachedStore(ps)

	assert.NoError(t, ps.PutChangeSet(map[string][]byte{string(key): value}, nil))
	val, err := ts.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value, val)
	ts.Delete(key)
	val, err = ts.Get(key)
	assert.Equal(t, err, storage.ErrKeyNotFound)
	assert.Nil(t, val)
}

func TestCachedSeek(t *testing.T) {
	var (
		// Given this prefix...
		goodPrefix = []byte{'f'}
		// these pairs should be found...
		lowerKVs = []storage.KeyValue{
			{[]byte("foo"), []byte("bar")},
			{[]byte("faa"), []byte("bra")},
		}
		// and these should be not.
		deletedKVs = []storage.KeyValue{
			{[]byte("fee"), []byte("pow")},
			{[]byte("fii"), []byte("qaz")},
		}
		// and these should be not.
		updatedKVs = []storage.KeyValue{
			{[]byte("fuu"), []byte("wop")},
			{[]byte("fyy"), []byte("zaq")},
		}
		ps = storage.NewMemoryStore()
		ts = storage.NewMemCachedStore(ps)
	)
	for _, v := range lowerKVs {
		require.NoError(t, ps.PutChangeSet(map[string][]byte{string(v.Key): v.Value}, nil))
	}
	for _, v := range deletedKVs {
		require.NoError(t, ps.PutChangeSet(map[string][]byte{string(v.Key): v.Value}, nil))
		ts.Delete(v.Key)
	}
	for _, v := range updatedKVs {
		require.NoError(t, ps.PutChangeSet(map[string][]byte{string(v.Key): v.Value}, nil))
		ts.Put(v.Key, v.Value)
	}
	foundKVs := make(map[string][]byte)
	ts.Seek(storage.SeekRange{Prefix: goodPrefix}, func(k, v []byte) bool {
		foundKVs[string(k)] = v
		return true
	})
	assert.Equal(t, len(foundKVs), len(lowerKVs)+len(updatedKVs))
	for _, kv := range lowerKVs {
		assert.Equal(t, kv.Value, foundKVs[string(kv.Key)])
	}
	for _, kv := range deletedKVs {
		_, ok := foundKVs[string(kv.Key)]
		assert.Equal(t, false, ok)
	}
	for _, kv := range updatedKVs {
		assert.Equal(t, kv.Value, foundKVs[string(kv.Key)])
	}
}

func TestCachedSeekBackwardsEmptyStart(t *testing.T) {
	ps := storage.NewMemoryStore()
	ts := storage.NewMemCachedStore(ps)

	require.NoError(t, ps.PutChangeSet(map[string][]byte{
		"20": []byte("barb"),
		"21": []byte("barc"),
	}, nil))
	ts.Put([]byte("22"), []byte("bard"))

	var actual []storage.KeyValue
	ts.Seek(storage.SeekRange{Prefix: []byte("2"), Start: []byte{}, Backwards: true}, func(k, v []byte) bool {
		actual = append(actual, storage.KeyValue{Key: bytes.Clone(k), Value: bytes.Clone(v)})
		return true
	})
	require.ElementsMatch(t, []storage.KeyValue{}, actual)
}

func benchmarkCachedSeek(t *testing.B, ps storage.Store, psElementsCount, tsElementsCount int) {
	var (
		searchPrefix      = []byte{1}
		badPrefix         = []byte{2}
		lowerPrefixGood   = append(searchPrefix, 1)
		lowerPrefixBad    = append(badPrefix, 1)
		deletedPrefixGood = append(searchPrefix, 2)
		deletedPrefixBad  = append(badPrefix, 2)
		updatedPrefixGood = append(searchPrefix, 3)
		updatedPrefixBad  = append(badPrefix, 3)

		ts = storage.NewMemCachedStore(ps)
	)
	for i := range psElementsCount {
		// lower KVs with matching prefix that should be found
		ts.Put(append(lowerPrefixGood, random.Bytes(10)...), []byte("value"))
		// lower KVs with non-matching prefix that shouldn't be found
		ts.Put(append(lowerPrefixBad, random.Bytes(10)...), []byte("value"))

		// deleted KVs with matching prefix that shouldn't be found
		key := append(deletedPrefixGood, random.Bytes(10)...)
		ts.Put(key, []byte("deleted"))
		if i < tsElementsCount {
			ts.Delete(key)
		}
		// deleted KVs with non-matching prefix that shouldn't be found
		key = append(deletedPrefixBad, random.Bytes(10)...)
		ts.Put(key, []byte("deleted"))
		if i < tsElementsCount {
			ts.Delete(key)
		}

		// updated KVs with matching prefix that should be found
		key = append(updatedPrefixGood, random.Bytes(10)...)
		ts.Put(key, []byte("stub"))
		if i < tsElementsCount {
			ts.Put(key, []byte("updated"))
		}
		// updated KVs with non-matching prefix that shouldn't be found
		key = append(updatedPrefixBad, random.Bytes(10)...)
		ts.Put(key, []byte("stub"))
		if i < tsElementsCount {
			ts.Put(key, []byte("updated"))
		}
	}
	_, err := ts.PersistSync()
	require.NoError(t, err)

	t.ReportAllocs()
	for t.Loop() {
		ts.Seek(storage.SeekRange{Prefix: searchPrefix}, func(k, v []byte) bool { return true })
	}
	t.StopTimer()
}

func BenchmarkCachedSeek(t *testing.B) {
	var stores = map[string]func(testing.TB) storage.Store{
		"MemPS": func(t testing.TB) storage.Store {
			return storage.NewMemoryStore()
		},
		"BoltPS":  store.NewBoltDBForTesting,
		"LevelPS": store.NewLevelDBForTesting,
	}
	for psName, newPS := range stores {
		for psCount := 100; psCount <= 10000; psCount *= 10 {
			for tsCount := 10; tsCount <= psCount; tsCount *= 10 {
				t.Run(fmt.Sprintf("%s_%dTSItems_%dPSItems", psName, tsCount, psCount), func(t *testing.B) {
					ps := newPS(t)
					benchmarkCachedSeek(t, ps, psCount, tsCount)
					ps.Close()
				})
			}
		}
	}
}

type badView struct{}

func (badView) Close() error { return nil }

type BadStore struct {
	onPutBatch func()
}

func (b *BadStore) Delete(k []byte) error {
	return nil
}
func (b *BadStore) Get([]byte) ([]byte, error) {
	return nil, storage.ErrKeyNotFound
}
func (b *BadStore) Put(k, v []byte) error {
	return nil
}
func (b *BadStore) PutChangeSet(_ map[string][]byte, _ map[string][]byte) error {
	b.onPutBatch()
	return storage.ErrKeyNotFound
}
func (b *BadStore) Seek(rng storage.SeekRange, f func(k, v []byte) bool) {
}
func (b *BadStore) SeekGC(rng storage.SeekRange, keepCont func(k, v []byte) (bool, bool)) error {
	return nil
}
func (b *BadStore) BeginView() (storage.IView, error) {
	return badView{}, nil
}
func (b *BadStore) GetWithView(_ storage.IView, k []byte) ([]byte, error) {
	return b.Get(k)
}
func (b *BadStore) SeekWithView(_ storage.IView, rng storage.SeekRange, f func(k, v []byte) bool) {
	b.Seek(rng, f)
}
func (b *BadStore) Close() error {
	return nil
}

func TestMemCachedPersistFailing(t *testing.T) {
	var (
		bs BadStore
		t1 = []byte("t1")
		t2 = []byte("t2")
		b1 = []byte("b1")
	)
	// cached Store
	ts := storage.NewMemCachedStore(&bs)
	// Set a pair of keys.
	ts.Put(t1, t1)
	ts.Put(t2, t2)
	// This will be called during Persist().
	bs.onPutBatch = func() {
		// Drop one, add one.
		ts.Put(b1, b1)
		ts.Delete(t1)
	}
	_, err := ts.Persist()
	require.Error(t, err)
	// PutBatch() failed in Persist, but we still should have proper state.
	_, err = ts.Get(t1)
	require.Error(t, err)
	res, err := ts.Get(t2)
	require.NoError(t, err)
	require.Equal(t, t2, res)
	res, err = ts.Get(b1)
	require.NoError(t, err)
	require.Equal(t, b1, res)
}

func TestPrivateMemCachedPersistFailing(t *testing.T) {
	var (
		bs BadStore
		t1 = []byte("t1")
		t2 = []byte("t2")
	)
	// cached Store
	ts := storage.NewPrivateMemCachedStore(&bs)
	// Set a pair of keys.
	ts.Put(t1, t1)
	ts.Put(t2, t2)
	// This will be called during Persist().
	bs.onPutBatch = func() {}

	_, err := ts.Persist()
	require.Error(t, err)
	// PutBatch() failed in Persist, but we still should have proper state.
	res, err := ts.Get(t1)
	require.NoError(t, err)
	require.Equal(t, t1, res)
	res, err = ts.Get(t2)
	require.NoError(t, err)
	require.Equal(t, t2, res)
}

func TestCachedSeekSorting(t *testing.T) {
	var (
		// Given this prefix...
		goodPrefix = []byte{1}
		// these pairs should be found...
		lowerKVs = []storage.KeyValue{
			{[]byte{1, 2, 3}, []byte("bra")},
			{[]byte{1, 2, 5}, []byte("bar")},
			{[]byte{1, 3, 3}, []byte("bra")},
			{[]byte{1, 3, 5}, []byte("bra")},
		}
		// and these should be not.
		deletedKVs = []storage.KeyValue{
			{[]byte{1, 7, 3}, []byte("pow")},
			{[]byte{1, 7, 4}, []byte("qaz")},
		}
		// and these should be not.
		updatedKVs = []storage.KeyValue{
			{[]byte{1, 2, 4}, []byte("zaq")},
			{[]byte{1, 2, 6}, []byte("zaq")},
			{[]byte{1, 3, 2}, []byte("wop")},
			{[]byte{1, 3, 4}, []byte("zaq")},
		}
	)
	for _, newCached := range []func(storage.Store) *storage.MemCachedStore{storage.NewMemCachedStore, storage.NewPrivateMemCachedStore} {
		ps := storage.NewMemoryStore()
		ts := newCached(ps)
		for _, v := range lowerKVs {
			require.NoError(t, ps.PutChangeSet(map[string][]byte{string(v.Key): v.Value}, nil))
		}
		for _, v := range deletedKVs {
			require.NoError(t, ps.PutChangeSet(map[string][]byte{string(v.Key): v.Value}, nil))
			ts.Delete(v.Key)
		}
		for _, v := range updatedKVs {
			require.NoError(t, ps.PutChangeSet(map[string][]byte{string(v.Key): v.Value}, nil))
			ts.Put(v.Key, v.Value)
		}
		var foundKVs []storage.KeyValue
		ts.Seek(storage.SeekRange{Prefix: goodPrefix}, func(k, v []byte) bool {
			foundKVs = append(foundKVs, storage.KeyValue{Key: bytes.Clone(k), Value: bytes.Clone(v)})
			return true
		})
		assert.Equal(t, len(foundKVs), len(lowerKVs)+len(updatedKVs))
		expected := append(lowerKVs, updatedKVs...)
		slices.SortFunc(expected, func(a, b storage.KeyValue) int {
			return bytes.Compare(a.Key, b.Key)
		})
		require.Equal(t, expected, foundKVs)
	}
}

func testMemCachedViewConsistentAcrossPersist(t *testing.T, ps storage.Store) {
	s := storage.NewMemCachedStore(ps)

	prefix := byte(storage.STStorage)
	k0 := []byte{prefix, 'k', '0'}
	k1 := []byte{prefix, 'k', '1'}
	k2 := []byte{prefix, 'k', '2'}
	kdel := []byte{prefix, 'k', 'd'}

	// Initial disk state T0. Also pre-grow and free pages so subsequent small
	// BoltDB commits fit in the existing mmap without blocking on remap while
	// priv holds a read transaction on the same goroutine.
	padKey := []byte{0x01, 'p', 'a', 'd'}
	s.Put(padKey, make([]byte, 64*1024))
	_, err := s.Persist()
	require.NoError(t, err)
	s.Delete(padKey)
	s.Put(k0, []byte("old"))
	s.Put(kdel, []byte("gone"))
	_, err = s.Persist()
	require.NoError(t, err)

	// Unpersisted changes in s.
	s.Put(k0, []byte("new"))
	s.Put(k1, []byte("added"))
	s.Delete(kdel)

	// Pin a view at disk state T0.
	priv := storage.NewPrivateMemCachedStoreWithView(s)
	defer func() {
		require.NoError(t, priv.Finalize())
	}()
	child := storage.NewPrivateMemCachedStore(priv)

	// Flush s to disk (T1) while priv still holds view@T0.
	n, err := s.Persist()
	require.NoError(t, err)
	require.Equal(t, 3, n)

	checkState := func(st *storage.MemCachedStore, wantK0, wantK1, wantK2 []byte) {
		v, err := st.Get(k0)
		require.NoError(t, err)
		require.Equal(t, wantK0, v)

		v, err = st.Get(k1)
		require.NoError(t, err)
		require.Equal(t, wantK1, v)

		if wantK2 != nil {
			v, err = st.Get(k2)
			require.NoError(t, err)
			require.Equal(t, wantK2, v)
		}

		_, err = st.Get(kdel)
		require.ErrorIs(t, err, storage.ErrKeyNotFound)

		var expected []storage.KeyValue
		expected = append(expected,
			storage.KeyValue{Key: k0, Value: wantK0},
			storage.KeyValue{Key: k1, Value: wantK1},
		)
		if wantK2 != nil {
			expected = append(expected, storage.KeyValue{Key: k2, Value: wantK2})
		}

		var got []storage.KeyValue
		st.Seek(storage.SeekRange{Prefix: []byte{prefix}}, func(k, v []byte) bool {
			got = append(got, storage.KeyValue{Key: bytes.Clone(k), Value: bytes.Clone(v)})
			return true
		})
		require.Equal(t, expected, got)

		var gotAsync []storage.KeyValue
		for kv := range st.SeekAsync(context.Background(), storage.SeekRange{Prefix: []byte{prefix}}, false) {
			gotAsync = append(gotAsync, storage.KeyValue{Key: bytes.Clone(kv.Key), Value: bytes.Clone(kv.Value)})
		}
		require.Equal(t, expected, gotAsync)
	}

	checkState(priv, []byte("new"), []byte("added"), nil)
	checkState(child, []byte("new"), []byte("added"), nil)

	// Verify GetBatch sees keys persisted in T1 as existing.
	batchCheck := storage.NewPrivateMemCachedStore(priv)
	batchCheck.Put(k1, []byte("override"))
	batchCheck.Delete(k0)
	checkBatch(t, batchCheck,
		[]storage.KeyValueExists{{KeyValue: storage.KeyValue{Key: k1, Value: []byte("override")}, Exists: true}},
		[]storage.KeyValueExists{{KeyValue: storage.KeyValue{Key: k0}, Exists: true}},
	)

	// Open an overlapping second view at T1, then run a second Persist (T2).
	priv2 := storage.NewPrivateMemCachedStoreWithView(s)
	defer func() {
		require.NoError(t, priv2.Finalize())
	}()

	s.Put(k0, []byte("newer"))
	s.Put(k2, []byte("added2"))
	n, err = s.Persist()
	require.NoError(t, err)
	require.Equal(t, 2, n)

	checkState(priv, []byte("newer"), []byte("added"), []byte("added2"))
	checkState(priv2, []byte("newer"), []byte("added"), []byte("added2"))

	// Closing the older view prunes T1 layer while keeping T2 layer for priv2.
	require.NoError(t, priv.Finalize())
	// Finalize is idempotent.
	require.NoError(t, priv.Finalize())

	checkState(priv2, []byte("newer"), []byte("added"), []byte("added2"))
	require.NoError(t, priv2.Finalize())

	// Direct reads on s after all views close see the persisted T2 state.
	checkState(s, []byte("newer"), []byte("added"), []byte("added2"))
}

func TestMemCachedViewConsistentAcrossPersist(t *testing.T) {
	t.Run("BoltDBStore", func(t *testing.T) {
		ps := store.NewBoltDBForTesting(t)
		t.Cleanup(func() {
			require.NoError(t, ps.Close())
		})
		testMemCachedViewConsistentAcrossPersist(t, ps)
	})
	t.Run("LevelDBStore", func(t *testing.T) {
		ps := store.NewLevelDBForTesting(t)
		t.Cleanup(func() {
			require.NoError(t, ps.Close())
		})
		testMemCachedViewConsistentAcrossPersist(t, ps)
	})
	t.Run("NestedMemCachedStore", func(t *testing.T) {
		ps := store.NewLevelDBForTesting(t)
		t.Cleanup(func() {
			require.NoError(t, ps.Close())
		})
		lower := storage.NewMemCachedStore(ps)
		testMemCachedViewConsistentAcrossPersist(t, lower)
	})
}

func TestMemCachedViewPersistFailing(t *testing.T) {
	var (
		bs BadStore
		k0 = []byte{byte(storage.STStorage), 'k', '0'}
		k1 = []byte{byte(storage.STStorage), 'k', '1'}
	)
	ts := storage.NewMemCachedStore(&bs)
	ts.Put(k0, []byte("v0"))

	priv := storage.NewPrivateMemCachedStoreWithView(ts)
	defer func() {
		require.NoError(t, priv.Finalize())
	}()

	bs.onPutBatch = func() {
		ts.Put(k1, []byte("v1"))
		// Concurrent read while Persist is in-flight.
		v, err := priv.Get(k0)
		require.NoError(t, err)
		require.Equal(t, []byte("v0"), v)
	}
	_, err := ts.Persist()
	require.Error(t, err)

	v, err := priv.Get(k0)
	require.NoError(t, err)
	require.Equal(t, []byte("v0"), v)
	v, err = priv.Get(k1)
	require.NoError(t, err)
	require.Equal(t, []byte("v1"), v)
}

func TestMemCachedViewConcurrentPersist(t *testing.T) {
	for name, newPS := range map[string]func(testing.TB) storage.Store{
		"MemoryStore":  func(testing.TB) storage.Store { return storage.NewMemoryStore() },
		"BoltDBStore":  store.NewBoltDBForTesting,
		"LevelDBStore": store.NewLevelDBForTesting,
	} {
		t.Run(name, func(t *testing.T) {
			ps := newPS(t)
			t.Cleanup(func() {
				require.NoError(t, ps.Close())
			})
			s := storage.NewMemCachedStore(ps)

			const keyCount = 32
			prefix := byte(storage.STStorage)
			keys := make([][]byte, keyCount)
			for i := range keyCount {
				keys[i] = []byte{prefix, 'k', byte(i)}
				s.Put(keys[i], []byte("init"))
			}

			var wg sync.WaitGroup
			stop := make(chan struct{})

			// Background persister.
			wg.Go(func() {
				for {
					select {
					case <-stop:
						return
					default:
						_, err := s.Persist()
						require.NoError(t, err)
					}
				}
			})

			// Readers holding views across concurrent Persist calls.
			for range 4 {
				wg.Go(func() {
					for range 40 {
						priv := storage.NewPrivateMemCachedStoreWithView(s)
						for _, k := range keys {
							v, err := priv.Get(k)
							require.NoError(t, err)
							require.NotEmpty(t, v)
						}
						var seen int
						priv.Seek(storage.SeekRange{Prefix: []byte{prefix}}, func(_, _ []byte) bool {
							seen++
							return true
						})
						require.Equal(t, keyCount, seen)
						require.NoError(t, priv.Finalize())
					}
				})
			}

			// Writer updating keys concurrently (never deleting them).
			for i := range 40 {
				val := []byte(fmt.Sprintf("v-%d", i))
				for _, k := range keys {
					s.Put(k, val)
				}
			}

			close(stop)
			wg.Wait()
		})
	}
}

func TestMemCachedViewCloseDuringPersistSync(t *testing.T) {
	ps := storage.NewMemoryStore()
	s := storage.NewMemCachedStore(ps)
	k := []byte{byte(storage.STStorage), 'k'}
	s.Put(k, []byte("v1"))

	priv := storage.NewPrivateMemCachedStoreWithView(s)

	done := make(chan error, 1)
	go func() {
		_, err := s.PersistSync()
		done <- err
	}()

	// Closing the view while PersistSync holds s.mut and waits on ps.mut must not deadlock.
	require.NoError(t, priv.Finalize())
	require.NoError(t, <-done)

	v, err := s.Get(k)
	require.NoError(t, err)
	require.Equal(t, []byte("v1"), v)
}
