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
