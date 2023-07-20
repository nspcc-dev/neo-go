package transaction

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScopesFromString(t *testing.T) {
	_, err := ScopesFromString("")
	require.Error(t, err)

	_, err = ScopesFromString("123")
	require.Error(t, err)

	s, err := ScopesFromString("Global")
	require.NoError(t, err)
	require.Equal(t, Global, s)

	s, err = ScopesFromString("CalledByEntry")
	require.NoError(t, err)
	require.Equal(t, CalledByEntry, s)

	s, err = ScopesFromString("CustomContracts")
	require.NoError(t, err)
	require.Equal(t, CustomContracts, s)

	s, err = ScopesFromString("CustomGroups")
	require.NoError(t, err)
	require.Equal(t, CustomGroups, s)

	s, err = ScopesFromString("CalledByEntry,CustomGroups")
	require.NoError(t, err)
	require.Equal(t, CalledByEntry|CustomGroups, s)

	_, err = ScopesFromString("Global,CustomGroups")
	require.Error(t, err)

	_, err = ScopesFromString("CalledByEntry,Global,CustomGroups")
	require.Error(t, err)

	s, err = ScopesFromString("CalledByEntry,CustomGroups,CustomGroups")
	require.NoError(t, err)
	require.Equal(t, CalledByEntry|CustomGroups, s)

	s, err = ScopesFromString("CalledByEntry, CustomGroups")
	require.NoError(t, err)
	require.Equal(t, CalledByEntry|CustomGroups, s)

	s, err = ScopesFromString("CalledByEntry, CustomGroups, CustomContracts")
	require.NoError(t, err)
	require.Equal(t, CalledByEntry|CustomGroups|CustomContracts, s)
}

func TestScopesFromByte(t *testing.T) {
	testCases := []struct {
		in         byte
		expected   WitnessScope
		shouldFail bool
	}{
		{
			in:       0,
			expected: None,
		},
		{
			in:       1,
			expected: CalledByEntry,
		},
		{
			in:       16,
			expected: CustomContracts,
		},
		{
			in:       32,
			expected: CustomGroups,
		},
		{
			in:       64,
			expected: Rules,
		},
		{
			in:       128,
			expected: Global,
		},
		{
			in:       17,
			expected: CalledByEntry | CustomContracts,
		},
		{
			in:       48,
			expected: CustomContracts | CustomGroups,
		},
		{
			in:         128 + 1, // Global can't be combined with others.
			shouldFail: true,
		},
		{
			in:         2, // No such scope.
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(strconv.Itoa(int(tc.in)), func(t *testing.T) {
			actual, err := ScopesFromByte(tc.in)
			if tc.shouldFail {
				require.Error(t, err, tc.in)
			} else {
				require.NoError(t, err, tc.in)
				require.Equal(t, tc.expected, actual, tc.in)
			}
		})
	}
}
