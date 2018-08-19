package transaction

import "github.com/CityOfZion/neo-go/pkg/core/transaction"

// GetType returns the type of the given transaction.
// TODO: Double check if the type returned should be of type uint8.
func GetType(tx *transaction.Transaction) uint8 { return 0x00 }

// GetTXHash returns the hash of the given transaction.
func GetTXHash(tx *transaction.Transaction) []byte { return nil }

// GetAttributes returns the attributes of the given transaction.
func GetAttributes(tx *transaction.Transaction) []*transaction.Attribute { return nil }

// GetInputs returns the inputs of the given transaction.
func GetInputs(tx *transaction.Transaction) []*transaction.Input { return nil }

// GetOutputs returns the outputs of the given transaction.
func GetOutputs(tx *transaction.Transaction) []*transaction.Output { return nil }

// TODO: What does this return as data type?
// GetReferences returns the outputs of the given transaction.
// func GetReferences(tx *transaction.Transaction) { }

// TODO: What does this return as data type?
// GetUnspentCoins returns the unspent coins of the given transaction.
// func GetUnspentCoins(tx *transaction.Transaction) { }
