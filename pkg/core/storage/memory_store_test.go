package storage

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/stretchr/testify/require"
)

func newMemoryStoreForTesting(t testing.TB) Store {
	return NewMemoryStore()
}

func BenchmarkMemorySeek(t *testing.B) {
	for count := 10; count <= 10000; count *= 10 {
		t.Run(fmt.Sprintf("%dElements", count), func(t *testing.B) {
			ms := NewMemoryStore()
			var (
				searchPrefix = []byte{1}
				badPrefix    = []byte{2}
			)
			ts := NewMemCachedStore(ms)
			for range count {
				ts.Put(append(searchPrefix, random.Bytes(10)...), random.Bytes(10))
				ts.Put(append(badPrefix, random.Bytes(10)...), random.Bytes(10))
			}
			_, err := ts.PersistSync()
			require.NoError(t, err)

			t.ReportAllocs()
			t.ResetTimer()
			for range t.N {
				ms.Seek(SeekRange{Prefix: searchPrefix}, func(k, v []byte) bool { return false })
			}
		})
	}
}
