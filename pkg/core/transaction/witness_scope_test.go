package transaction

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScopesFromString(t *testing.T) {
	s, err := ScopesFromString("")
	require.Error(t, err)

	_, err = ScopesFromString("123")
	require.Error(t, err)

	s, err = ScopesFromString("Global")
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

	s, err = ScopesFromString("Calledbyentry,customgroups")
	require.NoError(t, err)
	require.Equal(t, CalledByEntry|CustomGroups, s)

	_, err = ScopesFromString("global,customgroups")
	require.Error(t, err)

	_, err = ScopesFromString("calledbyentry,global,customgroups")
	require.Error(t, err)

	s, err = ScopesFromString("Calledbyentry,customgroups,Customgroups")
	require.NoError(t, err)
	require.Equal(t, CalledByEntry|CustomGroups, s)
}
