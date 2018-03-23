package util

import (
	"bytes"
	"strconv"
)

const decimals = 100000000

// Fixed8 represents a fixed-point number with precision 10^-8.
type Fixed8 int64

// String implements the Stringer interface.
func (f Fixed8) String() string {
	buf := new(bytes.Buffer)
	val := int64(f)
	if val < 0 {
		buf.WriteRune('-')
		val = -val
	}
	str := strconv.FormatInt(val/decimals, 10)
	buf.WriteString(str)
	val %= decimals
	if val > 0 {
		buf.WriteRune('.')
		str = strconv.FormatInt(val, 10)
		for i := len(str); i < 8; i++ {
			buf.WriteRune('0')
		}
		buf.WriteString(str)
	}
	return buf.String()
}

// Value returns the original value representing the Fixed8.
func (f Fixed8) Value() int64 {
	return int64(f) / int64(decimals)
}

// NewFixed8 return a new Fixed8 type multiplied by decimals.
func NewFixed8(val int) Fixed8 {
	return Fixed8(decimals * val)
}
