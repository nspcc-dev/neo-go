package block

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Base holds the base info of a block
type Base struct {
	// Version of the block.
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

	// Contract address of the next miner
	NextConsensus util.Uint160

	// Padding that is fixed to 1
	_ uint8

	// Script used to validate the block
	Script transaction.Witness

	// Hash of this block, created when binary encoded (double SHA256).
	hash util.Uint256

	// Hash of the block used to verify it (single SHA256).
	verificationHash util.Uint256
}

// baseAux is used to marshal/unmarshal to/from JSON, it's almost the same
// as original Base, but with Nonce and NextConsensus fields differing and
// Hash added.
type baseAux struct {
	Hash          util.Uint256        `json:"hash"`
	Version       uint32              `json:"version"`
	PrevHash      util.Uint256        `json:"previousblockhash"`
	MerkleRoot    util.Uint256        `json:"merkleroot"`
	Timestamp     uint32              `json:"time"`
	Index         uint32              `json:"index"`
	Nonce         string              `json:"nonce"`
	NextConsensus string              `json:"nextconsensus"`
	Script        transaction.Witness `json:"script"`
}

// Verify verifies the integrity of the Base.
func (b *Base) Verify() bool {
	// TODO: Need a persisted blockchain for this.
	return true
}

// Hash returns the hash of the block.
func (b *Base) Hash() util.Uint256 {
	if b.hash.Equals(util.Uint256{}) {
		b.createHash()
	}
	return b.hash
}

// VerificationHash returns the hash of the block used to verify it.
func (b *Base) VerificationHash() util.Uint256 {
	if b.verificationHash.Equals(util.Uint256{}) {
		b.createHash()
	}
	return b.verificationHash
}

// DecodeBinary implements Serializable interface.
func (b *Base) DecodeBinary(br *io.BinReader) {
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
func (b *Base) EncodeBinary(bw *io.BinWriter) {
	b.encodeHashableFields(bw)
	bw.WriteBytes([]byte{1})
	b.Script.EncodeBinary(bw)
}

// GetHashableData returns serialized hashable data of the block.
func (b *Base) GetHashableData() []byte {
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
func (b *Base) createHash() {
	bb := b.GetHashableData()
	b.verificationHash = hash.Sha256(bb)
	b.hash = hash.Sha256(b.verificationHash.BytesBE())
}

// encodeHashableFields will only encode the fields used for hashing.
// see Hash() for more information about the fields.
func (b *Base) encodeHashableFields(bw *io.BinWriter) {
	bw.WriteU32LE(b.Version)
	bw.WriteBytes(b.PrevHash[:])
	bw.WriteBytes(b.MerkleRoot[:])
	bw.WriteU32LE(b.Timestamp)
	bw.WriteU32LE(b.Index)
	bw.WriteU64LE(b.ConsensusData)
	bw.WriteBytes(b.NextConsensus[:])
}

// decodeHashableFields decodes the fields used for hashing.
// see Hash() for more information about the fields.
func (b *Base) decodeHashableFields(br *io.BinReader) {
	b.Version = br.ReadU32LE()
	br.ReadBytes(b.PrevHash[:])
	br.ReadBytes(b.MerkleRoot[:])
	b.Timestamp = br.ReadU32LE()
	b.Index = br.ReadU32LE()
	b.ConsensusData = br.ReadU64LE()
	br.ReadBytes(b.NextConsensus[:])

	// Make the hash of the block here so we dont need to do this
	// again.
	if br.Err == nil {
		b.createHash()
	}
}

// MarshalJSON implements json.Marshaler interface.
func (b Base) MarshalJSON() ([]byte, error) {
	nonce := strconv.FormatUint(b.ConsensusData, 16)
	for len(nonce) < 16 {
		nonce = "0" + nonce
	}
	aux := baseAux{
		Hash:          b.Hash(),
		Version:       b.Version,
		PrevHash:      b.PrevHash,
		MerkleRoot:    b.MerkleRoot,
		Timestamp:     b.Timestamp,
		Index:         b.Index,
		Nonce:         nonce,
		NextConsensus: address.Uint160ToString(b.NextConsensus),
		Script:        b.Script,
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (b *Base) UnmarshalJSON(data []byte) error {
	var aux = new(baseAux)
	var nonce uint64
	var nextC util.Uint160

	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}

	nonce, err = strconv.ParseUint(aux.Nonce, 16, 64)
	if err != nil {
		return err
	}
	nextC, err = address.StringToUint160(aux.NextConsensus)
	if err != nil {
		return err
	}
	b.Version = aux.Version
	b.PrevHash = aux.PrevHash
	b.MerkleRoot = aux.MerkleRoot
	b.Timestamp = aux.Timestamp
	b.Index = aux.Index
	b.ConsensusData = nonce
	b.NextConsensus = nextC
	b.Script = aux.Script
	if !aux.Hash.Equals(b.Hash()) {
		return errors.New("json 'hash' doesn't match block hash")
	}
	return nil
}
