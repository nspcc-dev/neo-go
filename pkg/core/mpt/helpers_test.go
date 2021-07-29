package mpt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToNibblesFromNibbles(t *testing.T) {
	check := func(t *testing.T, expected []byte) {
		actual := fromNibbles(toNibbles(expected))
		require.Equal(t, expected, actual)
	}
	t.Run("empty path", func(t *testing.T) {
		check(t, []byte{})
	})
	t.Run("non-empty path", func(t *testing.T) {
		check(t, []byte{0x01, 0xAC, 0x8d, 0x04, 0xFF})
	})
}
