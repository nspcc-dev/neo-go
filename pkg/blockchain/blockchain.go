package blockchain

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
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

// AddHeaders should only be added to from
// the syncmanager, not safe for concurrent access

func (c *Chain) AddBlock(msg *payload.BlockMessage) error {
	fmt.Println("We have received a Block")
	fmt.Println("TODO : Validate and Verify")
	fmt.Println("Block Hash is ", msg.Hash.String())
	fmt.Println("Number of TXs is ", len(msg.Txs))
	return nil
}
