package native

import (
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
)

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
