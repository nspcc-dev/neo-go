package payload

import "io"

// Payload is anything that can be binary encoded/decoded.
type Payload interface {
	EncodeBinary(io.Writer) error
	DecodeBinary(io.Reader) error
}

// NullPayload is a dummy payload with no fields.
type NullPayload struct {
}

// NewNullPayload returns zero-sized stub payload.
func NewNullPayload() *NullPayload {
	return &NullPayload{}
}

// DecodeBinary implements the Payload interface.
func (p *NullPayload) DecodeBinary(r io.Reader) error {
	return nil
}

// EncodeBinary implements the Payload interface.
func (p *NullPayload) EncodeBinary(r io.Writer) error {
	return nil
}
