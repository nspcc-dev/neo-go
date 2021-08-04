package payload

import "github.com/nspcc-dev/neo-go/pkg/io"

// MaxSize is maximum payload size in decompressed form.
const MaxSize = 0x02000000

// Payload is anything that can be binary encoded/decoded.
type Payload interface {
	io.Serializable
}

// NullPayload is a dummy payload with no fields.
type NullPayload struct {
}

// NewNullPayload returns zero-sized stub payload.
func NewNullPayload() NullPayload {
	return NullPayload{}
}

// DecodeBinary implements Serializable interface.
func (p NullPayload) DecodeBinary(r io.BinaryReader) {}

// EncodeBinary implements Serializable interface.
func (p NullPayload) EncodeBinary(w io.BinaryWriter) {}
