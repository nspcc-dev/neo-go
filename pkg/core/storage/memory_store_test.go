package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPut(t *testing.T) {
	var (
		s     = NewMemoryStore()
		key   = []byte("sparse")
		value = []byte("rocks")
	)

	s.Put(key, value)

	newVal, err := s.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, value, newVal)
}

func TestKeyNotExist(t *testing.T) {
	var (
		s   = NewMemoryStore()
		key = []byte("sparse")
	)

	_, err := s.Get(key)
	assert.NotNil(t, err)
	assert.Equal(t, err.Error(), "key not found")
}

func TestPutBatch(t *testing.T) {
	var (
		s     = NewMemoryStore()
		key   = []byte("sparse")
		value = []byte("rocks")
		batch = s.Batch()
	)

	batch.Put(key, value)

	s.PutBatch(batch)
	newVal, err := s.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, value, newVal)
}
