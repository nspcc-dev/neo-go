package native

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Compatibility test. hashes are taken directly from C# node.
func TestNativeHashes(t *testing.T) {
	require.Equal(t, "136ec44854ad9a714901eb7d714714f1791203f2", newDesignate(false).Hash.StringLE())
	require.Equal(t, "a6a6c15dcdc9b997dac448b6926522d22efeedfb", newGAS().Hash.StringLE())
	require.Equal(t, "081514120c7894779309255b7fb18b376cec731a", newManagement().Hash.StringLE())
	require.Equal(t, "0a46e2e37c9987f570b4af253fb77e7eef0f72b6", newNEO().Hash.StringLE())
	// Not yet a part of NEO.
	//require.Equal(t, "", newNotary().Hash.StringLE()())
	require.Equal(t, "b1c37d5847c2ae36bdde31d0cc833a7ad9667f8f", newOracle().Hash.StringLE())
	require.Equal(t, "dde31084c0fdbebc7f5ed5f53a38905305ccee14", newPolicy().Hash.StringLE())
}
