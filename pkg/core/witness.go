package core

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Witness ...
type Witness struct {
	InvocationScript   []byte
	VerificationScript []byte
}

// DecodeBinary implements the payload interface.
func (wit *Witness) DecodeBinary(r io.Reader) error {
	lenb := util.ReadVarUint(r)
	wit.InvocationScript = make([]byte, lenb)
	if err := binary.Read(r, binary.LittleEndian, &wit.InvocationScript); err != nil {
		panic(err)
	}

	lenb = util.ReadVarUint(r)
	wit.VerificationScript = make([]byte, lenb)
	if err := binary.Read(r, binary.LittleEndian, &wit.VerificationScript); err != nil {
		panic(err)
	}

	return nil
}

// EncodeBinary implements the payload interface.
func (wit *Witness) EncodeBinary(w io.Writer) error {
	return nil
}
