package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHardfork_Prev(t *testing.T) {
	var prev = HFAspidochelone
	for _, hf := range Hardforks[1:] {
		require.Equal(t, prev, hf.Prev())
		prev = hf
	}
}
