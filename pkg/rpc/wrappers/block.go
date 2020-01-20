package wrappers

import (
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/block"
	"github.com/CityOfZion/neo-go/pkg/util"
)

type (
	// Block wrapper used for the representation of
	// block.Block / block.Base on the RPC Server.
	Block struct {
		*block.Block
		Confirmations uint32       `json:"confirmations"`
		NextBlockHash util.Uint256 `json:"nextblockhash,omitempty"`
		Hash          util.Uint256 `json:"hash"`
	}
)

// NewBlock creates a new Block wrapper.
func NewBlock(block *block.Block, chain core.Blockchainer) Block {
	blockWrapper := Block{
		Block: block,
		Hash:  block.Hash(),
	}

	hash := chain.GetHeaderHash(int(block.Index) + 1)
	if !hash.Equals(util.Uint256{}) {
		blockWrapper.NextBlockHash = hash
	}

	blockWrapper.Confirmations = chain.BlockHeight() - block.Index - 1
	return blockWrapper
}
