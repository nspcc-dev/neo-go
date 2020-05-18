package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/attribute"
	"github.com/nspcc-dev/neo-go/pkg/interop/input"
	"github.com/nspcc-dev/neo-go/pkg/interop/output"
)

// Package transaction provides function signatures that can be used inside
// smart contracts that are written in the neo-go framework.

// Transaction stubs a NEO transaction type.
type Transaction struct{}

// GetHash returns the hash of the given transaction.
func GetHash(t Transaction) []byte {
	return nil
}

// GetType returns the type of the given transaction.
func GetType(t Transaction) byte {
	return 0x00
}

// GetAttributes returns a slice of attributes for the given transaction.
func GetAttributes(t Transaction) []attribute.Attribute {
	return []attribute.Attribute{}
}

// GetReferences returns a slice of references for the given transaction.
// FIXME: What is the correct return type for this?
func GetReferences(t Transaction) []interface{} {
	return []interface{}{}
}

// GetUnspentCoins returns the unspent coins for the given transaction.
// FIXME: What is the correct return type for this?
func GetUnspentCoins(t Transaction) interface{} {
	return 0
}

// GetInputs returns the inputs of the given transaction.
func GetInputs(t Transaction) []input.Input {
	return []input.Input{}
}

// GetOutputs returns the outputs of the given transaction.
func GetOutputs(t Transaction) []output.Output {
	return []output.Output{}
}

// GetScript returns the script stored in a given Invocation transaction.
// Calling it for any other Transaction type would lead to failure. It uses
// `Neo.InvocationTransaction.GetScript` syscall.
func GetScript(t Transaction) []byte {
	return nil
}
