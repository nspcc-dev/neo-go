package engine

import "github.com/CityOfZion/neo-storm/interop/transaction"

// Package engine provides function signatures that can be used inside
// smart contracts that are written in the neo-storm framework.

// GetScriptContainer returns the transaction that is in the execution context.
func GetScriptContainer() transaction.Transaction {
	return transaction.Transaction{}
}

// GetExecutingScriptHash returns the script hash of the contract that is
// currently being executed.
func GetExecutingScriptHash() []byte {
	return nil
}

// GetCallingScriptHash returns the script hash of the contract that started
// the execution of the current script.
func GetCallingScriptHash() []byte {
	return nil
}

// GetEntryScriptHash returns the script hash of the contract the started the
// execution from the start.
func GetEntryScriptHash() []byte {
	return nil
}
