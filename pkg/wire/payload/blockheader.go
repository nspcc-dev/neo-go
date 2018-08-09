package payload

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

var (
	ErrPadding = errors.New("There is a padding mismatch")
)

type BlockHeader struct {
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

	// Contract addresss of the next miner
	NextConsensus util.Uint160 `json:"next_consensus"`

	// Padding that is fixed to 1
	_ uint8

	// Script used to validate the block
	Witness transaction.Witness `json:"script"`

	// hash of this block, created when binary encoded.
	Hash util.Uint256

	// Padding that is fixed to 0
	_ uint8
}

func (b *BlockHeader) EncodePayload(bw *util.BinWriter) error {

	b.encodeHashableFields(bw)

	bw.Write(uint8(1))
	b.Witness.Encode(bw)
	bw.Write(uint8(0))

	return bw.Err
}

func (b *BlockHeader) encodeHashableFields(bw *util.BinWriter) {
	bw.Write(b.Version)
	bw.Write(b.PrevHash)
	bw.Write(b.MerkleRoot)
	bw.Write(b.Timestamp)
	bw.Write(b.Index)
	bw.Write(b.ConsensusData)
	bw.Write(b.NextConsensus)
}

func (b *BlockHeader) DecodePayload(br *util.BinReader) error {

	b.decodeHashableFields(br)

	var padding uint8
	br.Read(&padding)
	if padding != 1 {
		return ErrPadding
	}

	b.Witness = transaction.Witness{}
	b.Witness.Decode(br)

	br.Read(&padding)
	if padding != 0 {
		return ErrPadding
	}
	return br.Err
}

func (b *BlockHeader) decodeHashableFields(br *util.BinReader) {
	br.Read(&b.Version)
	br.Read(&b.PrevHash)
	br.Read(&b.MerkleRoot)
	br.Read(&b.Timestamp)
	br.Read(&b.Index)
	br.Read(&b.ConsensusData)
	br.Read(&b.NextConsensus)
}

func (b *BlockHeader) createHash() error {

	hash, err := util.CalculateHash(b.encodeHashableFields)
	b.Hash = hash
	return err
}
