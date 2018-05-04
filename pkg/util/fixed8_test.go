package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFixed8(t *testing.T) {
	values := []int{9000, 100000000, 5, 10945}

	for _, val := range values {
		assert.Equal(t, Fixed8(val*decimals), NewFixed8(val))
		assert.Equal(t, int64(val), NewFixed8(val).Value())
	}
}

func TestFixed8DecodeString(t *testing.T) {
	// Fixed8DecodeString works correctly with integers
	ivalues := []string{"9000", "100000000", "5", "10945"}
	for _, val:= range ivalues {
		n, err := Fixed8DecodeString(val)
		assert.Nil(t, err)
		assert.Equal(t, val, n.String())
	}

	// Fixed8DecodeString parses number with maximal precision
	val := "123456789.12345678"
	n, err := Fixed8DecodeString(val)
	assert.Nil(t, err)
	assert.Equal(t, Fixed8(12345678912345678), n)

	// Fixed8DecodeString parses number with non-maximal precision
	val = "901.2341"
	n, err = Fixed8DecodeString(val)
	assert.Nil(t, err)
	assert.Equal(t, Fixed8(90123410000), n)
}
