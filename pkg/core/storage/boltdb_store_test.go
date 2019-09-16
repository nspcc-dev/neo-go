package storage

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoltDBBatch(t *testing.T) {
	boltDB := BoltDBStore{}
	want := &BoltDBBatch{mem: map[*[]byte][]byte{}}
	if got := boltDB.Batch(); !reflect.DeepEqual(got, want) {
		t.Errorf("BoltDB Batch() = %v, want %v", got, want)
	}
}

func TestBoltDBBatch_Len(t *testing.T) {
	batch := &BoltDBBatch{mem: map[*[]byte][]byte{}}
	want := len(map[*[]byte][]byte{})
	assert.Equal(t, want, batch.Len())
}

func TestBoltDBBatch_PutBatchAndGet(t *testing.T) {
	key := []byte("foo")
	value := []byte("bar")
	batch := &BoltDBBatch{mem: map[*[]byte][]byte{&key: value}}

	boltDBStore := openStore(t)

	errPut := boltDBStore.PutBatch(batch)
	assert.Nil(t, errPut, "Error while PutBatch")

	result, err := boltDBStore.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value, result)

	require.NoError(t, boltDBStore.Close())
}

func TestBoltDBBatch_PutAndGet(t *testing.T) {
	key := []byte("foo")
	value := []byte("bar")

	boltDBStore := openStore(t)

	errPut := boltDBStore.Put(key, value)
	assert.Nil(t, errPut, "Error while Put")

	result, err := boltDBStore.Get(key)
	assert.Nil(t, err)
	assert.Equal(t, value, result)

	require.NoError(t, boltDBStore.Close())
}

func TestBoltDBStore_Seek(t *testing.T) {
	key := []byte("foo")
	value := []byte("bar")

	boltDBStore := openStore(t)

	errPut := boltDBStore.Put(key, value)
	assert.Nil(t, errPut, "Error while Put")

	boltDBStore.Seek(key, func(k, v []byte) {
		assert.Equal(t, value, v)
	})

	require.NoError(t, boltDBStore.Close())
}

func openStore(t *testing.T) *BoltDBStore {
	testFileName := "test_bolt_db"
	file, err := ioutil.TempFile("", testFileName)
	defer func() {
		err := os.RemoveAll(testFileName)
		require.NoError(t, err)
	}()
	require.NoError(t, err)
	require.NoError(t, file.Close())
	boltDBStore, err := NewBoltDBStore(BoltDBOptions{FilePath: testFileName})
	require.NoError(t, err)
	return boltDBStore
}
