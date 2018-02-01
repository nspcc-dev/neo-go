package core

import (
	"encoding/binary"
	"io"

	. "github.com/CityOfZion/neo-go/pkg/util"
)

// Block represents one block in the chain.
type Block struct {
	Version uint32
	// hash of the previous block.
	PrevBlock Uint256
	// Root hash of a transaction list.
	MerkleRoot Uint256
	// timestamp
	Timestamp uint32
	// height of the block
	Height uint32
	// Random number
	Nonce uint64
	// contract addresss of the next miner
	NextMiner Uint160
	// seperator ? fixed to 1
	_sep uint8
	// Script used to validate the block
	Script *Witness
	// transaction list
	Transactions []*Transaction
}

// EncodeBinary encodes the block to the given writer.
func (b *Block) EncodeBinary(w io.Writer) error {
	return nil
}

// DecodeBinary decods the block from the given reader.
func (b *Block) DecodeBinary(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &b.Version)
	err = binary.Read(r, binary.LittleEndian, &b.PrevBlock)
	err = binary.Read(r, binary.LittleEndian, &b.MerkleRoot)
	err = binary.Read(r, binary.LittleEndian, &b.Timestamp)
	err = binary.Read(r, binary.LittleEndian, &b.Height)
	err = binary.Read(r, binary.LittleEndian, &b.Nonce)
	err = binary.Read(r, binary.LittleEndian, &b.NextMiner)
	err = binary.Read(r, binary.LittleEndian, &b._sep)

	b.Script = &Witness{}
	if err := b.Script.DecodeBinary(r); err != nil {
		return err
	}

	var lentx uint8
	err = binary.Read(r, binary.LittleEndian, &lentx)

	b.Transactions = make([]*Transaction, lentx)
	for i := 0; i < int(lentx); i++ {
		tx := &Transaction{}
		if err := tx.DecodeBinary(r); err != nil {
			return err
		}
		b.Transactions[i] = tx
	}

	return err
}

// Size implements the payload interface.
func (b *Block) Size() uint32 { return 0 }
