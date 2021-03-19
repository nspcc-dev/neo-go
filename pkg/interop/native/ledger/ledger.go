/*
Package ledger provides interface to LedgerContract native contract.
It allows to access ledger contents like transactions and blocks.
*/
package ledger

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents Ledger contract hash.
const Hash = "\xbe\xf2\x04\x31\x40\x36\x2a\x77\xc1\x50\x99\xc7\xe6\x4c\x12\xf7\x00\xb6\x65\xda"

// CurrentHash represents `currentHash` method of Ledger native contract.
func CurrentHash() interop.Hash256 {
	return contract.Call(interop.Hash160(Hash), "currentHash", contract.ReadStates).(interop.Hash256)
}

// CurrentIndex represents `currentIndex` method of Ledger native contract.
func CurrentIndex() int {
	return contract.Call(interop.Hash160(Hash), "currentIndex", contract.ReadStates).(int)
}

// GetBlock represents `getBlock` method of Ledger native contract.
func GetBlock(indexOrHash interface{}) *Block {
	return contract.Call(interop.Hash160(Hash), "getBlock", contract.ReadStates, indexOrHash).(*Block)
}

// GetTransaction represents `getTransaction` method of Ledger native contract.
func GetTransaction(hash interop.Hash256) *Transaction {
	return contract.Call(interop.Hash160(Hash), "getTransaction", contract.ReadStates, hash).(*Transaction)
}

// GetTransactionHeight represents `getTransactionHeight` method of Ledger native contract.
func GetTransactionHeight(hash interop.Hash256) int {
	return contract.Call(interop.Hash160(Hash), "getTransactionHeight", contract.ReadStates, hash).(int)
}

// GetTransactionFromBlock represents `getTransactionFromBlock` method of Ledger native contract.
func GetTransactionFromBlock(indexOrHash interface{}, txIndex int) *Transaction {
	return contract.Call(interop.Hash160(Hash), "getTransactionFromBlock", contract.ReadStates,
		indexOrHash, txIndex).(*Transaction)
}
