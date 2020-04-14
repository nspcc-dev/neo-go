package transaction

import (
	"math/rand"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// StateTX represents a state transaction.
type StateTX struct {
	Descriptors []*StateDescriptor
}

// NewStateTX creates Transaction of StateType type.
func NewStateTX(state *StateTX) *Transaction {
	return &Transaction{
		Type:       StateType,
		Version:    0,
		Nonce:      rand.Uint32(),
		Data:       state,
		Attributes: []Attribute{},
		Inputs:     []Input{},
		Outputs:    []Output{},
		Scripts:    []Witness{},
		Trimmed:    false,
	}
}

// DecodeBinary implements Serializable interface.
func (tx *StateTX) DecodeBinary(r *io.BinReader) {
	r.ReadArray(&tx.Descriptors)
}

// EncodeBinary implements Serializable interface.
func (tx *StateTX) EncodeBinary(w *io.BinWriter) {
	w.WriteArray(tx.Descriptors)
}
