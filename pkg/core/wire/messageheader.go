package wire

import (
	"io"
)

type MessageHeader struct {
	Magic    uint32
	Command  CommandType
	Length   uint32
	Checksum uint32
}

// Note, That there is no EncodeMessageHeader
// As the header is implicitly inferred from
// the message on encode
func (h *MessageHeader) DecodeMessageHeader(r io.Reader) error {
	return nil
}
