/*
Package engine allows to make contract calls.
It's roughly similar in function to ExecutionEngine class in the Neo .net
framework.
*/
package engine

import "github.com/nspcc-dev/neo-go/pkg/interop"

// AppCall executes previously deployed blockchain contract with specified hash
// (160 bit in BE form represented as 20-byte slice) using provided arguments.
// It returns whatever this contract returns. This function uses
// `System.Contract.Call` syscall.
func AppCall(scriptHash interop.Hash160, method string, args ...interface{}) interface{} {
	return nil
}
