package payload

import (
	"bytes"
	"io"

	"github.com/CityOfZion/neo-go/pkg/wire/command"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	checksum "github.com/CityOfZion/neo-go/pkg/wire/util/Checksum"
)

type TXMessage struct {
	w  *bytes.Buffer
	Tx transaction.Transactioner
}

func NewTXMessage(tx transaction.Transactioner) (*TXMessage, error) {

	Tx := &TXMessage{new(bytes.Buffer), tx}
	if err := Tx.EncodePayload(Tx.w); err != nil {
		return nil, err
	}
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
func (v *TXMessage) PayloadLength() uint32 {
	return util.CalculatePayloadLength(v.w)
}

// Implements messager interface
func (v *TXMessage) Checksum() uint32 {
	return checksum.FromBuf(v.w)
}

// Implements messager interface
func (v *TXMessage) Command() command.Type {
	return command.TX
}
