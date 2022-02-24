package binding

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewOverrideFromString(t *testing.T) {
	testCases := []struct {
		expected Override
		value    string
	}{
		{Override{"import.com/pkg", "pkg.Type"}, "import.com/pkg.Type"},
		{Override{"", "map[int]int"}, "map[int]int"},
		{Override{"", "[]int"}, "[]int"},
		{Override{"", "map[int][]int"}, "map[int][]int"},
		{Override{"import.com/pkg", "map[int]pkg.Type"}, "map[int]import.com/pkg.Type"},
		{Override{"import.com/pkg", "[]pkg.Type"}, "[]import.com/pkg.Type"},
		{Override{"import.com/pkg", "map[int]*pkg.Type"}, "map[int]*import.com/pkg.Type"},
		{Override{"import.com/pkg", "[]*pkg.Type"}, "[]*import.com/pkg.Type"},
		{Override{"import.com/pkg", "[][]*pkg.Type"}, "[][]*import.com/pkg.Type"},
		{Override{"import.com/pkg", "map[string][]pkg.Type"}, "map[string][]import.com/pkg.Type"}}

	for _, tc := range testCases {
		require.Equal(t, tc.expected, NewOverrideFromString(tc.value))

		s, err := tc.expected.MarshalYAML()
		require.NoError(t, err)
		require.Equal(t, tc.value, s)
	}
}
