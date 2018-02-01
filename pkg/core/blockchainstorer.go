package core

// BlockchainStorer is anything that can persist and retrieve the blockchain.
type BlockchainStorer interface {
	// To take all in consideration, trying to get some puzzle pieces together here.
	Persist(*Block) error
	GetBlockByHeight(uint32) (*Block, error)
	GetBlockByHash(uint32) (*Block, error)
}
