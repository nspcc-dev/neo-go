package main

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/stretchr/testify/require"
)

func TestCompatibility(t *testing.T) {
	cs := native.NewContracts(config.ProtocolConfiguration{})
	require.Equal(t, cs.Ledger.ID, int32(ledgerContractID))
}
