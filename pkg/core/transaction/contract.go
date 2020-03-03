package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// ContractTX represents a contract transaction.
// This TX has not special attributes.
type ContractTX struct{}

// NewContractTX creates Transaction of ContractType type.
func NewContractTX() *Transaction {
	return &Transaction{
		Type: ContractType,
	}
}

// DecodeBinary implements Serializable interface.
func (tx *ContractTX) DecodeBinary(r *io.BinReader) {
}

// EncodeBinary implements Serializable interface.
func (tx *ContractTX) EncodeBinary(w *io.BinWriter) {
}
