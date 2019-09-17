package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
)

// IssueTX represents a issue transaction.
// This TX has not special attributes.
type IssueTX struct{}

// DecodeBinary implements Serializable interface.
func (tx *IssueTX) DecodeBinary(r *io.BinReader) {
}

// EncodeBinary implements Serializable interface.
func (tx *IssueTX) EncodeBinary(w *io.BinWriter) {
}
