package native

import (
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

// Compatibility test. hashes are taken directly from C# node.
func TestNativeHashes(t *testing.T) {
	require.Equal(t, "a501d7d7d10983673b61b7a2d3a813b36f9f0e43", newManagement().Hash.StringLE())
	require.Equal(t, "971d69c6dd10ce88e7dfffec1dc603c6125a8764", newLedger().Hash.StringLE())
	require.Equal(t, "f61eebf573ea36593fd43aa150c055ad7906ab83", newNEO().Hash.StringLE())
	require.Equal(t, "70e2301955bf1e74cbb31d18c2f96972abadb328", newGAS().Hash.StringLE())
	require.Equal(t, "79bcd398505eb779df6e67e4be6c14cded08e2f2", newPolicy().Hash.StringLE())
	require.Equal(t, "597b1471bbce497b7809e2c8f10db67050008b02", newDesignate(false).Hash.StringLE())
	require.Equal(t, "8dc0e742cbdfdeda51ff8a8b78d46829144c80ee", newOracle().Hash.StringLE())
	// Not yet a part of NEO.
	//require.Equal(t, "", newNotary().Hash.StringLE()())
}

// "C" and "O" can easily be typed by accident.
func TestNamesASCII(t *testing.T) {
	cs := NewContracts(true)
	for _, c := range cs.Contracts {
		require.True(t, isASCII(c.Metadata().Name))
		for m := range c.Metadata().Methods {
			require.True(t, isASCII(m.Name))
		}
		for _, e := range c.Metadata().Manifest.ABI.Events {
			require.True(t, isASCII(e.Name))
		}
	}
}

func isASCII(s string) bool {
	ok := true
	for i := 0; i < len(s); i++ {
		ok = ok && s[i] <= unicode.MaxASCII
	}
	return ok
}
