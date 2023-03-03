package util

import (
	"math"
	"math/big"

	"github.com/holiman/uint256"
)

var (
	minInt64 = new(uint256.Int).Neg(uint256.NewInt(math.MaxInt64 + 1))
	maxInt64 = new(uint256.Int).SetUint64(math.MaxInt64)
)

func ToBig(x *uint256.Int) *big.Int {
	if x.Sign() == 0 {
		return big.NewInt(0)
	}
	if x.Sign() > 0 {
		return x.ToBig()
	}
	b := new(uint256.Int).Neg(x).ToBig()
	return b.Neg(b)
}

func IsInt64(x *uint256.Int) bool {
	return !(x.Sgt(maxInt64) || x.Slt(minInt64))
}

func ToInt64(x *uint256.Int) int64 {
	var v int64
	if x.Sign() < 0 {
		v -= int64(new(uint256.Int).Neg(x).Uint64())
	} else {
		v = int64(x.Uint64())
	}
	return int64(v)
}
