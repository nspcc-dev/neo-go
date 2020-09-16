package hash

import (
	"math/rand"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func BenchmarkMerkle(t *testing.B) {
	var err error
	var hashes = make([]util.Uint256, 100000)
	var h = make([]byte, 32)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range hashes {
		r.Read(h)
		hashes[i], err = util.Uint256DecodeBytesBE(h)
		require.NoError(t, err)
	}

	t.Run("NewMerkleTree", func(t *testing.B) {
		t.ResetTimer()
		for n := 0; n < t.N; n++ {
			tr, err := NewMerkleTree(hashes)
			require.NoError(t, err)
			_ = tr.Root()
		}
	})
	t.Run("CalcMerkleRoot", func(t *testing.B) {
		t.ResetTimer()
		for n := 0; n < t.N; n++ {
			_ = CalcMerkleRoot(hashes)
		}
	})
}
