package chain

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/CityOfZion/neo-go/pkg/chaincfg"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"

	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

var (
	// ErrBlockAlreadyExists happens when you try to save the same block twice
	ErrBlockAlreadyExists = errors.New("this block has already been saved in the database")

	// ErrFutureBlock happens when you try to save a block that is not the next block sequentially
	ErrFutureBlock = errors.New("this is not the next block sequentially, that should be added to the chain")
)

// Chain represents a blockchain instance
type Chain struct {
	Db     *Chaindb
	height uint32
}

// New returns a new chain instance
func New(db database.Database, magic protocol.Magic) (*Chain, error) {

	chain := &Chain{
		Db: &Chaindb{db},
	}

	// Get last header saved to see if this is a fresh database
	_, err := chain.Db.GetLastHeader()
	if err == nil {
		return chain, nil
	}

	if err != database.ErrNotFound {
		return nil, err
	}

	// We have a database.ErrNotFound. Insert the genesisBlock
	fmt.Printf("Starting a fresh database for %s\n", magic.String())

	params, err := chaincfg.NetParams(magic)
	if err != nil {
		return nil, err
	}
	err = chain.Db.saveHeader(&params.GenesisBlock.BlockBase)
	if err != nil {
		return nil, err
	}
	err = chain.Db.saveBlock(params.GenesisBlock, true)
	if err != nil {
		return nil, err
	}
	return chain, nil
}

// ProcessBlock verifies and saves the block in the database
// XXX: for now we will just save without verifying the block
// This function is called by the server and if an error is returned then
// the server informs the sync manager to redownload the block
// XXX:We should also check if the header is already saved in the database
// If not, then we need to validate the header with the rest of the chain
// For now we re-save the header
func (c *Chain) ProcessBlock(block payload.Block) error {

	// Check if we already have this block saved
	// XXX: We can optimise by implementing a Has() method
	// caching the last block in memory
	lastBlock, err := c.Db.GetLastBlock()
	if err != nil {
		return err
	}
	if lastBlock.Index > block.Index {
		return ErrBlockAlreadyExists
	}

	if block.Index > lastBlock.Index+1 {
		return ErrFutureBlock
	}

	err = c.verifyBlock(block)
	if err != nil {
		return ValidationError{err.Error()}
	}
	err = c.Db.saveBlock(block, false)
	if err != nil {
		return DatabaseError{err.Error()}
	}
	return nil
}

// VerifyBlock verifies whether a block is valid according
// to the rules of consensus
func (c *Chain) verifyBlock(block payload.Block) error {
	return nil
}

// VerifyTx verifies whether a transaction is valid according
// to the rules of consensus
func (c *Chain) VerifyTx(tx transaction.Transactioner) error {
	return nil
}

// ProcessHeaders will save the set of headers without validating
func (c *Chain) ProcessHeaders(hdrs []*payload.BlockBase) error {

	err := c.verifyHeaders(hdrs)
	if err != nil {
		return ValidationError{err.Error()}
	}
	err = c.Db.saveHeaders(hdrs)
	if err != nil {
		return DatabaseError{err.Error()}
	}
	return nil
}

// verifyHeaders will be used to verify a batch of headers
// should only ever be called during the initial block download
// or when the node receives a HeadersMessage
func (c *Chain) verifyHeaders(hdrs []*payload.BlockBase) error {
	return nil
}

// CurrentHeight returns the index of the block
// at the tip of the chain
func (c Chain) CurrentHeight() uint32 {
	return c.height
}
