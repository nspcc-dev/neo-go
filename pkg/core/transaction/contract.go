package transaction

import (
	"io"
)

// ContractTX represents a contract transaction.
// This TX has not special attributes.
type ContractTX struct{}

func NewContractTX() *Transaction {
	return &Transaction{
		Type: ContractType,
	}
}

// DecodeBinary implements the Payload interface.
func (tx *ContractTX) DecodeBinary(r io.Reader) error {
	return nil
}

// EncodeBinary implements the Payload interface.
func (tx *ContractTX) EncodeBinary(w io.Writer) error {
	return nil
}

func (tx *ContractTX) Size() int {
	return 0
}
