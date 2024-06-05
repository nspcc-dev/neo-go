package native

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/stretchr/testify/require"
)

func TestNativenamesIsValid(t *testing.T) {
	contracts := NewContracts(config.ProtocolConfiguration{})
	for _, c := range contracts.Contracts {
		require.True(t, nativenames.IsValid(c.Metadata().Name), fmt.Errorf("add %s to nativenames.IsValid(...)", c))
	}

	require.False(t, nativenames.IsValid("unknown"))
}
