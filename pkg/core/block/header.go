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

// VersionInitial is the default Neo block version.
const VersionInitial uint32 = 0

// Header holds the base info of a block. Fields follow the P2P format of the
// N3 block header unless noted specifically.
type Header struct {
	// Version of the block, currently only 0.
	Version uint32

	// hash of the previous block.
	PrevHash util.Uint256

	// Root hash of a transaction list.
	MerkleRoot util.Uint256

	// Timestamp is a millisecond-precision timestamp.
	// The time stamp of each block must be later than the previous block's time stamp.
	// Generally, the difference between two block's time stamps is about 15 seconds and imprecision is allowed.
	// The height of the block must be exactly equal to the height of the previous block plus 1.
	Timestamp uint64

	// Nonce is block random number.
	Nonce uint64

	// index/height of the block
	Index uint32

	// Contract address of the next miner
	NextConsensus util.Uint160

	// Witness scripts used for block validation. These scripts
	// are not a part of a hashable field set.
	Script transaction.Witness

	// StateRootEnabled specifies if the header contains state root.
	// It's NeoGo-specific, and is not a part of a standard Neo N3 header,
	// it's also never serialized into P2P payload. When it's false
	// PrevStateRoot is always zero, when true it works as intended.
	StateRootEnabled bool
	// PrevStateRoot is the state root of the previous block. This field
	// is relevant only if StateRootEnabled is true which is a
	// NeoGo-specific extension of the protocol.
	PrevStateRoot util.Uint256
	// PrimaryIndex is the index of the primary consensus node for this block.
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
	Nonce         string                `json:"nonce"`
	Index         uint32                `json:"index"`
	NextConsensus string                `json:"nextconsensus"`
	PrimaryIndex  byte                  `json:"primary"`
	PrevStateRoot *util.Uint256         `json:"previousstateroot,omitempty"`
	Witnesses     []transaction.Witness `json:"witnesses"`
}

// Hash returns the hash of the block. Notice that it is cached internally,
// so no matter how you change the [Header] after the first invocation of this
// method it won't change. To get an updated hash in case you're changing
// the [Header] please encode/decode it.
func (b *Header) Hash() util.Uint256 {
	if b.hash.Equals(util.Uint256{}) {
		b.createHash()
	}
	return b.hash
}

// DecodeBinary implements the [io.Serializable] interface. Notice that it
// also automatically updates the internal hash cache, see [Header.Hash].
func (b *Header) DecodeBinary(br *io.BinReader) {
	b.decodeHashableFields(br)
	witnessCount := br.ReadVarUint()
	if br.Err == nil && witnessCount != 1 {
		br.Err = errors.New("wrong witness count")
		return
	}

	b.Script.DecodeBinary(br)
}

// EncodeBinary implements the Serializable interface.
func (b *Header) EncodeBinary(bw *io.BinWriter) {
	b.encodeHashableFields(bw)
	bw.WriteVarUint(1)
	b.Script.EncodeBinary(bw)
}

// createHash creates the hash of the block.
// When calculating the hash value of the block, instead of processing the entire block,
// only the header (without the signatures) is added as an input for the hash. It differs
// from the complete block only in that it doesn't contain transactions, but their hashes
// are used for MerkleRoot hash calculation. Therefore, adding/removing/changing any
// transaction affects the header hash and there is no need to use the complete block for
// hash calculation.
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
	bw.WriteU64LE(b.Nonce)
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
	b.Nonce = br.ReadU64LE()
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

// MarshalJSON implements the json.Marshaler interface.
func (b Header) MarshalJSON() ([]byte, error) {
	aux := baseAux{
		Hash:          b.Hash(),
		Version:       b.Version,
		PrevHash:      b.PrevHash,
		MerkleRoot:    b.MerkleRoot,
		Timestamp:     b.Timestamp,
		Nonce:         fmt.Sprintf("%016X", b.Nonce),
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

// UnmarshalJSON implements the json.Unmarshaler interface.
func (b *Header) UnmarshalJSON(data []byte) error {
	var aux = new(baseAux)
	var nextC util.Uint160

	err := json.Unmarshal(data, aux)
	if err != nil {
		return err
	}

	var nonce uint64
	if len(aux.Nonce) != 0 {
		nonce, err = strconv.ParseUint(aux.Nonce, 16, 64)
		if err != nil {
			return err
		}
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
	b.Nonce = nonce
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
