package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenesisBlockMainNet(t *testing.T) {
	cfg, err := config.Load("../../config", config.ModeMainNet)
	require.NoError(t, err)

	block, err := createGenesisBlock(cfg.ProtocolConfiguration)
	require.NoError(t, err)

	//TODO: After we added Nonce field to transaction.Transaction, goveringTockenTx and UtilityTockenTx hashes
	// have been changed. Consequently, hash of the genesis block has been changed.
	// Update expected genesis block hash for better times.
	// Old hash is "d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf"
	expect := "e4cfc549c87d4ab7b570c368d05853ffb70eb9ef0f7d9c7a2e6e9e5d713ebbf4"
	assert.Equal(t, expect, block.Hash().StringLE())
}

func TestGetConsensusAddressMainNet(t *testing.T) {
	var (
		consensusAddr   = "ASEhJHr51ZMQUqmUbPC5uiuJwwvedwC48F"
		consensusScript = "72c3d9b3bbf776698694cd2c73fa597a10c31294"
	)

	cfg, err := config.Load("../../config", config.ModeMainNet)
	require.NoError(t, err)

	validators, err := getValidators(cfg.ProtocolConfiguration)
	require.NoError(t, err)

	script, err := getNextConsensusAddress(validators)
	require.NoError(t, err)

	assert.Equal(t, consensusScript, script.String())
	assert.Equal(t, consensusAddr, address.Uint160ToString(script))
}
