package storage

import (
	"testing"

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
	checkBatch(t, ts, []KeyValue{{Key: []byte("key"), Value: []byte("value")}}, nil)
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
	checkBatch(t, ts, []KeyValue{
		{Key: []byte("key"), Value: []byte("newvalue"), Exists: true},
		{Key: []byte("key2"), Value: []byte("value2")},
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
	checkBatch(t, ts, nil, []KeyValue{{Key: []byte("key"), Exists: true}})
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

func checkBatch(t *testing.T, ts *MemCachedStore, put []KeyValue, del []KeyValue) {
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

func newMemCachedStoreForTesting(t *testing.T) Store {
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
