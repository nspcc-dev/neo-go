package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemCachedStorePersist(t *testing.T) {
	// persistent Store
	ps := NewMemoryStore()
	// cached Store
	ts := NewMemCachedStore(ps)
	// persisting nothing should do nothing
	c, err := ts.Persist()
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
	// persisting one key should result in one key in ps and nothing in ts
	assert.NoError(t, ts.Put([]byte("key"), []byte("value")))
	c, err = ts.Persist()
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
	// two keys should be persisted (one overwritten and one new) and
	// available in the ps
	c, err = ts.Persist()
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
	// we've persisted some values, make sure successive persist is a no-op
	c, err = ts.Persist()
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
	// test persisting deletions
	err = ts.Delete([]byte("key"))
	assert.Equal(t, nil, err)
	c, err = ts.Persist()
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
	v, err = ps.Get([]byte("key"))
	assert.Equal(t, ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	v, err = ps.Get([]byte("key2"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("value2"), v)
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
