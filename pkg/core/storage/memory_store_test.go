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
			for i := 0; i < count; i++ {
				require.NoError(t, ms.Put(append(searchPrefix, random.Bytes(10)...), random.Bytes(10)))
				require.NoError(t, ms.Put(append(badPrefix, random.Bytes(10)...), random.Bytes(10)))
			}

			t.ReportAllocs()
			t.ResetTimer()
			for n := 0; n < t.N; n++ {
				ms.Seek(SeekRange{Prefix: searchPrefix}, func(k, v []byte) bool { return false })
			}
		})
	}
}
