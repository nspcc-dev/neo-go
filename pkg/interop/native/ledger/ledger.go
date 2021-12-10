/*
Package ledger provides interface to LedgerContract native contract.
It allows to access ledger contents like transactions and blocks.
*/
package ledger

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Hash represents Ledger contract hash.
const Hash = "\xbe\xf2\x04\x31\x40\x36\x2a\x77\xc1\x50\x99\xc7\xe6\x4c\x12\xf7\x00\xb6\x65\xda"

// CurrentHash represents `currentHash` method of Ledger native contract.
func CurrentHash() interop.Hash256 {
	return neogointernal.CallWithToken(Hash, "currentHash", int(contract.ReadStates)).(interop.Hash256)
}

// CurrentIndex represents `currentIndex` method of Ledger native contract.
func CurrentIndex() int {
	return neogointernal.CallWithToken(Hash, "currentIndex", int(contract.ReadStates)).(int)
}

// GetBlock represents `getBlock` method of Ledger native contract.
func GetBlock(indexOrHash interface{}) *Block {
	return neogointernal.CallWithToken(Hash, "getBlock", int(contract.ReadStates), indexOrHash).(*Block)
}

// GetTransaction represents `getTransaction` method of Ledger native contract.
func GetTransaction(hash interop.Hash256) *Transaction {
	return neogointernal.CallWithToken(Hash, "getTransaction", int(contract.ReadStates), hash).(*Transaction)
}

// GetTransactionHeight represents `getTransactionHeight` method of Ledger native contract.
func GetTransactionHeight(hash interop.Hash256) int {
	return neogointernal.CallWithToken(Hash, "getTransactionHeight", int(contract.ReadStates), hash).(int)
}

// GetTransactionFromBlock represents `getTransactionFromBlock` method of Ledger native contract.
func GetTransactionFromBlock(indexOrHash interface{}, txIndex int) *Transaction {
	return neogointernal.CallWithToken(Hash, "getTransactionFromBlock", int(contract.ReadStates),
		indexOrHash, txIndex).(*Transaction)
}
