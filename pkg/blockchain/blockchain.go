package blockchain

import (
	"errors"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

var (
	ErrBlockValidation   = errors.New("Block failed sanity check")
	ErrBlockVerification = errors.New("Block failed to be consistent with the current blockchain")
)

// Blockchain holds the state of the chain
type Chain struct {
	db *database.LDB
}

func New(db *database.LDB) *Chain {
	return &Chain{
		db,
	}
}
func (c *Chain) AddBlock(msg *payload.BlockMessage) error {
	fmt.Println("We have received a Block")
	if !validateBlock(msg) {
		return ErrBlockValidation
	}

	if !c.verifyBlock(msg) {
		return ErrBlockVerification
	}

	fmt.Println("Block Hash is ", msg.Hash.String())
	fmt.Println("Number of TXs is ", len(msg.Txs))
	c.db.Put(msg.Hash.Bytes(), []byte("Woh"))
	return nil
}

// validateBlock will check the transactions,
// merkleroot is good, signature is good,every that does not require state
// This may be moved to the syncmanager. This function should not be done in a seperate go-routine
// We are intentionally blocking here because if the block is invalid, we will
// disconnect from the peer.
// We could have this return an error instead; where the error could even
// say where the validation failed, for the logs.
func validateBlock(msg *payload.BlockMessage) bool {
	return true
}

func (c *Chain) verifyBlock(msg *payload.BlockMessage) bool {
	return true
}
