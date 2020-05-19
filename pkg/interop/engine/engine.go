/*
Package engine provides access to VM execution metadata and allows to make contract calls.
It's roughly similar in function to ExecutionEngine class in the Neo .net
framework.
*/
package engine

import "github.com/nspcc-dev/neo-go/pkg/interop/transaction"

// GetScriptContainer returns the transaction that initially triggered current
// execution context. It never changes in a single execution, no matter how deep
// this execution goes. See `transaction` package for details on how to use the
// returned value. This function uses `System.ExecutionEngine.GetScriptContainer`
// syscall.
func GetScriptContainer() transaction.Transaction {
	return transaction.Transaction{}
}

// GetExecutingScriptHash returns script hash (160 bit in BE form represented
// as 20-byte slice) of the contract that is currently being executed. Any
// AppCall can change the value returned by this function if it calls a
// different contract. This function uses
// `System.ExecutionEngine.GetExecutingScriptHash` syscall.
func GetExecutingScriptHash() []byte {
	return nil
}

// GetCallingScriptHash returns script hash (160 bit in BE form represented
// as 20-byte slice) of the contract that started the execution of the currently
// running context (caller of current contract or function), so it's one level
// above the GetExecutingScriptHash in the call stack. It uses
// `System.ExecutionEngine.GetCallingScriptHash` syscall.
func GetCallingScriptHash() []byte {
	return nil
}

// GetEntryScriptHash returns script hash (160 bit in BE form represented
// as 20-byte slice) of the contract that initially started current execution
// (this is a script that is contained in a transaction returned by
// GetScriptContainer) execution from the start. This function uses
// `System.ExecutionEngine.GetEntryScriptHash` syscall.
func GetEntryScriptHash() []byte {
	return nil
}

// AppCall executes previously deployed blockchain contract with specified hash
// (160 bit in BE form represented as 20-byte slice) using provided arguments.
// It returns whatever this contract returns. Even though this function accepts
// a slice for scriptHash you can only use it for contracts known at
// compile time, because there is a significant difference between static and
// dynamic calls in Neo (contracts should have a special property declared
// and paid for to be able to use dynamic calls). This function uses `APPCALL`
// opcode.
func AppCall(scriptHash []byte, args ...interface{}) interface{} {
	return nil
}

// DynAppCall executes previously deployed blockchain contract with specified
// hash (160 bit in BE form represented as 20-byte slice) using provided
// arguments. It returns whatever this contract returns. It differs from AppCall
// in that you can use it for truly dynamic scriptHash values, but at the same
// time using it requires HasDynamicInvoke property set for a contract doing
// this call. This function uses `APPCALL` opcode.
func DynAppCall(scriptHash []byte, args ...interface{}) interface{} {
	return nil
}
