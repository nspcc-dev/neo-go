/*
Package transaction provides functions to work with transactions.
*/
package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/attribute"
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

// GetAttributes returns a slice of attributes for agiven transaction. Refer to
// attribute package on how to use them. This function uses
// `Neo.Transaction.GetAttributes` syscall.
func GetAttributes(t Transaction) []attribute.Attribute {
	return []attribute.Attribute{}
}

// GetWitnesses returns a slice of witnesses of a given Transaction. Refer to
// witness package on how to use them. This function uses
// `Neo.Transaction.GetWitnesses` syscall.
func GetWitnesses(t Transaction) []witness.Witness {
	return []witness.Witness{}
}
