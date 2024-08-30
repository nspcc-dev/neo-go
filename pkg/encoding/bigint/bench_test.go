package bigint

import (
	"math/big"
	"testing"
)

func BenchmarkToPreallocatedBytes(b *testing.B) {
	v := big.NewInt(100500)
	vn := big.NewInt(-100500)
	buf := make([]byte, 4)

	for range b.N {
		_ = ToPreallocatedBytes(v, buf[:0])
		_ = ToPreallocatedBytes(vn, buf[:0])
	}
}
