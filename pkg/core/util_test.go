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
	expect := "8c5e44474b2b942286071254fd4bffddd3febd0511b101e566331b5f8f041902"
	assert.Equal(t, expect, block.Hash().StringLE())
}

func TestGetConsensusAddressMainNet(t *testing.T) {
	var (
		consensusAddr   = "APyEx5f4Zm4oCHwFWiSTaph1fPBxZacYVR"
		consensusScript = "59e75d652b5d3827bf04c165bbe9ef95cca4bf55"
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
	expect := "057ea06225860f0f7e69cca1e0052652918629929591b8138a516431be144ba8"
	assert.Equal(t, expect, UtilityTokenID().StringLE())
}

func TestGoverningTokenTX(t *testing.T) {
	//TODO: After we added Nonce field to transaction.Transaction, GoveringTockenTx hash
	// has been changed. Update it for better times.
	// Old hash is "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b"
	expect := "cd3a3b5654465238c3a4ac30eeb1bfd1171378d59b27f7d2e6893ce9d6150825"
	assert.Equal(t, expect, GoverningTokenID().StringLE())
}
