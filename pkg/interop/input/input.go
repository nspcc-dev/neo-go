/*
Package input provides functions dealing with transaction inputs.
*/
package input

// Input is an opaque data structure that can only be created by
// transaction.GetInputs and it represents transaction's input. It's similar
// to Neo .net framework's TransactionInput.
type Input struct{}

// GetHash returns the hash stored in the given input (which also is a
// transaction ID represented as 32 byte slice containing 256 bit BE value).
// It uses `Neo.Input.GetHash` syscall.
func GetHash(in Input) []byte {
	return nil
}

// GetIndex returns the index stored in the given input (which is a
// transaction's output number). It uses `Neo.Input.GetIndex` syscall.
func GetIndex(in Input) int {
	return 0
}
