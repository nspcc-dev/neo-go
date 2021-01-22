package native

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Compatibility test. hashes are taken directly from C# node.
func TestNativeHashes(t *testing.T) {
	require.Equal(t, "a501d7d7d10983673b61b7a2d3a813b36f9f0e43", newManagement().Hash.StringLE())
	require.Equal(t, "f617baca689d1abddedda7c3b80675c4ac21e932", newNEO().Hash.StringLE())
	require.Equal(t, "75844530eb44f4715a42950bb59b4d7ace0b2f3d", newGAS().Hash.StringLE())
	require.Equal(t, "e21a28cfc1e662e152f668c86198141cc17b6c37", newPolicy().Hash.StringLE())
	require.Equal(t, "69b1909aaa14143b0624ba0d61d5cd3b8b67529d", newDesignate(false).Hash.StringLE())
	require.Equal(t, "b82bbf650f963dbf71577d10ea4077e711a13e7b", newOracle().Hash.StringLE())
	// Not yet a part of NEO.
	//require.Equal(t, "", newNotary().Hash.StringLE()())
}
