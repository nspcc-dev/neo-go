package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/stretchr/testify/assert"
)

func TestGenesisBlockMainNet(t *testing.T) {
	cfg, err := config.Load("../../config", config.ModeMainNet)
	if err != nil {
		t.Fatal(err)
	}

	block, err := createGenesisBlock(cfg.ProtocolConfiguration)
	if err != nil {
		t.Fatal(err)
	}

	expect := "d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf"
	assert.Equal(t, expect, block.Hash().String())
}

func TestGetConsensusAddressMainNet(t *testing.T) {
	var (
		consensusAddr   = "APyEx5f4Zm4oCHwFWiSTaph1fPBxZacYVR"
		consensusScript = "59e75d652b5d3827bf04c165bbe9ef95cca4bf55"
	)

	cfg, err := config.Load("../../config", config.ModeMainNet)
	if err != nil {
		t.Fatal(err)
	}

	validators, err := getValidators(cfg.ProtocolConfiguration)
	if err != nil {
		t.Fatal(err)
	}

	script, err := getNextConsensusAddress(validators)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, consensusScript, script.String())
	assert.Equal(t, consensusAddr, crypto.AddressFromUint160(script))
}

func TestUtilityTokenTX(t *testing.T) {
	expect := "602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"
	tx := utilityTokenTX()
	assert.Equal(t, expect, tx.Hash().String())
}

func TestGoverningTokenTX(t *testing.T) {
	expect := "c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b"
	tx := governingTokenTX()
	assert.Equal(t, expect, tx.Hash().String())
}
