package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
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
func (v *GetAddrMessage) Command() command.Type {
	return command.GetAddr
}
