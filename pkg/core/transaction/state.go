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
	lenDesc := r.ReadVarUint()
	tx.Descriptors = make([]*StateDescriptor, lenDesc)
	for i := 0; i < int(lenDesc); i++ {
		tx.Descriptors[i] = &StateDescriptor{}
		tx.Descriptors[i].DecodeBinary(r)
	}
}

// EncodeBinary implements Serializable interface.
func (tx *StateTX) EncodeBinary(w *io.BinWriter) {
	w.WriteVarUint(uint64(len(tx.Descriptors)))
	for _, desc := range tx.Descriptors {
		desc.EncodeBinary(w)
	}
}
