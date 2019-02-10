package util

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const (
	precision = 8
	decimals  = 100000000
)

var errInvalidString = errors.New("Fixed8 must satisfy following regex \\d+(\\.\\d{1,8})?")

// Fixed8 represents a fixed-point number with precision 10^-8.
type Fixed8 int64

// String implements the Stringer interface.
func (f Fixed8) String() string {
	buf := new(strings.Builder)
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
func (f Fixed8) Value() int {
	return int(f) / decimals
}

// NewFixed8 return a new Fixed8 type multiplied by decimals.
func NewFixed8(val int) Fixed8 {
	return Fixed8(decimals * val)
}

// Fixed8DecodeString parses s which must be a fixed point number
// with precision up to 10^-8
func Fixed8DecodeString(s string) (Fixed8, error) {
	parts := strings.SplitN(s, ".", 2)
	ip, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, errInvalidString
	} else if len(parts) == 1 {
		return Fixed8(ip * decimals), nil
	}

	fp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || fp >= decimals {
		return 0, errInvalidString
	}
	for i := len(parts[1]); i < precision; i++ {
		fp *= 10
	}
	if ip < 0 {
		return Fixed8(ip*decimals - fp), nil
	}
	return Fixed8(ip*decimals + fp), nil
}

// UnmarshalJSON implements the json unmarshaller interface.
func (f *Fixed8) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		p, err := Fixed8DecodeString(s)
		if err != nil {
			return err
		}
		*f = p
		return nil
	}

	var fl float64
	if err := json.Unmarshal(data, &fl); err != nil {
		return err
	}

	*f = Fixed8(decimals * fl)
	return nil
}

// Size returns the size in number of bytes of Fixed8.
func (f *Fixed8) Size() int {
	return 8
}

// MarshalJSON implements the json marshaller interface.
func (f Fixed8) MarshalJSON() ([]byte, error) {
	return []byte(`"` + f.String() + `"`), nil
}

// Satoshi defines the value of a 'Satoshi'.
func Satoshi() Fixed8 {
	return NewFixed8(1)
}

// Div implements Fixd8 division operator.
func (f Fixed8) Div(i int) Fixed8 {
	return NewFixed8(f.Value() / i)
}

// Add implements Fixd8 addition operator.
func (f Fixed8) Add(g Fixed8) Fixed8 {
	return NewFixed8(f.Value() + g.Value())
}

// Sub implements Fixd8 subtraction operator.
func (f Fixed8) Sub(g Fixed8) Fixed8 {
	return NewFixed8(f.Value() - g.Value())
}
