package native

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The specification is following C# code:
// string domain = string.Join('.', name.Split('.')[^2..]);
func TestParseDomain(t *testing.T) {
	testCases := []struct {
		name   string
		domain string
	}{
		{"sim.pl.e", "pl.e"},
		{"some.long.d.o.m.a.i.n", "i.n"},
		{"t.wo", "t.wo"},
		{".dot", ".dot"},
		{".d.ot", "d.ot"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dom, ok := domainFromString(tc.name)
			require.True(t, ok)
			require.Equal(t, tc.domain, dom)
		})
	}

	_, ok := domainFromString("nodots")
	require.False(t, ok)
}
