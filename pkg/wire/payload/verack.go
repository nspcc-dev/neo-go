package payload

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
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
	return util.CalculateCheckSum(new(bytes.Buffer))
}

// Implements messager interface
func (v *VerackMessage) Command() command.Type {
	return command.Verack
}
