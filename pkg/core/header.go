package core

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Header holds the head info of a block.
type Header struct {
	// Base of the block.
	BlockBase
	// Padding that is fixed to 0
	_ uint8
}

// DecodeBinary impelements the Payload interface.
func (h *Header) DecodeBinary(r io.Reader) error {
	if err := h.BlockBase.DecodeBinary(r); err != nil {
		return err
	}

	var padding uint8
	binary.Read(r, binary.LittleEndian, &padding)
	if padding != 0 {
		return fmt.Errorf("format error: padding must equal 0 got %d", padding)
	}

	return nil
}

// EncodeBinary  impelements the Payload interface.
func (h *Header) EncodeBinary(w io.Writer) error {
	if err := h.BlockBase.EncodeBinary(w); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, uint8(0))
}
