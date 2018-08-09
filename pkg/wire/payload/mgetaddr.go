package payload

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	checksum "github.com/CityOfZion/neo-go/pkg/wire/util/Checksum"
)

// No payload
type GetAddrMessage struct{}

func NewGetAddrMessage() (*GetAddrMessage, error) {
	return &GetAddrMessage{}, nil
}

// Implements Messager interface
func (v *GetAddrMessage) DecodePayload(r io.Reader) error {
	return nil
}

// Implements messager interface
func (v *GetAddrMessage) EncodePayload(w io.Writer) error {
	return nil
}

// Implements messager interface
func (v *GetAddrMessage) PayloadLength() uint32 {
	return 0
}

// Implements messager interface
func (v *GetAddrMessage) Checksum() uint32 {
	return checksum.FromBuf(new(bytes.Buffer))
}

// Implements messager interface
func (v *GetAddrMessage) Command() command.Type {
	return command.GetAddr
}
