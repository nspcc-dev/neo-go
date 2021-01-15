package native

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Compatibility test. hashes are taken directly from C# node.
func TestNativeHashes(t *testing.T) {
	require.Equal(t, "bee421fdbb3e791265d2104cb34934f53fcc0e45", newManagement().Hash.StringLE())
	require.Equal(t, "4961bf0ab79370b23dc45cde29f568d0e0fa6e93", newNEO().Hash.StringLE())
	require.Equal(t, "9ac04cf223f646de5f7faccafe34e30e5d4382a2", newGAS().Hash.StringLE())
	require.Equal(t, "c939a4af1c762e5edca36d4b61c06ba82c4c6ff5", newPolicy().Hash.StringLE())
	require.Equal(t, "f4bbd95569e8dda2cb84eb609a1488ddd0d9fa91", newDesignate(false).Hash.StringLE())
	require.Equal(t, "8cd3889136056b3304ec59f6d424b8767710ed79", newOracle().Hash.StringLE())
	// Not yet a part of NEO.
	//require.Equal(t, "", newNotary().Hash.StringLE()())
}
