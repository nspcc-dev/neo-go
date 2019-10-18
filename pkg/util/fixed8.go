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

var errInvalidString = errors.New("fixed8 must satisfy following regex \\d+(\\.\\d{1,8})?")

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
		buf.WriteString(strings.TrimRight(str, "0"))
	}
	return buf.String()
}

// FloatValue returns the original value representing Fixed8 as float64.
func (f Fixed8) FloatValue() float64 {
	return float64(f) / decimals
}

// Int64Value returns the original value representing Fixed8 as int64.
func (f Fixed8) Int64Value() int64 {
	return int64(f) / decimals
}

// Fixed8FromInt64 returns a new Fixed8 type multiplied by decimals.
func Fixed8FromInt64(val int64) Fixed8 {
	return Fixed8(decimals * val)
}

// Fixed8FromFloat returns a new Fixed8 type multiplied by decimals.
func Fixed8FromFloat(val float64) Fixed8 {
	return Fixed8(int64(decimals * val))
}

// Fixed8FromString parses s which must be a fixed point number
// with precision up to 10^-8
func Fixed8FromString(s string) (Fixed8, error) {
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
		p, err := Fixed8FromString(s)
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

// MarshalJSON implements the json marshaller interface.
func (f Fixed8) MarshalJSON() ([]byte, error) {
	return []byte(`"` + f.String() + `"`), nil
}

// Satoshi defines the value of a 'Satoshi'.
func Satoshi() Fixed8 {
	return Fixed8(1)
}

// Div implements Fixd8 division operator.
func (f Fixed8) Div(i int64) Fixed8 {
	return f / Fixed8FromInt64(i)
}

// Add implements Fixd8 addition operator.
func (f Fixed8) Add(g Fixed8) Fixed8 {
	return f + g
}

// Sub implements Fixd8 subtraction operator.
func (f Fixed8) Sub(g Fixed8) Fixed8 {
	return f - g
}

// LessThan implements Fixd8 < operator.
func (f Fixed8) LessThan(g Fixed8) bool {
	return f < g
}

// GreaterThan implements Fixd8 < operator.
func (f Fixed8) GreaterThan(g Fixed8) bool {
	return f > g
}

// Equal implements Fixd8 == operator.
func (f Fixed8) Equal(g Fixed8) bool {
	return f == g
}

// CompareTo returns the difference between the f and g.
// difference < 0 implies f < g.
// difference = 0 implies f = g.
// difference > 0 implies f > g.
func (f Fixed8) CompareTo(g Fixed8) int {
	return int(f - g)
}
