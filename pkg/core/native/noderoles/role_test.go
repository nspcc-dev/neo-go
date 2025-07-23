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

func TestIsValid(t *testing.T) {
	tests := []struct {
		name     string
		role     Role
		expected bool
	}{
		{"below range", StateValidator - 1, false},
		{"at lower bound", StateValidator, true},
		{"at upper bound", last - 1, true},
		{"above range", last, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, IsValid(tc.role))
		})
	}
}
