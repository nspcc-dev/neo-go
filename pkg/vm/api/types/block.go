package types

// Block represents a block in the blockchain.
type Block struct{}

// Index returns the height of the block.
func (b Block) Index() int {
	return 0
}
