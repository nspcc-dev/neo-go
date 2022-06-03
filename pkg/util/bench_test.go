package util

import (
	"testing"
)

func BenchmarkUint256MarshalJSON(b *testing.B) {
	v := Uint256{0x01, 0x02, 0x03}

	for i := 0; i < b.N; i++ {
		_, _ = v.MarshalJSON()
	}
}
