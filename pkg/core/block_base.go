package core

import (
	"bytes"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// BlockBase holds the base info of a block
type BlockBase struct {
	// Version of the block.
	Version uint32 `json:"version"`

	// hash of the previous block.
	PrevHash util.Uint256 `json:"previousblockhash"`

	// Root hash of a transaction list.
	MerkleRoot util.Uint256 `json:"merkleroot"`

	// The time stamp of each block must be later than previous block's time stamp.
	// Generally the difference of two block's time stamp is about 15 seconds and imprecision is allowed.
	// The height of the block must be exactly equal to the height of the previous block plus 1.
	Timestamp uint32 `json:"time"`

	// index/height of the block
	Index uint32 `json:"height"`

	// Random number also called nonce
	ConsensusData uint64 `json:"nonce"`

	// Contract address of the next miner
	NextConsensus util.Uint160 `json:"next_consensus"`

	// Padding that is fixed to 1
	_ uint8

	// Script used to validate the block
	Script *transaction.Witness `json:"script"`

	// hash of this block, created when binary encoded.
	hash util.Uint256
}

// Verify verifies the integrity of the BlockBase.
func (b *BlockBase) Verify() bool {
	// TODO: Need a persisted blockchain for this.
	return true
}

// Hash return the hash of the block.
func (b *BlockBase) Hash() util.Uint256 {
	if b.hash.Equals(util.Uint256{}) {
		b.createHash()
	}
	return b.hash
}

// DecodeBinary implements the payload interface.
func (b *BlockBase) DecodeBinary(r io.Reader) error {
	if err := b.decodeHashableFields(r); err != nil {
		return err
	}

	var padding uint8
	br := util.NewBinReaderFromIO(r)
	br.ReadLE(&padding)
	if br.Err != nil {
		return br.Err
	}
	if padding != 1 {
		return fmt.Errorf("format error: padding must equal 1 got %d", padding)
	}

	b.Script = &transaction.Witness{}
	return b.Script.DecodeBinary(r)
}

// EncodeBinary implements the Payload interface
func (b *BlockBase) EncodeBinary(w io.Writer) error {
	if err := b.encodeHashableFields(w); err != nil {
		return err
	}
	bw := util.NewBinWriterFromIO(w)
	bw.WriteLE(uint8(1))
	if bw.Err != nil {
		return bw.Err
	}
	return b.Script.EncodeBinary(w)
}

// createHash creates the hash of the block.
// When calculating the hash value of the block, instead of calculating the entire block,
// only first seven fields in the block head will be calculated, which are
// version, PrevBlock, MerkleRoot, timestamp, and height, the nonce, NextMiner.
// Since MerkleRoot already contains the hash value of all transactions,
// the modification of transaction will influence the hash value of the block.
func (b *BlockBase) createHash() error {
	buf := new(bytes.Buffer)
	if err := b.encodeHashableFields(buf); err != nil {
		return err
	}
	b.hash = hash.DoubleSha256(buf.Bytes())

	return nil
}

// encodeHashableFields will only encode the fields used for hashing.
// see Hash() for more information about the fields.
func (b *BlockBase) encodeHashableFields(w io.Writer) error {
	bw := util.NewBinWriterFromIO(w)
	bw.WriteLE(b.Version)
	bw.WriteLE(b.PrevHash)
	bw.WriteLE(b.MerkleRoot)
	bw.WriteLE(b.Timestamp)
	bw.WriteLE(b.Index)
	bw.WriteLE(b.ConsensusData)
	bw.WriteLE(b.NextConsensus)
	return bw.Err
}

// decodeHashableFields will only decode the fields used for hashing.
// see Hash() for more information about the fields.
func (b *BlockBase) decodeHashableFields(r io.Reader) error {
	br := util.NewBinReaderFromIO(r)
	br.ReadLE(&b.Version)
	br.ReadLE(&b.PrevHash)
	br.ReadLE(&b.MerkleRoot)
	br.ReadLE(&b.Timestamp)
	br.ReadLE(&b.Index)
	br.ReadLE(&b.ConsensusData)
	br.ReadLE(&b.NextConsensus)

	if br.Err != nil {
		return br.Err
	}

	// Make the hash of the block here so we dont need to do this
	// again.
	return b.createHash()
}
