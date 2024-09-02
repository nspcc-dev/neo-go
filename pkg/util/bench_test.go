package util

import (
	"testing"
)

func BenchmarkUint256MarshalJSON(b *testing.B) {
	v := Uint256{0x01, 0x02, 0x03}

	for range b.N {
		_, _ = v.MarshalJSON()
	}
}
