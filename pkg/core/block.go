package core

import (
	"encoding/binary"
	"io"

	. "github.com/anthdm/neo-go/pkg/util"
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
	Script []byte
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
	var n uint8
	err = binary.Read(r, binary.LittleEndian, &n)
	err = binary.Read(r, binary.LittleEndian, &n)

	// txs := make([]byte, n)
	// err = binary.Read(r, binary.LittleEndian, &txs)
	// err = binary.Read(r, binary.LittleEndian, &n)
	// fmt.Println(n)

	return err
}

// Size implements the payload interface.
func (b *Block) Size() uint32 { return 0 }
