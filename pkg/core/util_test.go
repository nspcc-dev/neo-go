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
	expect := "1d4156d233220b893797a684fbb827bb2163b5042edd10653bbc1b2769adbb8d"
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

func TestUtilityTokenTX(t *testing.T) {
	//TODO: After we added Nonce field to transaction.Transaction, UtilityTockenTx hash
	// has been changed. Update it for better times.
	// Old hash is "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"
	expect := "f882fb865bab84b99623f21eedd902286af7da8d8a4609d7acefce04c851dc1c"
	assert.Equal(t, expect, UtilityTokenID().StringLE())
}

func TestGoverningTokenTX(t *testing.T) {
	//TODO: After we added Nonce field to transaction.Transaction, GoveringTockenTx hash
	// has been changed. Update it for better times.
	// Old hash is "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b"
	expect := "1a5e0e3eac2abced7de9ee2de0820a5c85e63756fcdfc29b82fead86a7c07c78"
	assert.Equal(t, expect, GoverningTokenID().StringLE())
}
