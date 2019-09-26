package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPut(t *testing.T) {
	var (
		s     = NewMemoryStore()
		key   = []byte("sparse")
		value = []byte("rocks")
	)

	if err := s.Put(key, value); err != nil {
		t.Fatal(err)
	}

	newVal, err := s.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, value, newVal)
	require.NoError(t, s.Close())
}

func TestKeyNotExist(t *testing.T) {
	var (
		s   = NewMemoryStore()
		key = []byte("sparse")
	)

	_, err := s.Get(key)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "key not found")
	require.NoError(t, s.Close())
}

func TestPutBatch(t *testing.T) {
	var (
		s     = NewMemoryStore()
		key   = []byte("sparse")
		value = []byte("rocks")
		batch = s.Batch()
	)

	batch.Put(key, value)

	if err := s.PutBatch(batch); err != nil {
		t.Fatal(err)
	}

	newVal, err := s.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, value, newVal)
	require.NoError(t, s.Close())
}

func TestMemoryStore_Seek(t *testing.T) {
	var (
		s     = NewMemoryStore()
		key   = []byte("sparse")
		value = []byte("rocks")
	)

	if err := s.Put(key, value); err != nil {
		t.Fatal(err)
	}

	s.Seek(key, func(k, v []byte) {
		assert.Equal(t, value, v)
	})
}

func TestMemoryStorePersist(t *testing.T) {
	// temporary Store
	ts := NewMemoryStore()
	// persistent Store
	ps := NewMemoryStore()
	// persisting nothing should do nothing
	c, err := ts.Persist(ps)
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
	// persisting one key should result in one key in ps and nothing in ts
	assert.NoError(t, ts.Put([]byte("key"), []byte("value")))
	c, err = ts.Persist(ps)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, c)
	v, err := ps.Get([]byte("key"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("value"), v)
	v, err = ts.Get([]byte("key"))
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
	c, err = ts.Persist(ps)
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, c)
	v, err = ts.Get([]byte("key"))
	assert.Equal(t, ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	v, err = ts.Get([]byte("key2"))
	assert.Equal(t, ErrKeyNotFound, err)
	assert.Equal(t, []byte(nil), v)
	v, err = ps.Get([]byte("key"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("newvalue"), v)
	v, err = ps.Get([]byte("key2"))
	assert.Equal(t, nil, err)
	assert.Equal(t, []byte("value2"), v)
	// we've persisted some values, make sure successive persist is a no-op
	c, err = ts.Persist(ps)
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, c)
}
