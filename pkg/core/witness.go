package core

import (
	"encoding/binary"
	"io"
)

// Witness ...
type Witness struct {
	InvocationScript   []byte
	VerificationScript []byte
}

// DecodeBinary implements the payload interface.
func (wit *Witness) DecodeBinary(r io.Reader) error {
	var lenb uint8

	err := binary.Read(r, binary.LittleEndian, &lenb)
	wit.InvocationScript = make([]byte, lenb)
	binary.Read(r, binary.LittleEndian, &wit.InvocationScript)
	err = binary.Read(r, binary.LittleEndian, &lenb)
	wit.VerificationScript = make([]byte, lenb)
	binary.Read(r, binary.LittleEndian, &wit.VerificationScript)

	return err
}

// EncodeBinary implements the payload interface.
func (wit *Witness) EncodeBinary(w io.Writer) error {
	return nil
}
