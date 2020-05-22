/*
Package transaction provides functions to work with transactions.
*/
package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/attribute"
	"github.com/nspcc-dev/neo-go/pkg/interop/input"
	"github.com/nspcc-dev/neo-go/pkg/interop/output"
	"github.com/nspcc-dev/neo-go/pkg/interop/witness"
)

// Transaction represents a NEO transaction, it's an opaque data structure
// that can be used with functions from this package. It's similar to
// Transaction class in Neo .net framework.
type Transaction struct{}

// GetHash returns the hash (256 bit BE value in a 32 byte slice) of the given
// transaction (which also is its ID). Is uses `Neo.Transaction.GetHash` syscall.
func GetHash(t Transaction) []byte {
	return nil
}

// GetType returns the type of the given transaction. Possible values:
//     MinerTransaction      = 0x00
//     IssueTransaction      = 0x01
//     ClaimTransaction      = 0x02
//     EnrollmentTransaction = 0x20
//     RegisterTransaction   = 0x40
//     ContractTransaction   = 0x80
//     StateType             = 0x90
//     AgencyTransaction     = 0xb0
//     PublishTransaction    = 0xd0
//     InvocationTransaction = 0xd1
// It uses `Neo.Transaction.GetType` syscall.
func GetType(t Transaction) byte {
	return 0x00
}

// GetAttributes returns a slice of attributes for agiven transaction. Refer to
// attribute package on how to use them. This function uses
// `Neo.Transaction.GetAttributes` syscall.
func GetAttributes(t Transaction) []attribute.Attribute {
	return []attribute.Attribute{}
}

// GetReferences returns a slice of references for a given Transaction. Elements
// of this slice can be casted to any of input.Input or output.Output, depending
// on which information you're interested in (as reference technically contains
// both input and corresponding output), refer to input and output package on
// how to use them. This function uses `Neo.Transaction.GetReferences` syscall.
func GetReferences(t Transaction) []interface{} {
	return []interface{}{}
}

// GetUnspentCoins returns a slice of not yet spent ouputs of a given transaction.
// This function uses `Neo.Transaction.GetUnspentCoint` syscall.
func GetUnspentCoins(t Transaction) []output.Output {
	return []output.Output{}
}

// GetInputs returns a slice of inputs of a given Transaction. Refer to input
// package on how to use them. This function uses `Neo.Transaction.GetInputs`
// syscall.
func GetInputs(t Transaction) []input.Input {
	return []input.Input{}
}

// GetOutputs returns a slice of outputs of a given Transaction. Refer to output
// package on how to use them. This function uses `Neo.Transaction.GetOutputs`
// syscall.
func GetOutputs(t Transaction) []output.Output {
	return []output.Output{}
}

// GetScript returns the script stored in a given Invocation transaction.
// Calling it for any other Transaction type would lead to failure. It uses
// `Neo.InvocationTransaction.GetScript` syscall.
func GetScript(t Transaction) []byte {
	return nil
}

// GetWitnesses returns a slice of witnesses of a given Transaction. Refer to
// witness package on how to use them. This function uses
// `Neo.Transaction.GetWitnesses` syscall.
func GetWitnesses(t Transaction) []witness.Witness {
	return []witness.Witness{}
}
