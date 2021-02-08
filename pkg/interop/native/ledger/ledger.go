package ledger

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
)

// Hash represents Ledger contract hash.
const Hash = "\x64\x87\x5a\x12\xc6\x03\xc6\x1d\xec\xff\xdf\xe7\x88\xce\x10\xdd\xc6\x69\x1d\x97"

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
