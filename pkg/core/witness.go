package core

import "io"

// Witness ...
type Witness struct {
	InvocationScript   []byte
	VerificationScript []byte
}

// DecodeBinary implements the payload interface.
func (wit *Witness) DecodeBinary(r io.Reader) error {
	return nil
}

// EncodeBinary implements the payload interface.
func (wit *Witness) EncodeBinary(w io.Writer) error {
	return nil
}
