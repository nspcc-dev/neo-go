package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenesisBlockMainNet(t *testing.T) {
	cfg, err := config.Load("../../config", netmode.MainNet)
	require.NoError(t, err)

	block, err := CreateGenesisBlock(cfg.ProtocolConfiguration)
	require.NoError(t, err)

	expect := "1f4d1defa46faa5e7b9b8d3f79a06bec777d7c26c4aa5f6f5899a291daa87c15"
	assert.Equal(t, expect, block.Hash().StringLE())
}

func TestGetConsensusAddressMainNet(t *testing.T) {
	var (
		consensusAddr   = "NVg7LjGcUSrgxgjX3zEgqaksfMaiS8Z6e1"
		consensusScript = "6b123dd8bec718648852bbc78595e3536a058f9f"
	)

	cfg, err := config.Load("../../config", netmode.MainNet)
	require.NoError(t, err)

	validators, _, err := validatorsFromConfig(cfg.ProtocolConfiguration)
	require.NoError(t, err)

	script, err := getNextConsensusAddress(validators)
	require.NoError(t, err)

	assert.Equal(t, consensusScript, script.String())
	assert.Equal(t, consensusAddr, address.Uint160ToString(script))
}

func TestGetExpectedHeaderSize(t *testing.T) {
	cfg, err := config.Load("../../config", netmode.MainNet)
	require.NoError(t, err)
	blk, err := CreateGenesisBlock(cfg.ProtocolConfiguration)
	require.NoError(t, err)
	w := io.NewBufBinWriter()
	blk.Header.EncodeBinary(w.BinWriter)
	require.NoError(t, w.Err)
	require.Equal(t, block.GetExpectedHeaderSize(false, 0), w.Len())
}
