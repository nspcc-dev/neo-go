package payload

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
)

// TXMessage represents a transaction message on the neo-network
type TXMessage struct {
	Tx transaction.Transactioner
}

//NewTXMessage returns a new tx object
func NewTXMessage(tx transaction.Transactioner) (*TXMessage, error) {

	Tx := &TXMessage{tx}
	return Tx, nil
}

// DecodePayload Implements Messager interface
func (t *TXMessage) DecodePayload(r io.Reader) error {
	return t.Tx.Decode(r)
}

// EncodePayload Implements messager interface
func (t *TXMessage) EncodePayload(w io.Writer) error {
	return t.Tx.Encode(w)
}

// Command Implements messager interface
func (t *TXMessage) Command() command.Type {
	return command.TX
}
