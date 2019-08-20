package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

//VerackMessage represents a verack message on the neo-network
type VerackMessage struct{}

//NewVerackMessage returns a verack message
func NewVerackMessage() (*VerackMessage, error) {
	return &VerackMessage{}, nil
}

// DecodePayload Implements Messager interface
func (v *VerackMessage) DecodePayload(r io.Reader) error {
	return nil
}

// EncodePayload Implements messager interface
func (v *VerackMessage) EncodePayload(w io.Writer) error {
	return nil
}

// Command Implements messager interface
func (v *VerackMessage) Command() command.Type {
	return command.Verack
}
