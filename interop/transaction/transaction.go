package transaction

import (
	"github.com/CityOfZion/neo-storm/interop/attribute"
	"github.com/CityOfZion/neo-storm/interop/input"
	"github.com/CityOfZion/neo-storm/interop/output"
)

// Package transaction provides function signatures that can be used inside
// smart contracts that are written in the neo-storm framework.

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

// FIXME: What is the correct return type for this?
// GetReferences returns a slice of references for the given transaction.
func GetReferences(t Transaction) interface{} {
	return 0
}

// FIXME: What is the correct return type for this?
// GetUnspentCoins returns the unspent coins for the given transaction.
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
