package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"

	. "github.com/CityOfZion/neo-go/pkg/util"
)

// BlockBase holds the base info of a block
type BlockBase struct {
	Version uint32
	// hash of the previous block.
	PrevBlock Uint256
	// Root hash of a transaction list.
	MerkleRoot Uint256
	// The time stamp of each block must be later than previous block's time stamp.
	// Generally the difference of two block's time stamp is about 15 seconds and imprecision is allowed.
	// The height of the block must be exactly equal to the height of the previous block plus 1.
	Timestamp uint32
	// height of the block
	Height uint32
	// Random number
	Nonce uint64
	// contract addresss of the next miner
	NextMiner Uint160
	// fixed to 1
	_sep uint8
	// Script used to validate the block
	Script *Witness
}

// BlockHead holds the head info of a block
type BlockHead struct {
	BlockBase
	// fixed to 0
	_sep1 uint8
}

// Block represents one block in the chain.
type Block struct {
	BlockBase
	// transaction list
	Transactions []*Transaction
}

// encodeHashableFields will only encode the fields used for hashing.
// see Hash() for more information about the fields.
func (b *Block) encodeHashableFields(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, &b.Version)
	err = binary.Write(w, binary.LittleEndian, &b.PrevBlock)
	err = binary.Write(w, binary.LittleEndian, &b.MerkleRoot)
	err = binary.Write(w, binary.LittleEndian, &b.Timestamp)
	err = binary.Write(w, binary.LittleEndian, &b.Height)
	err = binary.Write(w, binary.LittleEndian, &b.Nonce)
	err = binary.Write(w, binary.LittleEndian, &b.NextMiner)

	return err
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

// Hash return the hash of the block.
// When calculating the hash value of the block, instead of calculating the entire block,
// only first seven fields in the block head will be calculated, which are
// version, PrevBlock, MerkleRoot, timestamp, and height, the nonce, NextMiner.
// Since MerkleRoot already contains the hash value of all transactions,
// the modification of transaction will influence the hash value of the block.
func (b *Block) Hash() (hash Uint256, err error) {
	buf := new(bytes.Buffer)
	if err = b.encodeHashableFields(buf); err != nil {
		return
	}
	hash = sha256.Sum256(buf.Bytes())
	return
}

// Size implements the payload interface.
func (b *Block) Size() uint32 { return 0 }
