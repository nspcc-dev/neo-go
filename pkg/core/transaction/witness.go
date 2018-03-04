package transaction

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Witness contains 2 scripts.
type Witness struct {
	InvocationScript   []byte
	VerificationScript []byte
}

// DecodeBinary implements the payload interface.
func (wit *Witness) DecodeBinary(r io.Reader) error {
	lenb := util.ReadVarUint(r)
	wit.InvocationScript = make([]byte, lenb)
	if err := binary.Read(r, binary.LittleEndian, wit.InvocationScript); err != nil {
		return err
	}
	lenb = util.ReadVarUint(r)
	wit.VerificationScript = make([]byte, lenb)
	return binary.Read(r, binary.LittleEndian, wit.VerificationScript)
}

// EncodeBinary implements the payload interface.
func (wit *Witness) EncodeBinary(w io.Writer) error {
	util.WriteVarUint(w, uint64(len(wit.InvocationScript)))
	if err := binary.Write(w, binary.LittleEndian, wit.InvocationScript); err != nil {
		return err
	}
	util.WriteVarUint(w, uint64(len(wit.VerificationScript)))
	return binary.Write(w, binary.LittleEndian, wit.VerificationScript)
}
