package mpt

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
)

func benchmarkBytes(b *testing.B, n Node) {
	inv := n.(interface{ invalidateCache() })
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		inv.invalidateCache()
		_ = n.Bytes()
	}
}

func BenchmarkBytes(b *testing.B) {
	b.Run("extension", func(b *testing.B) {
		n := NewExtensionNode(random.Bytes(10), NewLeafNode(random.Bytes(10)))
		benchmarkBytes(b, n)
	})
	b.Run("leaf", func(b *testing.B) {
		n := NewLeafNode(make([]byte, 15))
		benchmarkBytes(b, n)
	})
	b.Run("hash", func(b *testing.B) {
		n := NewHashNode(random.Uint256())
		benchmarkBytes(b, n)
	})
	b.Run("branch", func(b *testing.B) {
		n := NewBranchNode()
		n.Children[0] = NewLeafNode(random.Bytes(10))
		n.Children[4] = NewLeafNode(random.Bytes(10))
		n.Children[7] = NewLeafNode(random.Bytes(10))
		n.Children[8] = NewLeafNode(random.Bytes(10))
	})
}
