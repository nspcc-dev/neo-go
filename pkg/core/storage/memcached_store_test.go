package storage

import (
	"bytes"
	"fmt"
	"slices"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemCachedPutGetDelete(t *testing.T) {
	ps := NewMemoryStore()
	s := NewMemCachedStore(ps)
	key := []byte("foo")
	value := []byte("bar")

	s.Put(key, value)

	result, err := s.Get(key)
	assert.Nil(t, err)
	require.Equal(t, value, result)

	s.Delete(key)

	_, err = s.Get(key)
	assert.NotNil(t, err)
	assert.Equal(t, err, ErrKeyNotFound)

	// Double delete.
	s.Delete(key)

	_, err = s.Get(key)
	assert.NotNil(t, err)
	assert.Equal(t, err, ErrKeyNotFound)

	// Nonexistent.
	key = []byte("sparse")
	s.Delete(key)

	_, err = s.Get(key)
	assert.NotNil(t, err)
	assert.Equal(t, err, ErrKeyNotFound)
}

func testMemCachedStorePersist(t *testing.T, ps Store) {
	// cached Store
	ts := NewMemCachedStore(ps)
	// persisting nothing should do nothing
	c, err := ts.Persist()
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
	// persisting one key should result in one key in ps and nothing in ts
	ts.Put([]byte("key"), []byte("value"))
	checkBatch(t, ts, []KeyValueExists{{KeyValue: KeyValue{Key: []byte("key"), Value: []byte("value")}}}, nil)
	c, err = ts.Persist()
	checkBatch(t, ts, nil, nil)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, c)
	v, err := ps.Get([]byte("key"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("value"), v)
	v, err = ts.MemoryStore.Get([]byte("key"))
	assert.Equal(t, ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	// now we overwrite the previous `key` contents and also add `key2`,
	ts.Put([]byte("key"), []byte("newvalue"))
	ts.Put([]byte("key2"), []byte("value2"))
	// this is to check that now key is written into the ps before we do
	// persist
	v, err = ps.Get([]byte("key2"))
	assert.Equal(t, ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	checkBatch(t, ts, []KeyValueExists{
		{KeyValue: KeyValue{Key: []byte("key"), Value: []byte("newvalue")}, Exists: true},
		{KeyValue: KeyValue{Key: []byte("key2"), Value: []byte("value2")}},
	}, nil)
	// two keys should be persisted (one overwritten and one new) and
	// available in the ps
	c, err = ts.Persist()
	checkBatch(t, ts, nil, nil)
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, c)
	v, err = ts.MemoryStore.Get([]byte("key"))
	assert.Equal(t, ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	v, err = ts.MemoryStore.Get([]byte("key2"))
	assert.Equal(t, ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	v, err = ps.Get([]byte("key"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("newvalue"), v)
	v, err = ps.Get([]byte("key2"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("value2"), v)
	checkBatch(t, ts, nil, nil)
	// we've persisted some values, make sure successive persist is a no-op
	c, err = ts.Persist()
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
	// test persisting deletions
	ts.Delete([]byte("key"))
	checkBatch(t, ts, nil, []KeyValueExists{{KeyValue: KeyValue{Key: []byte("key")}, Exists: true}})
	c, err = ts.Persist()
	checkBatch(t, ts, nil, nil)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, c)
	v, err = ps.Get([]byte("key"))
	assert.Equal(t, ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	v, err = ps.Get([]byte("key2"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("value2"), v)
}

func checkBatch(t *testing.T, ts *MemCachedStore, put []KeyValueExists, del []KeyValueExists) {
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
		ps := NewMemoryStore()
		testMemCachedStorePersist(t, ps)
	})
	t.Run("MemoryCachedStore", func(t *testing.T) {
		ps1 := NewMemoryStore()
		ps2 := NewMemCachedStore(ps1)
		testMemCachedStorePersist(t, ps2)
	})
	t.Run("BoltDBStore", func(t *testing.T) {
		ps := newBoltStoreForTesting(t)
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
	ps := NewMemoryStore()
	ts := NewMemCachedStore(ps)

	assert.NoError(t, ps.PutChangeSet(map[string][]byte{string(key): value}, nil))
	val, err := ts.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value, val)
	ts.Delete(key)
	val, err = ts.Get(key)
	assert.Equal(t, err, ErrKeyNotFound)
	assert.Nil(t, val)
}

func TestCachedSeek(t *testing.T) {
	var (
		// Given this prefix...
		goodPrefix = []byte{'f'}
		// these pairs should be found...
		lowerKVs = []KeyValue{
			{[]byte("foo"), []byte("bar")},
			{[]byte("faa"), []byte("bra")},
		}
		// and these should be not.
		deletedKVs = []KeyValue{
			{[]byte("fee"), []byte("pow")},
			{[]byte("fii"), []byte("qaz")},
		}
		// and these should be not.
		updatedKVs = []KeyValue{
			{[]byte("fuu"), []byte("wop")},
			{[]byte("fyy"), []byte("zaq")},
		}
		ps = NewMemoryStore()
		ts = NewMemCachedStore(ps)
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
	ts.Seek(SeekRange{Prefix: goodPrefix}, func(k, v []byte) bool {
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

func benchmarkCachedSeek(t *testing.B, ps Store, psElementsCount, tsElementsCount int) {
	var (
		searchPrefix      = []byte{1}
		badPrefix         = []byte{2}
		lowerPrefixGood   = append(searchPrefix, 1)
		lowerPrefixBad    = append(badPrefix, 1)
		deletedPrefixGood = append(searchPrefix, 2)
		deletedPrefixBad  = append(badPrefix, 2)
		updatedPrefixGood = append(searchPrefix, 3)
		updatedPrefixBad  = append(badPrefix, 3)

		ts = NewMemCachedStore(ps)
	)
	for i := 0; i < psElementsCount; i++ {
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
	t.ResetTimer()
	for n := 0; n < t.N; n++ {
		ts.Seek(SeekRange{Prefix: searchPrefix}, func(k, v []byte) bool { return true })
	}
	t.StopTimer()
}

func BenchmarkCachedSeek(t *testing.B) {
	var stores = map[string]func(testing.TB) Store{
		"MemPS": func(t testing.TB) Store {
			return NewMemoryStore()
		},
		"BoltPS":  newBoltStoreForTesting,
		"LevelPS": newLevelDBForTesting,
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

func newMemCachedStoreForTesting(t testing.TB) Store {
	return NewMemCachedStore(NewMemoryStore())
}

type BadStore struct {
	onPutBatch func()
}

func (b *BadStore) Delete(k []byte) error {
	return nil
}
func (b *BadStore) Get([]byte) ([]byte, error) {
	return nil, ErrKeyNotFound
}
func (b *BadStore) Put(k, v []byte) error {
	return nil
}
func (b *BadStore) PutChangeSet(_ map[string][]byte, _ map[string][]byte) error {
	b.onPutBatch()
	return ErrKeyNotFound
}
func (b *BadStore) Seek(rng SeekRange, f func(k, v []byte) bool) {
}
func (b *BadStore) SeekGC(rng SeekRange, keep func(k, v []byte) bool) error {
	return nil
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
	ts := NewMemCachedStore(&bs)
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
	ts := NewPrivateMemCachedStore(&bs)
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
		lowerKVs = []KeyValue{
			{[]byte{1, 2, 3}, []byte("bra")},
			{[]byte{1, 2, 5}, []byte("bar")},
			{[]byte{1, 3, 3}, []byte("bra")},
			{[]byte{1, 3, 5}, []byte("bra")},
		}
		// and these should be not.
		deletedKVs = []KeyValue{
			{[]byte{1, 7, 3}, []byte("pow")},
			{[]byte{1, 7, 4}, []byte("qaz")},
		}
		// and these should be not.
		updatedKVs = []KeyValue{
			{[]byte{1, 2, 4}, []byte("zaq")},
			{[]byte{1, 2, 6}, []byte("zaq")},
			{[]byte{1, 3, 2}, []byte("wop")},
			{[]byte{1, 3, 4}, []byte("zaq")},
		}
	)
	for _, newCached := range []func(Store) *MemCachedStore{NewMemCachedStore, NewPrivateMemCachedStore} {
		ps := NewMemoryStore()
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
		var foundKVs []KeyValue
		ts.Seek(SeekRange{Prefix: goodPrefix}, func(k, v []byte) bool {
			foundKVs = append(foundKVs, KeyValue{Key: bytes.Clone(k), Value: bytes.Clone(v)})
			return true
		})
		assert.Equal(t, len(foundKVs), len(lowerKVs)+len(updatedKVs))
		expected := append(lowerKVs, updatedKVs...)
		slices.SortFunc(expected, func(a, b KeyValue) int {
			return bytes.Compare(a.Key, b.Key)
		})
		require.Equal(t, expected, foundKVs)
	}
}
