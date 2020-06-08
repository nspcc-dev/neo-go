/*
Package block provides getters for Neo Block structure.
*/
package block

import "github.com/nspcc-dev/neo-go/pkg/interop/blockchain"

// Block represents a NEO block, it's an opaque data structure that you can get
// data from only using functions from this package. It's similar in function to
// the Block class in the Neo .net framework. To use it you need to get it via
// blockchain.GetBlock function call.
type Block struct{}

// GetTransactionCount returns the number of recorded transactions in the given
// block. It uses `Neo.Block.GetTransactionCount` syscall internally.
func GetTransactionCount(b Block) int {
	return 0
}

// GetTransactions returns a slice of transactions recorded in the given block.
// It uses `Neo.Block.GetTransactions` syscall internally.
func GetTransactions(b Block) []blockchain.Transaction {
	return []blockchain.Transaction{}
}

// GetTransaction returns transaction from the given block by its index. It
// uses `Neo.Block.GetTransaction` syscall internally.
func GetTransaction(b Block, index int) blockchain.Transaction {
	return blockchain.Transaction{}
}
