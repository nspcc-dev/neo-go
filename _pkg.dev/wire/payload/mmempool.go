package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

// GetMempool represents a GetMempool message on the neo-network
type GetMempool struct{}

//NewGetMempool returns a GetMempool message
func NewGetMempool() (*GetMempool, error) {
	return &GetMempool{}, nil
}

// DecodePayload Implements Messager interface
func (v *GetMempool) DecodePayload(r io.Reader) error {
	return nil
}

// EncodePayload Implements messager interface
func (v *GetMempool) EncodePayload(w io.Writer) error {
	return nil
}

// Command Implements messager interface
func (v *GetMempool) Command() command.Type {
	return command.Mempool
}
