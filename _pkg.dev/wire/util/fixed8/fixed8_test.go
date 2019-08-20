package fixed8

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixed8Value(t *testing.T) {

	input := int64(12)
	assert.Equal(t, float64(input), FromInt(input).Value())

}
func TestFixed8Add(t *testing.T) {

	a := FromInt(1)
	b := FromInt(2)

	c := a.Add(b)
	expected := float64(3)
	assert.Equal(t, expected, c.Value())

}
func TestFixed8AddRecursive(t *testing.T) {

	a := FromInt(1)
	sum := int64(1)

	for i := int64(2); i <= 10; i++ {

		sum += i
		b := FromInt(i)
		c := a.Add(b)
		a = c // 1 + 2 + 3 ... + 10
	}
	assert.Equal(t, float64(sum), a.Value())

}

func TestFromInt(t *testing.T) {

	inputs := []int64{12, 23, 100, 456789}

	for _, val := range inputs {
		assert.Equal(t, Fixed8(val*decimals), FromInt(val))
		assert.Equal(t, float64(val), FromInt(val).Value())
	}

	for _, val := range inputs {
		valString := strconv.FormatInt(val, 10)
		assert.Equal(t, valString, FromInt(val).String())
	}

}
func TestFromFloat(t *testing.T) {
	inputs := []float64{12.98, 23.87654333, 100.654322, 456789.12345665}

	for _, val := range inputs {
		assert.Equal(t, Fixed8(val*decimals), FromFloat(val))
		assert.Equal(t, float64(val), FromFloat(val).Value())
	}
}
func TestFromString(t *testing.T) {
	inputs := []string{"9000", "100000000", "5", "10945", "20.45", "0.00000001"}

	for _, val := range inputs {

		n, err := FromString(val)
		assert.Nil(t, err)
		assert.Equal(t, val, n.String())

	}

	val := "123456789.12345678"
	n, err := FromString(val)
	assert.Nil(t, err)
	assert.Equal(t, Fixed8(12345678912345678), n)

	val = "901.2341"
	n, err = FromString(val)
	assert.Nil(t, err)
	assert.Equal(t, Fixed8(90123410000), n)
}
