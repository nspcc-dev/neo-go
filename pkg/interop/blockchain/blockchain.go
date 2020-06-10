/*
Package blockchain provides functions to access various blockchain data.
*/
package blockchain

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/account"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/header"
)

// Transaction represents a NEO transaction. It's similar to Transaction class
// in Neo .net framework.
type Transaction struct {
	// Hash represents the hash (256 bit BE value in a 32 byte slice) of the
	// given transaction (which also is its ID).
	Hash []byte
	// Version represents the transaction version.
	Version int
	// Nonce is a random number to avoid hash collision.
	Nonce int
	// Sender represents the sender (160 bit BE value in a 20 byte slice) of the
	// given Transaction.
	Sender []byte
	// SysFee represents fee to be burned.
	SysFee int
	// NetFee represents fee to be distributed to consensus nodes.
	NetFee int
	// ValidUntilBlock is the maximum blockchain height exceeding which
	// transaction should fail verification.
	ValidUntilBlock int
	// Script represents code to run in NeoVM for this transaction.
	Script []byte
}

// Block represents a NEO block, it's a data structure that you can get
// block-related data from. It's similar to the Block class in the Neo .net
// framework. To use it you need to get it via GetBlock function call.
type Block struct {
	// Hash represents the hash (256 bit BE value in a 32 byte slice) of the
	// given block.
	Hash []byte
	// Version of the block.
	Version int
	// PrevHash represents the hash (256 bit BE value in a 32 byte slice) of the
	// previous block.
	PrevHash []byte
	// MerkleRoot represents the root hash (256 bit BE value in a 32 byte slice)
	// of a transaction list.
	MerkleRoot []byte
	// Timestamp represents millisecond-precision block timestamp.
	Timestamp int
	// Index represents the height of the block.
	Index int
	// NextConsensus representes contract address of the next miner (160 bit BE
	// value in a 20 byte slice).
	NextConsensus []byte
	// TransactionsLength represents the length of block's transactions array.
	TransactionsLength int
}

// GetHeight returns current block height (index of the last accepted block).
// Note that when transaction is being run as a part of new block this block is
// considered as not yet accepted (persisted) and thus you'll get an index of
// the previous (already accepted) block. This function uses
// `Neo.Blockchain.GetHeight` syscall.
func GetHeight() int {
	return 0
}

// GetHeader returns header found by the given hash (256 bit hash in BE format
// represented as a slice of 32 bytes) or index (integer). Refer to the `header`
// package for possible uses of returned structure. This function uses
// `Neo.Blockchain.GetHeader` syscall.
func GetHeader(heightOrHash interface{}) header.Header {
	return header.Header{}
}

// GetBlock returns block found by the given hash or index (with the same
// encoding as for GetHeader). This function uses `System.Blockchain.GetBlock`
// syscall.
func GetBlock(heightOrHash interface{}) Block {
	return Block{}
}

// GetTransaction returns transaction found by the given hash (256 bit in BE
// format represented as a slice of 32 bytes). This function uses
// `System.Blockchain.GetTransaction` syscall.
func GetTransaction(hash []byte) Transaction {
	return Transaction{}
}

// GetTransactionFromBlock returns transaction hash (256 bit in BE format
// represented as a slice of 32 bytes) from the block found by the given hash or
// index (with the same encoding as for GetHeader) by its index. This
// function uses `System.Blockchain.GetTransactionFromBlock` syscall.
func GetTransactionFromBlock(heightOrHash interface{}, index int) []byte {
	return nil
}

// GetTransactionHeight returns transaction's height (index of the block that
// includes it) by the given ID (256 bit in BE format represented as a slice of
// 32 bytes). This function uses `System.Blockchain.GetTransactionHeight` syscall.
func GetTransactionHeight(hash []byte) int {
	return 0
}

// GetContract returns contract found by the given script hash (160 bit in BE
// format represented as a slice of 20 bytes). Refer to the `contract` package
// for details on how to use the returned structure. This function uses
// `Neo.Blockchain.GetContract` syscall.
func GetContract(scriptHash []byte) contract.Contract {
	return contract.Contract{}
}

// GetAccount returns account found by the given script hash (160 bit in BE
// format represented as a slice of 20 bytes). Refer to the `account` package
// for details on how to use the returned structure. This function uses
// `Neo.Blockchain.GetAccount` syscall.
func GetAccount(scriptHash []byte) account.Account {
	return account.Account{}
}

// GetValidators returns a slice of current validators public keys represented
// as a compressed serialized byte slice (33 bytes long). This function uses
// `Neo.Blockchain.GetValidators` syscall.
func GetValidators() [][]byte {
	return nil
}
