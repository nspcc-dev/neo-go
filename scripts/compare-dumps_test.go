package main

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/stretchr/testify/require"
)

func TestCompatibility(t *testing.T) {
	cs := native.NewContracts(false, map[string][]uint32{})
	require.Equal(t, cs.Ledger.ID, int32(ledgerContractID))
}
