package block

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Header holds the head info of a block.
type Header struct {
	// Base of the block.
	Base
}

// DecodeBinary implements Serializable interface.
func (h *Header) DecodeBinary(r *io.BinReader) {
	h.Base.DecodeBinary(r)

	padding := []byte{0}
	r.ReadBytes(padding)

	if padding[0] != 0 {
		r.Err = fmt.Errorf("format error: padding must equal 0 got %d", padding)
	}
}

// EncodeBinary implements Serializable interface.
func (h *Header) EncodeBinary(w *io.BinWriter) {
	h.Base.EncodeBinary(w)
	w.WriteBytes([]byte{0})
}
