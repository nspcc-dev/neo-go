package noderoles

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromString(t *testing.T) {
	valid := map[string]Role{
		"StateValidator": StateValidator,
		"Oracle":         Oracle,
		"NeoFSAlphabet":  NeoFSAlphabet,
		"P2PNotary":      P2PNotary,
	}
	for s, expected := range valid {
		actual, ok := FromString(s)
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}

	invalid := []string{"last", "InvalidRole"}
	for _, s := range invalid {
		_, ok := FromString(s)
		require.False(t, ok)
	}
}
