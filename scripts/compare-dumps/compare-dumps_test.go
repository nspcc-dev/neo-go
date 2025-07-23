package main

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/stretchr/testify/require"
)

func TestCompatibility(t *testing.T) {
	cs := native.NewDefaultContracts(config.ProtocolConfiguration{})
	for _, c := range cs {
		if c.Metadata().Name == nativenames.Ledger {
			require.Equal(t, c.Metadata().ID, int32(ledgerContractID))
		}
	}
}
