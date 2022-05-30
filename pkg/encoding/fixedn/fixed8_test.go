package fixedn

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestFixed8FromInt64(t *testing.T) {
	values := []int64{9000, 100000000, 5, 10945, -42}

	for _, val := range values {
		assert.Equal(t, Fixed8(val*decimals), Fixed8FromInt64(val))
		assert.Equal(t, val, Fixed8FromInt64(val).IntegralValue())
		assert.Equal(t, int32(0), Fixed8FromInt64(val).FractionalValue())
	}
}

func TestFixed8Add(t *testing.T) {
	a := Fixed8FromInt64(1)
	b := Fixed8FromInt64(2)

	c := a.Add(b)
	expected := int64(3)
	assert.Equal(t, strconv.FormatInt(expected, 10), c.String())
}

func TestFixed8Sub(t *testing.T) {
	a := Fixed8FromInt64(42)
	b := Fixed8FromInt64(34)

	c := a.Sub(b)
	assert.Equal(t, int64(8), c.IntegralValue())
	assert.Equal(t, int32(0), c.FractionalValue())
}

func TestFixed8FromFloat(t *testing.T) {
	inputs := []float64{12.98, 23.87654333, 100.654322, 456789.12345665, -3.14159265}

	for _, val := range inputs {
		assert.Equal(t, Fixed8(val*decimals), Fixed8FromFloat(val))
		assert.Equal(t, val, Fixed8FromFloat(val).FloatValue())
		trunc := math.Trunc(val)
		rem := (val - trunc) * decimals
		assert.Equal(t, int64(trunc), Fixed8FromFloat(val).IntegralValue())
		assert.Equal(t, int32(math.Round(rem)), Fixed8FromFloat(val).FractionalValue())
	}
}

func TestFixed8FromString(t *testing.T) {
	// Fixed8FromString works correctly with integers
	ivalues := []string{"9000", "100000000", "5", "10945", "20.45", "0.00000001", "-42"}
	for _, val := range ivalues {
		n, err := Fixed8FromString(val)
		assert.Nil(t, err)
		assert.Equal(t, val, n.String())
	}

	// Fixed8FromString parses number with maximal precision
	val := "123456789.12345678"
	n, err := Fixed8FromString(val)
	assert.Nil(t, err)
	assert.Equal(t, Fixed8(12345678912345678), n)

	// Fixed8FromString parses number with non-maximal precision
	val = "901.2341"
	n, err = Fixed8FromString(val)
	assert.Nil(t, err)
	assert.Equal(t, Fixed8(90123410000), n)

	// Fixed8FromString with errors
	val = "90n1"
	_, err = Fixed8FromString(val)
	assert.Error(t, err)

	val = "90.1s"
	_, err = Fixed8FromString(val)
	assert.Error(t, err)
}

func TestSatoshi(t *testing.T) {
	satoshif8 := Satoshi()
	assert.Equal(t, "0.00000001", satoshif8.String())
}

func TestFixed8UnmarshalJSON(t *testing.T) {
	var testCases = []float64{
		123.45,
		-123.45,
	}

	for _, fl := range testCases {
		str := strconv.FormatFloat(fl, 'g', -1, 64)
		expected, _ := Fixed8FromString(str)

		// UnmarshalJSON should decode floats
		var u1 Fixed8
		s, _ := json.Marshal(fl)
		assert.Nil(t, json.Unmarshal(s, &u1))
		assert.Equal(t, expected, u1)

		// UnmarshalJSON should decode strings
		var u2 Fixed8
		s, _ = json.Marshal(str)
		assert.Nil(t, json.Unmarshal(s, &u2))
		assert.Equal(t, expected, u2)
	}

	errorCases := []string{
		`"123.u"`,
		"13.j",
	}

	for _, tc := range errorCases {
		var u Fixed8
		assert.Error(t, u.UnmarshalJSON([]byte(tc)))
	}
}

func TestFixed8_Unmarshal(t *testing.T) {
	var expected = Fixed8(223719420)
	var cases = []string{"2.2371942", `"2.2371942"`} // this easily gives 223719419 if interpreted as float

	for _, c := range cases {
		var u1, u2 Fixed8
		assert.Nil(t, json.Unmarshal([]byte(c), &u1))
		assert.Equal(t, expected, u1)
		assert.Nil(t, yaml.Unmarshal([]byte(c), &u2))
		assert.Equal(t, expected, u2)
	}
}

func TestFixed8_MarshalJSON(t *testing.T) {
	u, err := Fixed8FromString("123.4")
	assert.NoError(t, err)

	s, err := json.Marshal(u)
	assert.NoError(t, err)
	assert.Equal(t, []byte(`"123.4"`), s)
}

func TestFixed8_UnmarshalYAML(t *testing.T) {
	u, err := Fixed8FromString("123.4")
	assert.NoError(t, err)

	s, err := yaml.Marshal(u)
	assert.NoError(t, err)
	assert.Equal(t, []byte("\"123.4\"\n"), s) // yaml marshaler inserts LF at the end

	var f Fixed8
	assert.NoError(t, yaml.Unmarshal([]byte(`"123.4"`), &f))
	assert.Equal(t, u, f)
}

func TestFixed8_Arith(t *testing.T) {
	u1 := Fixed8FromInt64(3)
	u2 := Fixed8FromInt64(8)

	assert.True(t, u1.LessThan(u2))
	assert.True(t, u2.GreaterThan(u1))
	assert.True(t, u1.Equal(u1))
	assert.NotZero(t, u1.CompareTo(u2))
	assert.Zero(t, u1.CompareTo(u1))
	assert.EqualValues(t, Fixed8(2), u2.Div(3))
}

func TestFixed8_Serializable(t *testing.T) {
	a := Fixed8(0x0102030405060708)

	testserdes.EncodeDecodeBinary(t, &a, new(Fixed8))
}
