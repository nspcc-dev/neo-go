package block

import "github.com/CityOfZion/neo-storm/interop/transaction"

// Package block provides function signatures that can be used inside
// smart contracts that are written in the neo-storm framework.

// Block stubs a NEO block type.
type Block struct{}

// GetTransactionCount return the number of recorded transactions in the given block.
func GetTransactionCount(b Block) int {
	return 0
}

// GetTransactions returns a slice of transactions recorded in the given block.
func GetTransactions(b Block) []transaction.Transaction {
	return []transaction.Transaction{}
}

// GetTransaction returns a transaction from the given a block hash of the
// transaction.
func GetTransaction(b Block, hash []byte) transaction.Transaction {
	return transaction.Transaction{}
}
