package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
)

// StateTX represents a state transaction.
type StateTX struct {
	Descriptors []*StateDescriptor
}

// DecodeBinary implements Serializable interface.
func (tx *StateTX) DecodeBinary(r *io.BinReader) {
	tx.Descriptors = r.ReadArray(StateDescriptor{}).([]*StateDescriptor)
}

// EncodeBinary implements Serializable interface.
func (tx *StateTX) EncodeBinary(w *io.BinWriter) {
	w.WriteArray(tx.Descriptors)
}
