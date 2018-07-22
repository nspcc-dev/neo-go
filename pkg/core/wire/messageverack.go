package wire

import (
	"bytes"
	"io"
)

// No payload
type VerackMessage struct{}

func NewVerackMessage() (*VerackMessage, error) {
	return &VerackMessage{}, nil
}

// Implements Messager interface
func (v *VerackMessage) DecodePayload(r io.Reader) error {
	return nil
}

// Implements messager interface
func (v *VerackMessage) EncodePayload(w io.Writer) error {
	return nil
}

// Implements messager interface
func (v *VerackMessage) PayloadLength() uint32 {
	return 0
}

// Implements messager interface
func (v *VerackMessage) Checksum() uint32 {
	return calculateCheckSum(new(bytes.Buffer))
}

// Implements messager interface
func (v *VerackMessage) Command() CommandType {
	return CMDVerack
}
