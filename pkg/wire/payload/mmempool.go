package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
)

// No payload
type GetMempool struct{}

func NewGetMempool() (*GetMempool, error) {
	return &GetMempool{}, nil
}

// Implements Messager interface
func (v *GetMempool) DecodePayload(r io.Reader) error {
	return nil
}

// Implements messager interface
func (v *GetMempool) EncodePayload(w io.Writer) error {
	return nil
}

// Implements messager interface
func (v *GetMempool) Command() command.Type {
	return command.Mempool
}
