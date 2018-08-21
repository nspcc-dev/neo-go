package input

// Package input provides function signatures that can be used inside
// smart contracts that are written in the neo-go-sc framework.

// Input stubs the input of a NEO transaction.
type Input struct{}

// GetHash returns the hash of the given input.
func GetHash(in Input) []byte {
	return nil
}

// GetIndex returns the index of the given input.
func GetIndex(in Input) int {
	return 0
}
