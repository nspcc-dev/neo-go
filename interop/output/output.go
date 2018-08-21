package output

// Package output provides function signatures that can be used inside
// smart contracts that are written in the neo-go-sc framework.

// Output stubs the output of a NEO transaction.
type Output struct{}

// GetAssetID returns the asset id of the given output.
func GetAssetID(out Output) []byte {
	return nil
}

// GetValue returns the value of the given output.
func GetValue(out Output) int {
	return 0
}

// GetScriptHash returns the script hash of the given output.
func GetScriptHash(out Output) []byte {
	return nil
}
