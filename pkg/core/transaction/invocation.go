package transaction

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// InvocationTX represents a invocation transaction and is used to
// deploy smart contract to the NEO blockchain.
type InvocationTX struct {
	// Script output of the smart contract.
	Script []byte

	// Gas cost of the smart contract.
	Gas util.Fixed8
}

// NewInvocationTX returns a new invocation transaction.
func NewInvocationTX(script []byte) *Transaction {
	return &Transaction{
		Type:    InvocationType,
		Version: 1,
		Data: &InvocationTX{
			Script: script,
		},
		Attributes: []*Attribute{},
		Inputs:     []*Input{},
		Outputs:    []*Output{},
		Scripts:    []*Witness{},
	}
}

// DecodeBinary implements the Payload interface.
func (tx *InvocationTX) DecodeBinary(r io.Reader) error {
	lenScript := util.ReadVarUint(r)
	tx.Script = make([]byte, lenScript)
	if err := binary.Read(r, binary.LittleEndian, tx.Script); err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &tx.Gas)
}

// EncodeBinary implements the Payload interface.
func (tx *InvocationTX) EncodeBinary(w io.Writer) error {
	if err := util.WriteVarUint(w, uint64(len(tx.Script))); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, tx.Script); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, tx.Gas)
}
