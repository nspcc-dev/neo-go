package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

//GetAddrMessage represents a GetAddress message on the neo-network
type GetAddrMessage struct{}

// NewGetAddrMessage returns a GetAddrMessage object
func NewGetAddrMessage() (*GetAddrMessage, error) {
	return &GetAddrMessage{}, nil
}

// DecodePayload Implements Messager interface
func (v *GetAddrMessage) DecodePayload(r io.Reader) error {
	return nil
}

// EncodePayload Implements messager interface
func (v *GetAddrMessage) EncodePayload(w io.Writer) error {
	return nil
}

// Command Implements messager interface
func (v *GetAddrMessage) Command() command.Type {
	return command.GetAddr
}
