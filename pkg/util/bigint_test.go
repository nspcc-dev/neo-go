package util

import (
	"math"
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
)

var testCases = []int64{
	0,
	1,
	-1,
	2,
	-2,
	127,
	-127,
	128,
	-128,
	129,
	-129,
	255,
	-255,
	256,
	-256,
	123456789,
	-123456789,
	-6777216,
	6777216,
}

func TestToBig(t *testing.T) {
	for _, tc := range testCases {
		x, _ := uint256.FromBig(big.NewInt(tc))
		assert.Equal(t, big.NewInt(tc), ToBig(x))
	}
}

func TestToInt64(t *testing.T) {
	min, _ := uint256.FromBig(big.NewInt(math.MinInt64))
	max, _ := uint256.FromBig(big.NewInt(math.MaxInt64))
	x := ToInt64(min)
	assert.Equal(t, int64(math.MinInt64), x)
	x = ToInt64(max)
	assert.Equal(t, int64(math.MaxInt64), x)

	v := uint256.NewInt(uint64(math.MaxInt64) + 1)
	ok := IsInt64(v)
	assert.False(t, ok)
	v = new(uint256.Int).Neg(uint256.NewInt(uint64(math.MaxInt64) + 2))
	ok = IsInt64(v)
	assert.False(t, ok)
}

func BenchmarkToInt64_1(b *testing.B) {
	min, _ := uint256.FromBig(big.NewInt(math.MinInt64))
	max, _ := uint256.FromBig(big.NewInt(math.MaxInt64))
	for i := 0; i < b.N; i++ {
		ToInt64(min)
		ToInt64(max)
	}
}
