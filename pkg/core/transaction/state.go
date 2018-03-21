package transaction

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// StateTX represents a state transaction.
type StateTX struct {
	Descriptors []*StateDescriptor
}

// DecodeBinary implements the Payload interface.
func (tx *StateTX) DecodeBinary(r io.Reader) error {
	lenDesc := util.ReadVarUint(r)
	for i := 0; i < int(lenDesc); i++ {
		tx.Descriptors[i] = &StateDescriptor{}
		if err := tx.Descriptors[i].DecodeBinary(r); err != nil {
			return err
		}
	}
	return nil
}

// EncodeBinary implements the Payload interface.
func (tx *StateTX) EncodeBinary(w io.Writer) error {
	return nil
}
