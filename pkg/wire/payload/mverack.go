package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
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
func (v *VerackMessage) Command() command.Type {
	return command.Verack
}
