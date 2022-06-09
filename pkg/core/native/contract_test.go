package native

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/stretchr/testify/require"
)

// TestNativeGetMethod is needed to ensure that methods list has the same sorting
// rule as we expect inside the `ContractMD.GetMethod`.
func TestNativeGetMethod(t *testing.T) {
	cfg := config.ProtocolConfiguration{P2PSigExtensions: true}
	cs := NewContracts(cfg)
	for _, c := range cs.Contracts {
		t.Run(c.Metadata().Name, func(t *testing.T) {
			for _, m := range c.Metadata().Methods {
				_, ok := c.Metadata().GetMethod(m.MD.Name, len(m.MD.Parameters))
				require.True(t, ok)
			}
		})
	}
}
