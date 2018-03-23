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
