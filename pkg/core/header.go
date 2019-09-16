package core

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/io"
)

// Header holds the head info of a block.
type Header struct {
	// Base of the block.
	BlockBase
	// Padding that is fixed to 0
	_ uint8
}

// DecodeBinary implements Serializable interface.
func (h *Header) DecodeBinary(r *io.BinReader) {
	h.BlockBase.DecodeBinary(r)

	var padding uint8
	r.ReadLE(&padding)

	if padding != 0 {
		r.Err = fmt.Errorf("format error: padding must equal 0 got %d", padding)
	}
}

// EncodeBinary  implements Serializable interface.
func (h *Header) EncodeBinary(w *io.BinWriter) {
	h.BlockBase.EncodeBinary(w)
	w.WriteLE(uint8(0))
}
