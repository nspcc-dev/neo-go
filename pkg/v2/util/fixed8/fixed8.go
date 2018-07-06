package fixed8

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	decimals = 100000000
)

var errInvalidString = errors.New("Fixed8 must satisfy following regex \\d+(\\.\\d{1,8})?")

// Fixed8 represents a fixed-point number with precision 10^-8.
type Fixed8 int64

// String implements the Stringer interface.
func (f Fixed8) String() string {
	val := f.Value()
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// Value returns the original value representing the Fixed8.
func (f Fixed8) Value() float64 {
	return float64(f) / float64(decimals)
}
func (f Fixed8) Add(val Fixed8) Fixed8 {
	a := int64(f.Value())
	b := int64(val.Value())
	c := a + b
	return FromInt(c)
}
func (f Fixed8) Sub(val Fixed8) Fixed8 {
	a := int64(f.Value())
	b := int64(val.Value())
	c := a - b
	return FromInt(c)
}

func FromInt(val int64) Fixed8 {
	return Fixed8(val * decimals)
}

func FromFloat(val float64) Fixed8 {
	return Fixed8(val * decimals)
}

func FromString(val string) (Fixed8, error) {
	res, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("Failed at parsing string %s", val)
	}
	return FromFloat(res), nil
}
