package flags

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEachName(t *testing.T) {
	expected := "*one*two*three"
	actual := ""

	eachName(" one,two ,three", func(s string) {
		actual += "*" + s
	})
	require.Equal(t, expected, actual)
}
