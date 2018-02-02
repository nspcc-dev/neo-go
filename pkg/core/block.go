package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// BlockBase holds the base info of a block
type BlockBase struct {
	Version uint32
	// hash of the previous block.
	PrevHash util.Uint256
	// Root hash of a transaction list.
	MerkleRoot util.Uint256
	// The time stamp of each block must be later than previous block's time stamp.
	// Generally the difference of two block's time stamp is about 15 seconds and imprecision is allowed.
	// The height of the block must be exactly equal to the height of the previous block plus 1.
	Timestamp uint32
	// index/height of the block
	Index uint32
	// Random number also called nonce
	ConsensusData uint64
	// contract addresss of the next miner
	NextConsensus util.Uint160
	// fixed to 1
	_ uint8 // padding
	// Script used to validate the block
	Script *Witness
}

// DecodeBinary implements the payload interface.
func (b *BlockBase) DecodeBinary(r io.Reader) error {
	binary.Read(r, binary.LittleEndian, &b.Version)
	binary.Read(r, binary.LittleEndian, &b.PrevHash)
	binary.Read(r, binary.LittleEndian, &b.MerkleRoot)
	binary.Read(r, binary.LittleEndian, &b.Timestamp)
	binary.Read(r, binary.LittleEndian, &b.Index)
	binary.Read(r, binary.LittleEndian, &b.ConsensusData)
	binary.Read(r, binary.LittleEndian, &b.NextConsensus)

	var padding uint8
	binary.Read(r, binary.LittleEndian, &padding)
	if padding != 1 {
		return fmt.Errorf("format error: padding must equal 1 got %d", padding)
	}

	b.Script = &Witness{}
	return b.Script.DecodeBinary(r)
}

// Hash return the hash of the block.
// When calculating the hash value of the block, instead of calculating the entire block,
// only first seven fields in the block head will be calculated, which are
// version, PrevBlock, MerkleRoot, timestamp, and height, the nonce, NextMiner.
// Since MerkleRoot already contains the hash value of all transactions,
// the modification of transaction will influence the hash value of the block.
func (b *BlockBase) Hash() (hash util.Uint256, err error) {
	buf := new(bytes.Buffer)
	if err = b.encodeHashableFields(buf); err != nil {
		return
	}

	// Double hash the encoded fields.
	hash = sha256.Sum256(buf.Bytes())
	hash = sha256.Sum256(hash.ToSlice())
	return hash, nil
}

// encodeHashableFields will only encode the fields used for hashing.
// see Hash() for more information about the fields.
func (b *BlockBase) encodeHashableFields(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, &b.Version)
	err = binary.Write(w, binary.LittleEndian, &b.PrevHash)
	err = binary.Write(w, binary.LittleEndian, &b.MerkleRoot)
	err = binary.Write(w, binary.LittleEndian, &b.Timestamp)
	err = binary.Write(w, binary.LittleEndian, &b.Index)
	err = binary.Write(w, binary.LittleEndian, &b.ConsensusData)
	err = binary.Write(w, binary.LittleEndian, &b.NextConsensus)

	return err
}

// Header holds the head info of a block
type Header struct {
	BlockBase
	// fixed to 0
	_ uint8 // padding
}

// DecodeBinary impelements the Payload interface.
func (h *Header) DecodeBinary(r io.Reader) error {
	if err := h.BlockBase.DecodeBinary(r); err != nil {
		return err
	}

	var padding uint8
	binary.Read(r, binary.LittleEndian, &padding)
	if padding != 0 {
		return fmt.Errorf("format error: padding must equal 0 got %d", padding)
	}

	return nil
}

// EncodeBinary  impelements the Payload interface.
func (h *Header) EncodeBinary(w io.Writer) error {
	return nil
}

// Block represents one block in the chain.
type Block struct {
	BlockBase
	// transaction list
	Transactions []*Transaction
}

// Header returns a pointer to the head of the block (BlockHead).
func (b *Block) Header() *Header {
	return &Header{
		BlockBase: b.BlockBase,
	}
}

// Verify the integrity of the block.
func (b *Block) Verify(full bool) bool {
	// The first TX has to be a miner transaction.
	if b.Transactions[0].Type != MinerTX {
		return false
	}

	// If the first TX is a minerTX then all others cant.
	for _, tx := range b.Transactions[1:] {
		if tx.Type == MinerTX {
			return false
		}
	}

	// TODO: When full is true, do a full verification.
	if full {
		log.Println("full verification of blocks is not yet implemented")
	}

	return true
}

// EncodeBinary encodes the block to the given writer.
func (b *Block) EncodeBinary(w io.Writer) error {
	return nil
}

// DecodeBinary decodes the block from the given reader.
func (b *Block) DecodeBinary(r io.Reader) error {
	if err := b.BlockBase.DecodeBinary(r); err != nil {
		return err
	}

	lentx := util.ReadVarUint(r)
	b.Transactions = make([]*Transaction, lentx)
	for i := 0; i < int(lentx); i++ {
		tx := &Transaction{}
		if err := tx.DecodeBinary(r); err != nil {
			return err
		}
		b.Transactions[i] = tx
	}

	return nil
}
