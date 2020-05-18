/*
Package output provides functions dealing with transaction outputs.
*/
package output

// Output is an opaque data structure that can only be created by
// transaction.GetOutputs and it represents transaction's output. It's similar
// to Neo .net framework's TransactionOutput.
type Output struct{}

// GetAssetID returns the asset ID (256 bit BE value in a 32 byte slice) of the
// given output. It uses `Neo.Output.GetAssetId` syscall.
func GetAssetID(out Output) []byte {
	return nil
}

// GetValue returns the value (asset quantity) of the given output. It uses
// `Neo.Output.GetValue` syscall.
func GetValue(out Output) int {
	return 0
}

// GetScriptHash returns the script hash (receiver's address represented as
// 20 byte slice containing 160 bit BE value) of the given output. It uses
// `Neo.Output.GetScriptHash` syscall.
func GetScriptHash(out Output) []byte {
	return nil
}
