package bigint

import (
	"math/big"
	"testing"
)

func BenchmarkToPreallocatedBytes(b *testing.B) {
	v := big.NewInt(100500)
	vn := big.NewInt(-100500)
	buf := make([]byte, 4)

	for i := 0; i < b.N; i++ {
		_ = ToPreallocatedBytes(v, buf[:0])
		_ = ToPreallocatedBytes(vn, buf[:0])
	}
}
