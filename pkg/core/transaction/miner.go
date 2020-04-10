package transaction

import (
	"math/rand"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// MinerTX represents a miner transaction.
type MinerTX struct{}

// NewMinerTX creates Transaction of MinerType type.
func NewMinerTX() *Transaction {
	return NewMinerTXWithNonce(rand.Uint32())
}

// NewMinerTXWithNonce creates Transaction of MinerType type with specified nonce.
func NewMinerTXWithNonce(nonce uint32) *Transaction {
	return &Transaction{
		Type:       MinerType,
		Version:    0,
		Nonce:      nonce,
		Data:       &MinerTX{},
		Attributes: []Attribute{},
		Inputs:     []Input{},
		Outputs:    []Output{},
		Scripts:    []Witness{},
		Trimmed:    false,
	}
}

// DecodeBinary implements Serializable interface.
func (tx *MinerTX) DecodeBinary(r *io.BinReader) {
}

// EncodeBinary implements Serializable interface.
func (tx *MinerTX) EncodeBinary(w *io.BinWriter) {
}
