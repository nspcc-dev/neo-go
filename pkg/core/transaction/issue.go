package transaction

import (
	"math/rand"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// IssueTX represents a issue transaction.
// This TX has not special attributes.
type IssueTX struct{}

// NewIssueTX creates Transaction of IssueType type.
func NewIssueTX() *Transaction {
	return &Transaction{
		Type:       IssueType,
		Version:    0,
		Nonce:      rand.Uint32(),
		Data:       &IssueTX{},
		Attributes: []Attribute{},
		Cosigners:  []Cosigner{},
		Inputs:     []Input{},
		Outputs:    []Output{},
		Scripts:    []Witness{},
		Trimmed:    false,
	}
}

// DecodeBinary implements Serializable interface.
func (tx *IssueTX) DecodeBinary(r *io.BinReader) {
}

// EncodeBinary implements Serializable interface.
func (tx *IssueTX) EncodeBinary(w *io.BinWriter) {
}
