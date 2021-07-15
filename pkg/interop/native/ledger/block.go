package ledger

import "github.com/nspcc-dev/neo-go/pkg/interop"

// Block represents a NEO block, it's a data structure that you can get
// block-related data from. It's similar to the Block class in the Neo .net
// framework. To use it you need to get it via GetBlock function call.
type Block struct {
	// Hash represents the hash (256 bit BE value in a 32 byte slice) of the
	// given block.
	Hash interop.Hash256
	// Version of the block.
	Version int
	// PrevHash represents the hash (256 bit BE value in a 32 byte slice) of the
	// previous block.
	PrevHash interop.Hash256
	// MerkleRoot represents the root hash (256 bit BE value in a 32 byte slice)
	// of a transaction list.
	MerkleRoot interop.Hash256
	// Timestamp represents millisecond-precision block timestamp.
	Timestamp int
	// Nonce represents block nonce.
	Nonce int
	// Index represents the height of the block.
	Index int
	// NextConsensus represents contract address of the next miner (160 bit BE
	// value in a 20 byte slice).
	NextConsensus interop.Hash160
	// TransactionsLength represents the length of block's transactions array.
	TransactionsLength int
}
