package blockchain

import (
	"errors"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
)

// Blockchain holds the state of the chain
type Chain struct {
	db      *database.LDB
	headers []*payload.BlockBase
}

func New(db *database.LDB) *Chain {
	return &Chain{
		db,
		nil,
	}
}

// AddHeaders should only be added to from
// the syncmanager, not safe for concurrent access
func (c *Chain) AddHeaders(msg *payload.HeadersMessage) error {
	fmt.Println("Adding Headers into List")
	// iterate headers
	for _, currentHeader := range msg.Headers {

		if len(c.headers) == 0 { // Add the genesis hash on blockchain init, for now just check for nil and add
			c.headers = append(c.headers, currentHeader)
			continue
		}

		// Check if header links and add to list, if not then return an error
		lastHeader := c.headers[len(c.headers)-1]
		lastHeaderHash := lastHeader.Hash

		if currentHeader.PrevHash != lastHeaderHash {
			return errors.New("Last Header hash != current header hash")
		}
		c.headers = append(c.headers, currentHeader)
	}
	return nil
}

func (c *Chain) AddBlock(msg *payload.BlockMessage) error {
	fmt.Println("We have received a Block")
	fmt.Println("TODO : Validate and Verify")
	fmt.Println("Block Hash is ", msg.Hash.String())
	fmt.Println("Number of TXs is ", len(msg.Txs))
	return nil
}
