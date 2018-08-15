package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
)

type TXMessage struct {
	// w  *bytes.Buffer
	Tx transaction.Transactioner
}

func NewTXMessage(tx transaction.Transactioner) (*TXMessage, error) {

	Tx := &TXMessage{tx}
	return Tx, nil
}

// Implements Messager interface
func (t *TXMessage) DecodePayload(r io.Reader) error {
	return t.Tx.Decode(r)
}

// Implements messager interface
func (t *TXMessage) EncodePayload(w io.Writer) error {
	return t.Tx.Encode(w)
}

// Implements messager interface
func (v *TXMessage) Command() command.Type {
	return command.TX
}
