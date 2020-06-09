package block

// Block is opaque type.
type Block struct{}

// GetTransactionCount is a mirror of `GetTransactionCount` interop.
func GetTransactionCount(b Block) int {
	return 42
}
