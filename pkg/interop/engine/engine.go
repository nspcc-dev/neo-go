/*
Package engine allows to make contract calls.
It's roughly similar in function to ExecutionEngine class in the Neo .net
framework.
*/
package engine

// AppCall executes previously deployed blockchain contract with specified hash
// (160 bit in BE form represented as 20-byte slice) using provided arguments.
// It returns whatever this contract returns. Even though this function accepts
// a slice for scriptHash you can only use it for contracts known at
// compile time, because there is a significant difference between static and
// dynamic calls in Neo (contracts should have a special property declared
// and paid for to be able to use dynamic calls). This function uses
// `System.Contract.Call` syscall.
func AppCall(scriptHash []byte, method string, args ...interface{}) interface{} {
	return nil
}
