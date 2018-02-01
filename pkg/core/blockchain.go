package core

// tuning parameters
const (
	secondsPerBlock = 15
)

var (
	genAmount = []int{8, 7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
)

// Blockchain holds the chain.
type Blockchain struct {
	// Any object that satisfies the BlockchainStorer interface.
	BlockchainStorer

	// index of the latest block.
	currentHeight uint32
}

// NewBlockchain returns a pointer to a Blockchain.
func NewBlockchain(store BlockchainStorer) *Blockchain {
	return &Blockchain{
		BlockchainStorer: store,
	}
}
