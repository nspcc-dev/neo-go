package blockchain

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/chainparams"

	"github.com/CityOfZion/neo-go/pkg/database"
	"github.com/CityOfZion/neo-go/pkg/wire/payload"
	"github.com/CityOfZion/neo-go/pkg/wire/protocol"
)

var (
	ErrBlockValidation   = errors.New("Block failed sanity check")
	ErrBlockVerification = errors.New("Block failed to be consistent with the current blockchain")
)

// Blockchain holds the state of the chain
type Chain struct {
	db  *database.LDB
	net protocol.Magic
}

func New(db *database.LDB, net protocol.Magic) *Chain {

	marker := []byte("HasBeenInitialisedAlready")

	_, err := db.Get(marker)

	if err != nil {
		// This is a new db
		fmt.Println("New Database initialisation")
		db.Put(marker, []byte{})

		// We add the genesis block into the db
		// along with the indexes for it
		if net == protocol.MainNet {

			genesisBlock, err := hex.DecodeString(chainparams.GenesisBlock)
			if err != nil {
				fmt.Println("Could not add genesis header into db")
				db.Delete(marker)
				return nil
			}
			r := bytes.NewReader(genesisBlock)
			b := payload.Block{}
			err = b.Decode(r)

			if err != nil {
				fmt.Println("could not Decode genesis block")
				db.Delete(marker)
				return nil
			}
			err = db.AddHeader(&b.BlockBase)
			if err != nil {
				fmt.Println("Could not add genesis header")
				db.Delete(marker)
				return nil
			}
			err = db.AddTransactions(b.Hash, b.Txs)
			if err != nil {
				fmt.Println("Could not add Genesis Transactions")
				db.Delete(marker)
				return nil
			}
		}
		if net == protocol.TestNet {
			fmt.Println("TODO: Setup the genesisBlock for TestNet")
			return nil
		}

	}
	return &Chain{
		db,
		net,
	}
}
func (c *Chain) AddBlock(msg *payload.BlockMessage) error {
	if !validateBlock(msg) {
		return ErrBlockValidation
	}

	if !c.verifyBlock(msg) {
		return ErrBlockVerification
	}

	fmt.Println("Block Hash is ", msg.Hash.String())

	buf := new(bytes.Buffer)
	err := msg.Encode(buf)
	if err != nil {
		return err
	}
	return c.db.Put(msg.Hash.Bytes(), buf.Bytes())

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

// This will add a header into the db,
// indexing it also, this method will not
// run any checks, like if it links with a header
// previously in the db
// func (c *Chain) addHeaderNoCheck(header *payload.BlockBase) error {

// }

//addHeaders is not safe for concurrent access
func (c *Chain) ValidateHeaders(msg *payload.HeadersMessage) error {

	table := database.NewTable(c.db, database.HEADER)

	latestHash, err := table.Get(database.LATESTHEADER)
	if err != nil {
		return err
	}

	key := latestHash
	val, err := table.Get(key)

	lastHeader := &payload.BlockBase{}
	err = lastHeader.Decode(bytes.NewReader(val))
	if err != nil {
		return err
	}

	// TODO?:Maybe we should sort these headers using the Index
	// We should not get them in mixed order, but doing it would not be expensive
	// If they are already in order

	// Do checks on headers
	for _, currentHeader := range msg.Headers {

		if lastHeader == nil {
			// This should not happen as genesis header is added if new
			// database, however we check nonetheless
			return errors.New("Previous Header is nil")
		}

		// Check current hash links with previous
		if currentHeader.PrevHash != lastHeader.Hash {
			return errors.New("Last Header hash != current header Prev hash")
		}

		// Check current Index is one more than the previous Index
		if currentHeader.Index != lastHeader.Index+1 {
			return errors.New("Last Header Index != current header Index")
		}

		// Check current timestamp is more than the previous header's timestamp
		if lastHeader.Timestamp > currentHeader.Timestamp {
			return errors.New("Timestamp of Previous Header is more than Timestamp of current Header")
		}

		// NONONO:Do not check if current is more than 15 secs in future
		// some blocks had delay from forks in past.

		// NOTE: These are the only non-contextual checks we can do without the blockchain state
		lastHeader = currentHeader
	}
	return nil
}

func (c *Chain) AddHeaders(msg *payload.HeadersMessage) error {
	for _, header := range msg.Headers {
		if err := c.db.AddHeader(header); err != nil {
			return err
		}
	}
	return nil
}
