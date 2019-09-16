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

// DecodeBinary implements the Payload interface.
func (h *Header) DecodeBinary(r *io.BinReader) error {
	if err := h.BlockBase.DecodeBinary(r); err != nil {
		return err
	}

	var padding uint8
	r.ReadLE(&padding)
	if r.Err != nil {
		return r.Err
	}

	if padding != 0 {
		return fmt.Errorf("format error: padding must equal 0 got %d", padding)
	}

	return nil
}

// EncodeBinary  implements the Payload interface.
func (h *Header) EncodeBinary(w *io.BinWriter) error {
	h.BlockBase.EncodeBinary(w)
	w.WriteLE(uint8(0))
	return w.Err
}
