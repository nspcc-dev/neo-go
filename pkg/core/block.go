package core

import (
	. "github.com/anthdm/neo-go/pkg/util"
)

// Block represents one block in the chain.
type Block struct {
	Version uint32
	// hash of the previous block.
	PrevBlock Uint256
	// Root hash of a transaction list.
	MerkleRoot Uint256
	// timestamp
	Timestamp uint32
	// height of the block
	Height uint32
	// Random number
	Nonce uint64
	// contract addresss of the next miner
	NextMiner Uint160
	// seperator ? fixed to 1
	_sep uint8
	// Script used to validate the block
	Script []byte
	// transaction list
	Transactions []*Transaction
}
