package transaction

import (
	"io"
)

// IssueTX represents a issue transaction.
// This TX has not special attributes.
type IssueTX struct{}

// DecodeBinary implements the Payload interface.
func (tx *IssueTX) DecodeBinary(r io.Reader) error {
	return nil
}

// EncodeBinary implements the Payload interface.
func (tx *IssueTX) EncodeBinary(w io.Writer) error {
	return nil
}

// Size returns serialized binary size for this transaction.
func (tx *IssueTX) Size() int {
	return 0
}
