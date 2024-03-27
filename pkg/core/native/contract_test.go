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
	latestHF := config.LatestHardfork()
	for _, c := range cs.Contracts {
		hfMD := c.Metadata().HFSpecificContractMD(&latestHF)
		t.Run(c.Metadata().Name, func(t *testing.T) {
			for _, m := range hfMD.Methods {
				_, ok := hfMD.GetMethod(m.MD.Name, len(m.MD.Parameters))
				require.True(t, ok)
			}
		})
	}
}
