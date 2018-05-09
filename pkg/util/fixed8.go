package util

import (
	"bytes"
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

// Fixed8DecodeString parses s which must be a fixed point number
// with precision up to 10^-8
func Fixed8DecodeString(s string) (Fixed8, error) {
	parts := strings.SplitN(s, ".", 2)
	ip, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, errInvalidString
	} else if len(parts) == 1 {
		return NewFixed8(ip), nil
	}

	fp, err := strconv.Atoi(parts[1])
	if err != nil || fp >= decimals {
		return 0, errInvalidString
	}
	for i := len(parts[1]); i < precision; i++ {
		fp *= 10
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
