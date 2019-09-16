package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
)

// StateTX represents a state transaction.
type StateTX struct {
	Descriptors []*StateDescriptor
}

// DecodeBinary implements the Payload interface.
func (tx *StateTX) DecodeBinary(r *io.BinReader) error {
	lenDesc := r.ReadVarUint()
	tx.Descriptors = make([]*StateDescriptor, lenDesc)
	for i := 0; i < int(lenDesc); i++ {
		tx.Descriptors[i] = &StateDescriptor{}
		err := tx.Descriptors[i].DecodeBinary(r)
		if err != nil {
			return err
		}
	}
	return r.Err
}

// EncodeBinary implements the Payload interface.
func (tx *StateTX) EncodeBinary(w *io.BinWriter) error {
	w.WriteVarUint(uint64(len(tx.Descriptors)))
	for _, desc := range tx.Descriptors {
		err := desc.EncodeBinary(w)
		if err != nil {
			return err
		}
	}
	return w.Err
}
