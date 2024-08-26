package fixedn

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

const (
	precision = 8
	decimals  = 100000000
)

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

// IntegralValue returns an integer part of the original value representing
// Fixed8 as int64.
func (f Fixed8) IntegralValue() int64 {
	return int64(f) / decimals
}

// FractionalValue returns a decimal part of the original value. It has the same
// sign as f, so that f = f.IntegralValue() + f.FractionalValue().
func (f Fixed8) FractionalValue() int32 {
	return int32(int64(f) % decimals)
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
// with precision up to 10^-8.
func Fixed8FromString(s string) (Fixed8, error) {
	num, err := FromString(s, precision)
	if err != nil {
		return 0, err
	}
	return Fixed8(num.Int64()), err
}

// UnmarshalJSON implements the json unmarshaller interface.
func (f *Fixed8) UnmarshalJSON(data []byte) error {
	if len(data) > 2 {
		if data[0] == '"' && data[len(data)-1] == '"' {
			data = data[1 : len(data)-1]
		}
	}
	return f.setFromString(string(data))
}

// UnmarshalYAML implements the yaml unmarshaler interface.
func (f *Fixed8) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	return f.setFromString(s)
}

func (f *Fixed8) setFromString(s string) error {
	p, err := Fixed8FromString(s)
	if err != nil {
		return err
	}
	*f = p
	return nil
}

// MarshalJSON implements the json marshaller interface.
func (f Fixed8) MarshalJSON() ([]byte, error) {
	return []byte(`"` + f.String() + `"`), nil
}

// MarshalYAML implements the yaml marshaller interface.
func (f Fixed8) MarshalYAML() (any, error) {
	return f.String(), nil
}

// DecodeBinary implements the io.Serializable interface.
func (f *Fixed8) DecodeBinary(r *io.BinReader) {
	*f = Fixed8(r.ReadU64LE())
}

// EncodeBinary implements the io.Serializable interface.
func (f *Fixed8) EncodeBinary(w *io.BinWriter) {
	w.WriteU64LE(uint64(*f))
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

// Compare performs three-way comparison between f and g.
//   - -1 implies f < g.
//   - 0 implies f = g.
//   - 1 implies f > g.
func (f Fixed8) Compare(g Fixed8) int {
	return cmp.Compare(f, g)
}
