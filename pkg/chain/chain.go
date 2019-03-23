package chain

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"

	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

var (
	// ErrBlockAlreadyExists happens when you try to save the same block twice
	ErrBlockAlreadyExists = errors.New("this block has already been saved in the database")
)

// Chain represents a blockchain instance
type Chain struct {
	db *Chaindb
}

//New returns a new chain instance
func New(db database.Database) *Chain {
	return &Chain{
		db: &Chaindb{db},
	}
}

// SaveBlock verifies and saves the block in the database
// XXX: for now we will just save without verifying the block
// This function is called by the server and if an error is returned then
// the server informs the sync manager to redownload the block
// XXX:We should also check if the header is already saved in the database
// If not, then we need to validate the header with the rest of the chain
// For now we re-save the header
func (c *Chain) SaveBlock(msg payload.BlockMessage) error {
	err := c.VerifyBlock(msg.Block)
	if err != nil {
		return err
	}
	//XXX(Issue): We can either check the hash here for genesisblock.
	//We most likely will have it anyways after validation/ We can return it from VerifyBlock
	// Or we can do it somewhere in startup, performance benefits
	// won't be that big since it's just a bytes.Equal.
	// so it's more about which is more readable and where it makes sense to put
	return c.db.saveBlock(msg.Block, false)
}

// VerifyBlock verifies whether a block is valid according
// to the rules of consensus
func (c *Chain) VerifyBlock(block payload.Block) error {

	// Check if we already have this block
	// XXX: We can optimise by implementing a Has method
	// caching the last block in memory
	lastBlock, err := c.db.GetLastBlock()
	if err != nil {
		return err
	}
	// Check if we have already saved this block
	// by looking if the latest block height is more than
	// incoming block height
	if lastBlock.Index > block.Index {
		return ErrBlockAlreadyExists
	}

	return nil
}

// VerifyTx verifies whether a transaction is valid according
// to the rules of consensus
func (c *Chain) VerifyTx(tx transaction.Transactioner) error {
	return nil
}

// SaveHeaders will save the set of headers without validating
func (c *Chain) SaveHeaders(msg payload.HeadersMessage) error {

	err := c.verifyHeaders(msg.Headers)
	if err != nil {
		return err
	}
	return c.db.saveHeaders(msg.Headers)
}

// verifyHeaders will be used to verify a batch of headers
// should only ever be called during the initial block download
// or when the node receives a HeadersMessage
func (c *Chain) verifyHeaders(hdrs []*payload.BlockBase) error {
	return nil
}
