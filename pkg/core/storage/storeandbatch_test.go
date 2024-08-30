package storage

import (
	"bytes"
	"reflect"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dbSetup struct {
	name   string
	create func(testing.TB) Store
}

type dbTestFunction func(*testing.T, Store)

func testStoreGetNonExistent(t *testing.T, s Store) {
	key := []byte("sparse")

	_, err := s.Get(key)
	assert.Equal(t, err, ErrKeyNotFound)
}

func pushSeekDataSet(t *testing.T, s Store) []KeyValue {
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
	up := NewMemCachedStore(s)
	for _, v := range kvs {
		up.Put(v.Key, v.Value)
	}
	_, err := up.PersistSync()
	require.NoError(t, err)
	return kvs
}

func testStoreSeek(t *testing.T, s Store) {
	kvs := pushSeekDataSet(t, s)
	check := func(t *testing.T, goodprefix, start []byte, goodkvs []KeyValue, backwards bool, cont func(k, v []byte) bool) {
		// Seek result expected to be sorted in an ascending (for forwards seeking) or descending (for backwards seeking) way.
		var cmpFunc = getCmpFunc(backwards)
		slices.SortFunc(goodkvs, func(a, b KeyValue) int {
			return cmpFunc(a.Key, b.Key)
		})

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
				Key:   bytes.Clone(k),
				Value: bytes.Clone(v),
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
}

func testStoreSeekGC(t *testing.T, s Store) {
	kvs := pushSeekDataSet(t, s)
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
	var tests = []dbTestFunction{testStoreGetNonExistent, testStoreSeek,
		testStoreSeekGC}
	for _, db := range DBs {
		for _, test := range tests {
			s := db.create(t)
			twrapper := func(t *testing.T) {
				test(t, s)
			}
			fname := runtime.FuncForPC(reflect.ValueOf(test).Pointer()).Name()
			t.Run(db.name+"/"+fname, twrapper)
			require.NoError(t, s.Close())
		}
	}
}
