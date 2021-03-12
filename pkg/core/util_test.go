package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenesisBlockMainNet(t *testing.T) {
	cfg, err := config.Load("../../config", netmode.MainNet)
	require.NoError(t, err)

	block, err := createGenesisBlock(cfg.ProtocolConfiguration)
	require.NoError(t, err)

	expect := "5816ac116af288777c4c454425fb687981f508826ec474810ff9e6b24202fd9a"
	assert.Equal(t, expect, block.Hash().StringLE())
}

func TestGetConsensusAddressMainNet(t *testing.T) {
	var (
		consensusAddr   = "NSX179gdoQmF8nu34rQdL4dYAfdCQhHtQS"
		consensusScript = "4870eaa62eee7c76b76d2ae933d4c027f5f5c77d"
	)

	cfg, err := config.Load("../../config", netmode.MainNet)
	require.NoError(t, err)

	validators, err := validatorsFromConfig(cfg.ProtocolConfiguration)
	require.NoError(t, err)

	script, err := getNextConsensusAddress(validators)
	require.NoError(t, err)

	assert.Equal(t, consensusScript, script.String())
	assert.Equal(t, consensusAddr, address.Uint160ToString(script))
}
