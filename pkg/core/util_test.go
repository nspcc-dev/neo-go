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
	expect := "75b6219158953816fbfe1884160f3fe0a4a4d0f7a2b7948bc89787d616f84983"
	assert.Equal(t, expect, block.Hash().StringLE())
}

func TestGetConsensusAddressMainNet(t *testing.T) {
	var (
		consensusAddr   = "ASwdHjdAGfmSDuZbr641W1eYFVugjByJAS"
		consensusScript = "7a818ecc4582f8526e7c4271a690c04bd3b9e017"
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
	expect := "8dd7d330dd7fc103836409bdcba826d15d88119c7f843357266b253aede72dfb"
	assert.Equal(t, expect, UtilityTokenID().StringLE())
}

func TestGoverningTokenTX(t *testing.T) {
	//TODO: After we added Nonce field to transaction.Transaction, GoveringTockenTx hash
	// has been changed. Update it for better times.
	// Old hash is "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b"
	expect := "0589624521f631b389197e1a69b1b92db0a45cc70f45c3409dfecc439e99bfa9"
	assert.Equal(t, expect, GoverningTokenID().StringLE())
}
