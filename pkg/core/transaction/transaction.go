package transaction

import (
	"encoding/binary"
	"io"
)

// Transaction is a process recorded in the NEO blockchain.
type Transaction struct {
	// The type of the transaction.
	Type TransactionType

	// The trading version which is currently 0.
	Version uint8

	// Transaction specific data.
	Data []byte

	// Transaction attributes.
	Attributes []Attribute

	// The inputs of the transaction.
	Inputs []Input

	// The outputs of the transaction.
	Outputs []Output

	// The scripts that comes with this transaction.
	// Scripts exist out of the verification script
	// and invocation script.
	Scripts Witness
}

// DecodeBinary implements the payload interface.
func (t Transaction) DecodeBinary(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &t.Type)
	return err
}

// EncodeBinary implements the payload interface.
func (t Transaction) EncodeBinary(w io.Writer) error {
	return nil
}
