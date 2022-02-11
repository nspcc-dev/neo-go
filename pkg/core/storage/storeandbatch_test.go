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

	check := func(t *testing.T, goodprefix, start []byte, goodkvs []KeyValue, backwards bool, cont func(k, v []byte) bool) {
		// Seek result expected to be sorted in an ascending (for forwards seeking) or descending (for backwards seeking) way.
		cmpFunc := func(i, j int) bool {
			return bytes.Compare(goodkvs[i].Key, goodkvs[j].Key) < 0
		}
		if backwards {
			cmpFunc = func(i, j int) bool {
				return bytes.Compare(goodkvs[i].Key, goodkvs[j].Key) > 0
			}
		}
		sort.Slice(goodkvs, cmpFunc)

		rng := SeekRange{
			Prefix: goodprefix,
			Start:  start,
		}
		if backwards {
			rng.Backwards = true
		}
		actual := make([]KeyValue, 0, len(goodkvs))
		s.Seek(rng, func(k, v []byte) bool {
			actual = append(actual, KeyValue{
				Key:   slice.Copy(k),
				Value: slice.Copy(v),
			})
			if cont == nil {
				return true
			}
			return cont(k, v)
		})
		assert.Equal(t, goodkvs, actual)
	}

	t.Run("non-empty prefix, empty start", func(t *testing.T) {
		t.Run("forwards", func(t *testing.T) {
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
				check(t, goodprefix, start, goodkvs, false, nil)
			})
			t.Run("no matching items", func(t *testing.T) {
				goodprefix := []byte("0")
				start := []byte{}
				check(t, goodprefix, start, []KeyValue{}, false, nil)
			})
			t.Run("early stop", func(t *testing.T) {
				// Given this prefix...
				goodprefix := []byte("2")
				// and empty start range...
				start := []byte{}
				// these pairs should be found.
				goodkvs := []KeyValue{
					kvs[2], // key = "20"
					kvs[3], // key = "21"
				}
				check(t, goodprefix, start, goodkvs, false, func(k, v []byte) bool {
					return string(k) < "21"
				})
			})
		})

		t.Run("backwards", func(t *testing.T) {
			t.Run("good", func(t *testing.T) {
				goodprefix := []byte("2")
				start := []byte{}
				goodkvs := []KeyValue{
					kvs[4], // key = "22"
					kvs[3], // key = "21"
					kvs[2], // key = "20"
				}
				check(t, goodprefix, start, goodkvs, true, nil)
			})
			t.Run("no matching items", func(t *testing.T) {
				goodprefix := []byte("0")
				start := []byte{}
				check(t, goodprefix, start, []KeyValue{}, true, nil)
			})
			t.Run("early stop", func(t *testing.T) {
				goodprefix := []byte("2")
				start := []byte{}
				goodkvs := []KeyValue{
					kvs[4], // key = "22"
					kvs[3], // key = "21"
				}
				check(t, goodprefix, start, goodkvs, true, func(k, v []byte) bool {
					return string(k) > "21"
				})
			})
		})
	})

	t.Run("non-empty prefix, non-empty start", func(t *testing.T) {
		t.Run("forwards", func(t *testing.T) {
			t.Run("good", func(t *testing.T) {
				goodprefix := []byte("2")
				start := []byte("1") // start will be appended to goodprefix to start seek from
				goodkvs := []KeyValue{
					kvs[3], // key = "21"
					kvs[4], // key = "22"
				}
				check(t, goodprefix, start, goodkvs, false, nil)
			})
			t.Run("no matching items", func(t *testing.T) {
				goodprefix := []byte("2")
				start := []byte("3") // start is more than all keys prefixed by '2'.
				check(t, goodprefix, start, []KeyValue{}, false, nil)
			})
			t.Run("early stop", func(t *testing.T) {
				goodprefix := []byte("2")
				start := []byte("0") // start will be appended to goodprefix to start seek from
				goodkvs := []KeyValue{
					kvs[2], // key = "20"
					kvs[3], // key = "21"
				}
				check(t, goodprefix, start, goodkvs, false, func(k, v []byte) bool {
					return string(k) < "21"
				})
			})
		})
		t.Run("backwards", func(t *testing.T) {
			t.Run("good", func(t *testing.T) {
				goodprefix := []byte("2")
				start := []byte("1") // start will be appended to goodprefix to start seek from
				goodkvs := []KeyValue{
					kvs[3], // key = "21"
					kvs[2], // key = "20"
				}
				check(t, goodprefix, start, goodkvs, true, nil)
			})
			t.Run("no matching items", func(t *testing.T) {
				goodprefix := []byte("2")
				start := []byte(".") // start is less than all keys prefixed by '2'.
				check(t, goodprefix, start, []KeyValue{}, true, nil)
			})
			t.Run("early stop", func(t *testing.T) {
				goodprefix := []byte("2")
				start := []byte("2") // start will be appended to goodprefix to start seek from
				goodkvs := []KeyValue{
					kvs[4], // key = "24"
					kvs[3], // key = "21"
				}
				check(t, goodprefix, start, goodkvs, true, func(k, v []byte) bool {
					return string(k) > "21"
				})
			})
		})
	})

	t.Run("empty prefix, non-empty start", func(t *testing.T) {
		t.Run("forwards", func(t *testing.T) {
			t.Run("good", func(t *testing.T) {
				goodprefix := []byte{}
				start := []byte("21")
				goodkvs := []KeyValue{
					kvs[3], // key = "21"
					kvs[4], // key = "22"
					kvs[5], // key = "30"
					kvs[6], // key = "31"
				}
				check(t, goodprefix, start, goodkvs, false, nil)
			})
			t.Run("no matching items", func(t *testing.T) {
				goodprefix := []byte{}
				start := []byte("32") // start is more than all keys.
				check(t, goodprefix, start, []KeyValue{}, false, nil)
			})
			t.Run("early stop", func(t *testing.T) {
				goodprefix := []byte{}
				start := []byte("21")
				goodkvs := []KeyValue{
					kvs[3], // key = "21"
					kvs[4], // key = "22"
					kvs[5], // key = "30"
				}
				check(t, goodprefix, start, goodkvs, false, func(k, v []byte) bool {
					return string(k) < "30"
				})
			})
		})
		t.Run("backwards", func(t *testing.T) {
			t.Run("good", func(t *testing.T) {
				goodprefix := []byte{}
				start := []byte("21")
				goodkvs := []KeyValue{
					kvs[3], // key = "21"
					kvs[2], // key = "20"
					kvs[1], // key = "11"
					kvs[0], // key = "10"
				}
				check(t, goodprefix, start, goodkvs, true, nil)
			})
			t.Run("no matching items", func(t *testing.T) {
				goodprefix := []byte{}
				start := []byte("0") // start is less than all keys.
				check(t, goodprefix, start, []KeyValue{}, true, nil)
			})
			t.Run("early stop", func(t *testing.T) {
				goodprefix := []byte{}
				start := []byte("21")
				goodkvs := []KeyValue{
					kvs[3], // key = "21"
					kvs[2], // key = "20"
					kvs[1], // key = "11"
				}
				check(t, goodprefix, start, goodkvs, true, func(k, v []byte) bool {
					return string(k) > "11"
				})
			})
		})
	})

	t.Run("empty prefix, empty start", func(t *testing.T) {
		goodprefix := []byte{}
		start := []byte{}
		goodkvs := make([]KeyValue, len(kvs))
		copy(goodkvs, kvs)
		t.Run("forwards", func(t *testing.T) {
			t.Run("good", func(t *testing.T) {
				check(t, goodprefix, start, goodkvs, false, nil)
			})
			t.Run("early stop", func(t *testing.T) {
				goodkvs := []KeyValue{
					kvs[0], // key = "10"
					kvs[1], // key = "11"
					kvs[2], // key = "20"
					kvs[3], // key = "21"
				}
				check(t, goodprefix, start, goodkvs, false, func(k, v []byte) bool {
					return string(k) < "21"
				})
			})
		})
		t.Run("backwards", func(t *testing.T) {
			t.Run("good", func(t *testing.T) {
				check(t, goodprefix, start, goodkvs, true, nil)
			})
			t.Run("early stop", func(t *testing.T) {
				goodkvs := []KeyValue{
					kvs[6], // key = "31"
					kvs[5], // key = "30"
					kvs[4], // key = "22"
					kvs[3], // key = "21"
				}
				check(t, goodprefix, start, goodkvs, true, func(k, v []byte) bool {
					return string(k) > "21"
				})
			})
		})
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

func testStoreSeekGC(t *testing.T, s Store) {
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
	err := s.SeekGC(SeekRange{Prefix: []byte("1")}, func(k, v []byte) bool {
		return true
	})
	require.NoError(t, err)
	for i := range kvs {
		_, err = s.Get(kvs[i].Key)
		require.NoError(t, err)
	}
	err = s.SeekGC(SeekRange{Prefix: []byte("3")}, func(k, v []byte) bool {
		return false
	})
	require.NoError(t, err)
	for i := range kvs[:5] {
		_, err = s.Get(kvs[i].Key)
		require.NoError(t, err)
	}
	for _, kv := range kvs[5:] {
		_, err = s.Get(kv.Key)
		require.Error(t, err)
	}
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
		testStorePutBatchWithDelete, testStoreSeekGC}
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
