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
	expect := "094c2c2db5dcb868d85aa4d652aed23bc67e7166f53223a228e382265b1be84b"
	assert.Equal(t, expect, block.Hash().StringLE())
}

func TestGetConsensusAddressMainNet(t *testing.T) {
	var (
		consensusAddr   = "APtiVEdLi5GEmQ8CL5RcCE7BNcsPsxeXh7"
		consensusScript = "590c459950f1d83e67ee11fcef202a6ebb8b1a77"
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
	expect := "b16384a950ed01ed5fc15c03fe7b98228871cb43b1bc22d67029449fc854d104"
	assert.Equal(t, expect, UtilityTokenID().StringLE())
}

func TestGoverningTokenTX(t *testing.T) {
	//TODO: After we added Nonce field to transaction.Transaction, GoveringTockenTx hash
	// has been changed. Update it for better times.
	// Old hash is "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b"
	expect := "7a37715546c6cfa5bac8d7f7e87c667a1e5a6ba0601238be475ab8c79a5abcf5"
	assert.Equal(t, expect, GoverningTokenID().StringLE())
}
