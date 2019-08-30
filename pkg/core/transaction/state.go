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
	br := util.BinReader{R: r}
	lenDesc := br.ReadVarUint()
	if br.Err != nil {
		return br.Err
	}
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
	bw := util.BinWriter{W: w}
	bw.WriteVarUint(uint64(len(tx.Descriptors)))
	if bw.Err != nil {
		return bw.Err
	}
	for _, desc := range tx.Descriptors {
		err := desc.EncodeBinary(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tx *StateTX) Size() int {
	sz := util.GetVarSize(uint64(len(tx.Descriptors)))
	for _, desc := range tx.Descriptors {
		sz += desc.Size()
	}
	return sz
}
