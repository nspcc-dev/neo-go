package block

import (
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Header holds the base info of a block
type Header struct {
	// Version of the block.
	Version uint32

	// hash of the previous block.
	PrevHash util.Uint256

	// Root hash of a transaction list.
	MerkleRoot util.Uint256

	// Timestamp is a millisecond-precision timestamp.
	// The time stamp of each block must be later than previous block's time stamp.
	// Generally the difference of two block's time stamp is about 15 seconds and imprecision is allowed.
	// The height of the block must be exactly equal to the height of the previous block plus 1.
	Timestamp uint64

	// index/height of the block
	Index uint32

	// Contract address of the next miner
	NextConsensus util.Uint160

	// Script used to validate the block
	Script transaction.Witness

	// StateRootEnabled specifies if header contains state root.
	StateRootEnabled bool
	// PrevStateRoot is state root of the previous block.
	PrevStateRoot util.Uint256
	// PrimaryIndex is the index of primary consensus node for this block.
	PrimaryIndex byte

	// Hash of this block, created when binary encoded (double SHA256).
	hash util.Uint256
}

// baseAux is used to marshal/unmarshal to/from JSON, it's almost the same
// as original Base, but with Nonce and NextConsensus fields differing and
// Hash added.
type baseAux struct {
	Hash          util.Uint256          `json:"hash"`
	Version       uint32                `json:"version"`
	PrevHash      util.Uint256          `json:"previousblockhash"`
	MerkleRoot    util.Uint256          `json:"merkleroot"`
	Timestamp     uint64                `json:"time"`
	Index         uint32                `json:"index"`
	NextConsensus string                `json:"nextconsensus"`
	PrimaryIndex  byte                  `json:"primary"`
	PrevStateRoot *util.Uint256         `json:"previousstateroot,omitempty"`
	Witnesses     []transaction.Witness `json:"witnesses"`
}

// Hash returns the hash of the block.
func (b *Header) Hash() util.Uint256 {
	if b.hash.Equals(util.Uint256{}) {
		b.createHash()
	}
	return b.hash
}

// DecodeBinary implements Serializable interface.
func (b *Header) DecodeBinary(br *io.BinReader) {
	b.decodeHashableFields(br)
	witnessCount := br.ReadVarUint()
	if br.Err == nil && witnessCount != 1 {
		br.Err = errors.New("wrong witness count")
		return
	}

	b.Script.DecodeBinary(br)
}

// EncodeBinary implements Serializable interface
func (b *Header) EncodeBinary(bw *io.BinWriter) {
	b.encodeHashableFields(bw)
	bw.WriteVarUint(1)
	b.Script.EncodeBinary(bw)
}

// createHash creates the hash of the block.
// When calculating the hash value of the block, instead of calculating the entire block,
// only first seven fields in the block head will be calculated, which are
// version, PrevBlock, MerkleRoot, timestamp, and height, the nonce, NextMiner.
// Since MerkleRoot already contains the hash value of all transactions,
// the modification of transaction will influence the hash value of the block.
func (b *Header) createHash() {
	buf := io.NewBufBinWriter()
	// No error can occur while encoding hashable fields.
	b.encodeHashableFields(buf.BinWriter)

	b.hash = hash.Sha256(buf.Bytes())
}

// encodeHashableFields will only encode the fields used for hashing.
// see Hash() for more information about the fields.
func (b *Header) encodeHashableFields(bw *io.BinWriter) {
	bw.WriteU32LE(b.Version)
	bw.WriteBytes(b.PrevHash[:])
	bw.WriteBytes(b.MerkleRoot[:])
	bw.WriteU64LE(b.Timestamp)
	bw.WriteU32LE(b.Index)
	bw.WriteB(b.PrimaryIndex)
	bw.WriteBytes(b.NextConsensus[:])
	if b.StateRootEnabled {
		bw.WriteBytes(b.PrevStateRoot[:])
	}
}

// decodeHashableFields decodes the fields used for hashing.
// see Hash() for more information about the fields.
func (b *Header) decodeHashableFields(br *io.BinReader) {
	b.Version = br.ReadU32LE()
	br.ReadBytes(b.PrevHash[:])
	br.ReadBytes(b.MerkleRoot[:])
	b.Timestamp = br.ReadU64LE()
	b.Index = br.ReadU32LE()
	b.PrimaryIndex = br.ReadB()
	br.ReadBytes(b.NextConsensus[:])
	if b.StateRootEnabled {
		br.ReadBytes(b.PrevStateRoot[:])
	}

	// Make the hash of the block here so we dont need to do this
	// again.
	if br.Err == nil {
		b.createHash()
	}
}

// MarshalJSON implements json.Marshaler interface.
func (b Header) MarshalJSON() ([]byte, error) {
	aux := baseAux{
		Hash:          b.Hash(),
		Version:       b.Version,
		PrevHash:      b.PrevHash,
		MerkleRoot:    b.MerkleRoot,
		Timestamp:     b.Timestamp,
		Index:         b.Index,
		PrimaryIndex:  b.PrimaryIndex,
		NextConsensus: address.Uint160ToString(b.NextConsensus),
		Witnesses:     []transaction.Witness{b.Script},
	}
	if b.StateRootEnabled {
		aux.PrevStateRoot = &b.PrevStateRoot
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (b *Header) UnmarshalJSON(data []byte) error {
	var aux = new(baseAux)
	var nextC util.Uint160

	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}

	nextC, err = address.StringToUint160(aux.NextConsensus)
	if err != nil {
		return err
	}
	if len(aux.Witnesses) != 1 {
		return errors.New("wrong number of witnesses")
	}
	b.Version = aux.Version
	b.PrevHash = aux.PrevHash
	b.MerkleRoot = aux.MerkleRoot
	b.Timestamp = aux.Timestamp
	b.Index = aux.Index
	b.PrimaryIndex = aux.PrimaryIndex
	b.NextConsensus = nextC
	b.Script = aux.Witnesses[0]
	if b.StateRootEnabled {
		if aux.PrevStateRoot == nil {
			return errors.New("'previousstateroot' is empty")
		}
		b.PrevStateRoot = *aux.PrevStateRoot
	}
	if !aux.Hash.Equals(b.Hash()) {
		return errors.New("json 'hash' doesn't match block hash")
	}
	return nil
}
