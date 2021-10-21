package storage

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kvSeen is used to test Seek implementations.
type kvSeen struct {
	key  []byte
	val  []byte
	seen bool
}

type dbSetup struct {
	name   string
	create func(testing.TB) Store
}

type dbTestFunction func(*testing.T, Store)

func testStoreClose(t *testing.T, s Store) {
	require.NoError(t, s.Close())
}

func testStorePutAndGet(t *testing.T, s Store) {
	key := []byte("foo")
	value := []byte("bar")

	require.NoError(t, s.Put(key, value))

	result, err := s.Get(key)
	assert.Nil(t, err)
	require.Equal(t, value, result)

	require.NoError(t, s.Close())
}

func testStoreGetNonExistent(t *testing.T, s Store) {
	key := []byte("sparse")

	_, err := s.Get(key)
	assert.Equal(t, err, ErrKeyNotFound)
	require.NoError(t, s.Close())
}

func testStorePutBatch(t *testing.T, s Store) {
	var (
		key   = []byte("foo")
		value = []byte("bar")
		batch = s.Batch()
	)
	// Test that key and value are copied when batching.
	keycopy := slice.Copy(key)
	valuecopy := slice.Copy(value)

	batch.Put(keycopy, valuecopy)
	copy(valuecopy, key)
	copy(keycopy, value)

	require.NoError(t, s.PutBatch(batch))
	newVal, err := s.Get(key)
	assert.Nil(t, err)
	require.Equal(t, value, newVal)
	assert.Equal(t, value, newVal)
	require.NoError(t, s.Close())
}

func testStoreSeek(t *testing.T, s Store) {
	var (
		// Given this prefix...
		goodprefix = []byte{'f'}
		// these pairs should be found...
		goodkvs = []kvSeen{
			{[]byte("foo"), []byte("bar"), false},
			{[]byte("faa"), []byte("bra"), false},
			{[]byte("foox"), []byte("barx"), false},
		}
		// and these should be not.
		badkvs = []kvSeen{
			{[]byte("doo"), []byte("pow"), false},
			{[]byte("mew"), []byte("qaz"), false},
		}
	)

	for _, v := range goodkvs {
		require.NoError(t, s.Put(v.key, v.val))
	}
	for _, v := range badkvs {
		require.NoError(t, s.Put(v.key, v.val))
	}

	numFound := 0
	s.Seek(goodprefix, func(k, v []byte) {
		for i := 0; i < len(goodkvs); i++ {
			if string(k) == string(goodkvs[i].key) {
				assert.Equal(t, string(goodkvs[i].val), string(v))
				goodkvs[i].seen = true
			}
		}
		for i := 0; i < len(badkvs); i++ {
			if string(k) == string(badkvs[i].key) {
				badkvs[i].seen = true
			}
		}
		numFound++
	})
	assert.Equal(t, len(goodkvs), numFound)
	for i := 0; i < len(goodkvs); i++ {
		assert.Equal(t, true, goodkvs[i].seen)
	}
	for i := 0; i < len(badkvs); i++ {
		assert.Equal(t, false, badkvs[i].seen)
	}
	require.NoError(t, s.Close())
}

func testStoreDeleteNonExistent(t *testing.T, s Store) {
	key := []byte("sparse")

	assert.NoError(t, s.Delete(key))
	require.NoError(t, s.Close())
}

func testStorePutAndDelete(t *testing.T, s Store) {
	key := []byte("foo")
	value := []byte("bar")

	require.NoError(t, s.Put(key, value))

	err := s.Delete(key)
	assert.Nil(t, err)

	_, err = s.Get(key)
	assert.NotNil(t, err)
	assert.Equal(t, err, ErrKeyNotFound)

	// Double delete.
	err = s.Delete(key)
	assert.Nil(t, err)

	require.NoError(t, s.Close())
}

func testStorePutBatchWithDelete(t *testing.T, s Store) {
	var (
		toBeStored = map[string][]byte{
			"foo": []byte("bar"),
			"bar": []byte("baz"),
		}
		deletedInBatch = map[string][]byte{
			"edc": []byte("rfv"),
			"tgb": []byte("yhn"),
		}
		readdedToBatch = map[string][]byte{
			"yhn": []byte("ujm"),
		}
		toBeDeleted = map[string][]byte{
			"qaz": []byte("wsx"),
			"qwe": []byte("123"),
		}
		toStay = map[string][]byte{
			"key": []byte("val"),
			"faa": []byte("bra"),
		}
	)
	for k, v := range toBeDeleted {
		require.NoError(t, s.Put([]byte(k), v))
	}
	for k, v := range toStay {
		require.NoError(t, s.Put([]byte(k), v))
	}
	batch := s.Batch()
	for k, v := range toBeStored {
		batch.Put([]byte(k), v)
	}
	for k := range toBeDeleted {
		batch.Delete([]byte(k))
	}
	for k, v := range readdedToBatch {
		batch.Put([]byte(k), v)
	}
	for k, v := range deletedInBatch {
		batch.Put([]byte(k), v)
	}
	for k := range deletedInBatch {
		batch.Delete([]byte(k))
	}
	for k := range readdedToBatch {
		batch.Delete([]byte(k))
	}
	for k, v := range readdedToBatch {
		batch.Put([]byte(k), v)
	}
	require.NoError(t, s.PutBatch(batch))
	toBe := []map[string][]byte{toStay, toBeStored, readdedToBatch}
	notToBe := []map[string][]byte{deletedInBatch, toBeDeleted}
	for _, kvs := range toBe {
		for k, v := range kvs {
			value, err := s.Get([]byte(k))
			assert.Nil(t, err)
			assert.Equal(t, value, v)
		}
	}
	for _, kvs := range notToBe {
		for k, v := range kvs {
			_, err := s.Get([]byte(k))
			assert.Equal(t, ErrKeyNotFound, err, "%s:%s", k, v)
		}
	}
	require.NoError(t, s.Close())
}

func TestAllDBs(t *testing.T) {
	var DBs = []dbSetup{
		{"BoltDB", newBoltStoreForTesting},
		{"LevelDB", newLevelDBForTesting},
		{"MemCached", newMemCachedStoreForTesting},
		{"Memory", newMemoryStoreForTesting},
		{"RedisDB", newRedisStoreForTesting},
		{"BadgerDB", newBadgerDBForTesting},
	}
	var tests = []dbTestFunction{testStoreClose, testStorePutAndGet,
		testStoreGetNonExistent, testStorePutBatch, testStoreSeek,
		testStoreDeleteNonExistent, testStorePutAndDelete,
		testStorePutBatchWithDelete}
	for _, db := range DBs {
		for _, test := range tests {
			s := db.create(t)
			twrapper := func(t *testing.T) {
				test(t, s)
			}
			fname := runtime.FuncForPC(reflect.ValueOf(test).Pointer()).Name()
			t.Run(db.name+"/"+fname, twrapper)
		}
	}
}
