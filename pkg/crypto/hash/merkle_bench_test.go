package hash_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func BenchmarkMerkle(t *testing.B) {
	var hashes = make([]util.Uint256, 100000)
	for i := range hashes {
		hashes[i] = random.Uint256()
	}

	t.Run("NewMerkleTree", func(t *testing.B) {
		t.ResetTimer()
		for range t.N {
			tr, err := hash.NewMerkleTree(hashes)
			require.NoError(t, err)
			_ = tr.Root()
		}
	})
	t.Run("CalcMerkleRoot", func(t *testing.B) {
		t.ResetTimer()
		for range t.N {
			_ = hash.CalcMerkleRoot(hashes)
		}
	})
}
