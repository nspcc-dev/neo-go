package storage

import (
	"bytes"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	// Use the same set of kvs to test Seek with different prefix/start values.
	kvs := []KeyValue{
		{[]byte("10"), []byte("bar")},
		{[]byte("11"), []byte("bara")},
		{[]byte("20"), []byte("barb")},
		{[]byte("21"), []byte("barc")},
		{[]byte("22"), []byte("bard")},
		{[]byte("30"), []byte("bare")},
		{[]byte("31"), []byte("barf")},
	}
	for _, v := range kvs {
		require.NoError(t, s.Put(v.Key, v.Value))
	}

	check := func(t *testing.T, goodprefix, start []byte, goodkvs []KeyValue) {
		// Seek result expected to be sorted in an ascending way.
		sort.Slice(goodkvs, func(i, j int) bool {
			return bytes.Compare(goodkvs[i].Key, goodkvs[j].Key) < 0
		})

		actual := make([]KeyValue, 0, len(goodkvs))
		s.Seek(SeekRange{
			Prefix: goodprefix,
			Start:  start,
		}, func(k, v []byte) {
			actual = append(actual, KeyValue{
				Key:   slice.Copy(k),
				Value: slice.Copy(v),
			})
		})
		assert.Equal(t, goodkvs, actual)
	}

	t.Run("non-empty prefix, empty start", func(t *testing.T) {
		t.Run("good", func(t *testing.T) {
			// Given this prefix...
			goodprefix := []byte("2")
			// and empty start range...
			start := []byte{}
			// these pairs should be found.
			goodkvs := []KeyValue{
				kvs[2], // key = "20"
				kvs[3], // key = "21"
				kvs[4], // key = "22"
			}
			check(t, goodprefix, start, goodkvs)
		})
		t.Run("no matching items", func(t *testing.T) {
			goodprefix := []byte("0")
			start := []byte{}
			check(t, goodprefix, start, []KeyValue{})
		})
	})

	t.Run("non-empty prefix, non-empty start", func(t *testing.T) {
		t.Run("good", func(t *testing.T) {
			goodprefix := []byte("2")
			start := []byte("1") // start will be upended to goodprefix to start seek from
			goodkvs := []KeyValue{
				kvs[3], // key = "21"
				kvs[4], // key = "22"
			}
			check(t, goodprefix, start, goodkvs)
		})
		t.Run("no matching items", func(t *testing.T) {
			goodprefix := []byte("2")
			start := []byte("3") // start will be upended to goodprefix to start seek from
			check(t, goodprefix, start, []KeyValue{})
		})
	})

	t.Run("empty prefix, non-empty start", func(t *testing.T) {
		t.Run("good", func(t *testing.T) {
			goodprefix := []byte{}
			start := []byte("21")
			goodkvs := []KeyValue{
				kvs[3], // key = "21"
				kvs[4], // key = "22"
				kvs[5], // key = "30"
				kvs[6], // key = "31"
			}
			check(t, goodprefix, start, goodkvs)
		})
		t.Run("no matching items", func(t *testing.T) {
			goodprefix := []byte{}
			start := []byte("32")
			check(t, goodprefix, start, []KeyValue{})
		})
	})

	t.Run("empty prefix, empty start", func(t *testing.T) {
		goodprefix := []byte{}
		start := []byte{}
		goodkvs := make([]KeyValue, len(kvs))
		copy(goodkvs, kvs)
		check(t, goodprefix, start, goodkvs)
	})

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
