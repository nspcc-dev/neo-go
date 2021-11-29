package storage

import (
	"bytes"
	"fmt"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testMemCachedStorePersist(t *testing.T, ps Store) {
	// cached Store
	ts := NewMemCachedStore(ps)
	// persisting nothing should do nothing
	c, err := ts.Persist()
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
	// persisting one key should result in one key in ps and nothing in ts
	assert.NoError(t, ts.Put([]byte("key"), []byte("value")))
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
	assert.NoError(t, ts.Put([]byte("key"), []byte("newvalue")))
	assert.NoError(t, ts.Put([]byte("key2"), []byte("value2")))
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
	err = ts.Delete([]byte("key"))
	assert.Equal(t, nil, err)
	checkBatch(t, ts, nil, []KeyValueExists{{KeyValue: KeyValue{Key: []byte("key")}, Exists: true}})
	c, err = ts.Persist()
	checkBatch(t, ts, nil, nil)
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
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

	assert.NoError(t, ps.Put(key, value))
	val, err := ts.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value, val)
	assert.NoError(t, ts.Delete(key))
	val, err = ts.Get(key)
	assert.Equal(t, err, ErrKeyNotFound)
	assert.Nil(t, val)
}

func TestCachedSeek(t *testing.T) {
	var (
		// Given this prefix...
		goodPrefix = []byte{'f'}
		// these pairs should be found...
		lowerKVs = []kvSeen{
			{[]byte("foo"), []byte("bar"), false},
			{[]byte("faa"), []byte("bra"), false},
		}
		// and these should be not.
		deletedKVs = []kvSeen{
			{[]byte("fee"), []byte("pow"), false},
			{[]byte("fii"), []byte("qaz"), false},
		}
		// and these should be not.
		updatedKVs = []kvSeen{
			{[]byte("fuu"), []byte("wop"), false},
			{[]byte("fyy"), []byte("zaq"), false},
		}
		ps = NewMemoryStore()
		ts = NewMemCachedStore(ps)
	)
	for _, v := range lowerKVs {
		require.NoError(t, ps.Put(v.key, v.val))
	}
	for _, v := range deletedKVs {
		require.NoError(t, ps.Put(v.key, v.val))
		require.NoError(t, ts.Delete(v.key))
	}
	for _, v := range updatedKVs {
		require.NoError(t, ps.Put(v.key, []byte("stub")))
		require.NoError(t, ts.Put(v.key, v.val))
	}
	foundKVs := make(map[string][]byte)
	ts.Seek(goodPrefix, func(k, v []byte) {
		foundKVs[string(k)] = v
	})
	assert.Equal(t, len(foundKVs), len(lowerKVs)+len(updatedKVs))
	for _, kv := range lowerKVs {
		assert.Equal(t, kv.val, foundKVs[string(kv.key)])
	}
	for _, kv := range deletedKVs {
		_, ok := foundKVs[string(kv.key)]
		assert.Equal(t, false, ok)
	}
	for _, kv := range updatedKVs {
		assert.Equal(t, kv.val, foundKVs[string(kv.key)])
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
		require.NoError(t, ps.Put(append(lowerPrefixGood, random.Bytes(10)...), []byte("value")))
		// lower KVs with non-matching prefix that shouldn't be found
		require.NoError(t, ps.Put(append(lowerPrefixBad, random.Bytes(10)...), []byte("value")))

		// deleted KVs with matching prefix that shouldn't be found
		key := append(deletedPrefixGood, random.Bytes(10)...)
		require.NoError(t, ps.Put(key, []byte("deleted")))
		if i < tsElementsCount {
			require.NoError(t, ts.Delete(key))
		}
		// deleted KVs with non-matching prefix that shouldn't be found
		key = append(deletedPrefixBad, random.Bytes(10)...)
		require.NoError(t, ps.Put(key, []byte("deleted")))
		if i < tsElementsCount {
			require.NoError(t, ts.Delete(key))
		}

		// updated KVs with matching prefix that should be found
		key = append(updatedPrefixGood, random.Bytes(10)...)
		require.NoError(t, ps.Put(key, []byte("stub")))
		if i < tsElementsCount {
			require.NoError(t, ts.Put(key, []byte("updated")))
		}
		// updated KVs with non-matching prefix that shouldn't be found
		key = append(updatedPrefixBad, random.Bytes(10)...)
		require.NoError(t, ps.Put(key, []byte("stub")))
		if i < tsElementsCount {
			require.NoError(t, ts.Put(key, []byte("updated")))
		}
	}

	t.ReportAllocs()
	t.ResetTimer()
	for n := 0; n < t.N; n++ {
		ts.Seek(searchPrefix, func(k, v []byte) {})
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

type BadBatch struct{}

func (b BadBatch) Delete(k []byte) {}
func (b BadBatch) Put(k, v []byte) {}

type BadStore struct {
	onPutBatch func()
}

func (b *BadStore) Batch() Batch {
	return BadBatch{}
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
func (b *BadStore) PutBatch(Batch) error {
	return nil
}
func (b *BadStore) PutChangeSet(_ map[string][]byte, _ map[string]bool) error {
	b.onPutBatch()
	return ErrKeyNotFound
}
func (b *BadStore) Seek(k []byte, f func(k, v []byte)) {
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
	require.NoError(t, ts.Put(t1, t1))
	require.NoError(t, ts.Put(t2, t2))
	// This will be called during Persist().
	bs.onPutBatch = func() {
		// Drop one, add one.
		require.NoError(t, ts.Put(b1, b1))
		require.NoError(t, ts.Delete(t1))
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

func TestCachedSeekSorting(t *testing.T) {
	var (
		// Given this prefix...
		goodPrefix = []byte{1}
		// these pairs should be found...
		lowerKVs = []kvSeen{
			{[]byte{1, 2, 3}, []byte("bra"), false},
			{[]byte{1, 2, 5}, []byte("bar"), false},
			{[]byte{1, 3, 3}, []byte("bra"), false},
			{[]byte{1, 3, 5}, []byte("bra"), false},
		}
		// and these should be not.
		deletedKVs = []kvSeen{
			{[]byte{1, 7, 3}, []byte("pow"), false},
			{[]byte{1, 7, 4}, []byte("qaz"), false},
		}
		// and these should be not.
		updatedKVs = []kvSeen{
			{[]byte{1, 2, 4}, []byte("zaq"), false},
			{[]byte{1, 2, 6}, []byte("zaq"), false},
			{[]byte{1, 3, 2}, []byte("wop"), false},
			{[]byte{1, 3, 4}, []byte("zaq"), false},
		}
		ps = NewMemoryStore()
		ts = NewMemCachedStore(ps)
	)
	for _, v := range lowerKVs {
		require.NoError(t, ps.Put(v.key, v.val))
	}
	for _, v := range deletedKVs {
		require.NoError(t, ps.Put(v.key, v.val))
		require.NoError(t, ts.Delete(v.key))
	}
	for _, v := range updatedKVs {
		require.NoError(t, ps.Put(v.key, []byte("stub")))
		require.NoError(t, ts.Put(v.key, v.val))
	}
	var foundKVs []kvSeen
	ts.Seek(goodPrefix, func(k, v []byte) {
		foundKVs = append(foundKVs, kvSeen{key: slice.Copy(k), val: slice.Copy(v)})
	})
	assert.Equal(t, len(foundKVs), len(lowerKVs)+len(updatedKVs))
	expected := append(lowerKVs, updatedKVs...)
	sort.Slice(expected, func(i, j int) bool {
		return bytes.Compare(expected[i].key, expected[j].key) < 0
	})
	require.Equal(t, expected, foundKVs)
}
