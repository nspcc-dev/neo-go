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
