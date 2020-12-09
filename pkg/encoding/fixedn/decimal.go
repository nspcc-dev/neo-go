package fixedn

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const maxAllowedPrecision = 16

// ErrInvalidFormat is returned when decimal format is invalid.
var ErrInvalidFormat = errors.New("invalid decimal format")

var _pow10 []*big.Int

func init() {
	var p = int64(1)
	for i := 0; i <= maxAllowedPrecision; i++ {
		_pow10 = append(_pow10, big.NewInt(p))
		p *= 10
	}
}

func pow10(n int) *big.Int {
	last := len(_pow10) - 1
	if n <= last {
		return _pow10[n]
	}
	p := new(big.Int)
	p.Mul(_pow10[last], _pow10[1])
	for i := last + 1; i < n; i++ {
		p.Mul(p, _pow10[1])
	}
	return p
}

// ToString converts big decimal with specified precision to string.
func ToString(bi *big.Int, precision int) string {
	var dp, fp big.Int
	dp.QuoRem(bi, pow10(precision), &fp)

	var s = dp.String()
	if fp.Sign() == 0 {
		return s
	}
	frac := fp.Uint64()
	trimmed := 0
	for ; frac%10 == 0; frac /= 10 {
		trimmed++
	}
	return s + "." + fmt.Sprintf("%0"+strconv.FormatUint(uint64(precision-trimmed), 10)+"d", frac)
}

// FromString converts string to a big decimal with specified precision.
func FromString(s string, precision int) (*big.Int, error) {
	parts := strings.SplitN(s, ".", 2)
	bi, ok := new(big.Int).SetString(parts[0], 10)
	if !ok {
		return nil, ErrInvalidFormat
	}
	bi.Mul(bi, pow10(precision))
	if len(parts) == 1 {
		return bi, nil
	}

	if len(parts[1]) > precision {
		return nil, ErrInvalidFormat
	}
	fp, ok := new(big.Int).SetString(parts[1], 10)
	if !ok {
		return nil, ErrInvalidFormat
	}
	fp.Mul(fp, pow10(precision-len(parts[1])))
	if bi.Sign() == -1 {
		return bi.Sub(bi, fp), nil
	}
	return bi.Add(bi, fp), nil
}
