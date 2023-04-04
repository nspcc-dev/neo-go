package ledger

import "github.com/nspcc-dev/neo-go/pkg/interop"

// Block represents a NEO block, it's a data structure where you can get
// block-related data from. It's similar to the Block class in the Neo .net
// framework. To use it, you need to get it via GetBlock function call.
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
	// NextConsensus represents the contract address of the next miner (160 bit BE
	// value in a 20 byte slice).
	NextConsensus interop.Hash160
	// TransactionsLength represents the length of block's transactions array.
	TransactionsLength int
}

// BlockSR is a stateroot-enabled Neo block. It's returned from the Ledger contract's
// GetBlock method when StateRootInHeader NeoGo extension  is used. Use it only when
// you have it enabled when you need to access PrevStateRoot field, Block is sufficient
// otherwise. To get this data type ToBlockSR method of Block should be used. All of
// the fields are same as in Block except PrevStateRoot.
type BlockSR struct {
	Hash               interop.Hash256
	Version            int
	PrevHash           interop.Hash256
	MerkleRoot         interop.Hash256
	Timestamp          int
	Nonce              int
	Index              int
	NextConsensus      interop.Hash160
	TransactionsLength int
	// PrevStateRoot is a hash of the previous block's state root.
	PrevStateRoot interop.Hash256
}

// ToBlockSR converts Block into BlockSR for chains with StateRootInHeader option.
func (b *Block) ToBlockSR() *BlockSR {
	return any(b).(*BlockSR)
}
