package core

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/io"
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
	Script transaction.Witness `json:"script"`

	// Hash of this block, created when binary encoded (double SHA256).
	hash util.Uint256

	// Hash of the block used to verify it (single SHA256).
	verificationHash util.Uint256
}

// Verify verifies the integrity of the BlockBase.
func (b *BlockBase) Verify() bool {
	// TODO: Need a persisted blockchain for this.
	return true
}

// Hash returns the hash of the block.
func (b *BlockBase) Hash() util.Uint256 {
	if b.hash.Equals(util.Uint256{}) {
		b.createHash()
	}
	return b.hash
}

// VerificationHash returns the hash of the block used to verify it.
func (b *BlockBase) VerificationHash() util.Uint256 {
	if b.verificationHash.Equals(util.Uint256{}) {
		b.createHash()
	}
	return b.verificationHash
}

// DecodeBinary implements Serializable interface.
func (b *BlockBase) DecodeBinary(br *io.BinReader) {
	b.decodeHashableFields(br)

	padding := []byte{0}
	br.ReadBytes(padding)
	if padding[0] != 1 {
		br.Err = fmt.Errorf("format error: padding must equal 1 got %d", padding)
		return
	}

	b.Script.DecodeBinary(br)
}

// EncodeBinary implements Serializable interface
func (b *BlockBase) EncodeBinary(bw *io.BinWriter) {
	b.encodeHashableFields(bw)
	bw.WriteBytes([]byte{1})
	b.Script.EncodeBinary(bw)
}

// GetHashableData returns serialized hashable data of the block.
func (b *BlockBase) GetHashableData() []byte {
	buf := io.NewBufBinWriter()
	// No error can occure while encoding hashable fields.
	b.encodeHashableFields(buf.BinWriter)

	return buf.Bytes()
}

// createHash creates the hash of the block.
// When calculating the hash value of the block, instead of calculating the entire block,
// only first seven fields in the block head will be calculated, which are
// version, PrevBlock, MerkleRoot, timestamp, and height, the nonce, NextMiner.
// Since MerkleRoot already contains the hash value of all transactions,
// the modification of transaction will influence the hash value of the block.
func (b *BlockBase) createHash() {
	bb := b.GetHashableData()
	b.verificationHash = hash.Sha256(bb)
	b.hash = hash.Sha256(b.verificationHash.BytesBE())
}

// encodeHashableFields will only encode the fields used for hashing.
// see Hash() for more information about the fields.
func (b *BlockBase) encodeHashableFields(bw *io.BinWriter) {
	bw.WriteLE(b.Version)
	bw.WriteBytes(b.PrevHash[:])
	bw.WriteBytes(b.MerkleRoot[:])
	bw.WriteLE(b.Timestamp)
	bw.WriteLE(b.Index)
	bw.WriteLE(b.ConsensusData)
	bw.WriteBytes(b.NextConsensus[:])
}

// decodeHashableFields decodes the fields used for hashing.
// see Hash() for more information about the fields.
func (b *BlockBase) decodeHashableFields(br *io.BinReader) {
	br.ReadLE(&b.Version)
	br.ReadBytes(b.PrevHash[:])
	br.ReadBytes(b.MerkleRoot[:])
	br.ReadLE(&b.Timestamp)
	br.ReadLE(&b.Index)
	br.ReadLE(&b.ConsensusData)
	br.ReadBytes(b.NextConsensus[:])

	// Make the hash of the block here so we dont need to do this
	// again.
	if br.Err == nil {
		b.createHash()
	}
}
