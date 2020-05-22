package transaction

import (
	"math/rand"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// ClaimTX represents a claim transaction.
type ClaimTX struct {
	Claims []Input
}

// NewClaimTX creates Transaction of ClaimType type.
func NewClaimTX(claim *ClaimTX) *Transaction {
	return &Transaction{
		Type:       ClaimType,
		Version:    0,
		Nonce:      rand.Uint32(),
		Data:       claim,
		Attributes: []Attribute{},
		Cosigners:  []Cosigner{},
		Inputs:     []Input{},
		Outputs:    []Output{},
		Scripts:    []Witness{},
		Trimmed:    false,
	}
}

// DecodeBinary implements Serializable interface.
func (tx *ClaimTX) DecodeBinary(br *io.BinReader) {
	br.ReadArray(&tx.Claims)
}

// EncodeBinary implements Serializable interface.
func (tx *ClaimTX) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(tx.Claims)
}
