package native

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/stretchr/testify/require"
)

func TestNativenamesIsValid(t *testing.T) {
	// test that all native names has been added to IsValid
	cfg := config.ProtocolConfiguration{P2PSigExtensions: true}
	contracts := NewContracts(cfg)
	for _, c := range contracts.Contracts {
		require.True(t, nativenames.IsValid(c.Metadata().Name), fmt.Errorf("add %s to nativenames.IsValid(...)", c))
	}

	require.False(t, nativenames.IsValid("unknown"))
}
