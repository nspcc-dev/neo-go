package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
)

// IssueTX represents a issue transaction.
// This TX has not special attributes.
type IssueTX struct{}

// DecodeBinary implements the Payload interface.
func (tx *IssueTX) DecodeBinary(r *io.BinReader) error {
	return nil
}

// EncodeBinary implements the Payload interface.
func (tx *IssueTX) EncodeBinary(w *io.BinWriter) error {
	return nil
}
