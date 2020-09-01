package transaction

import (
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
