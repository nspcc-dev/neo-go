package transaction

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// InvocationTX represents a invocation transaction and is used to
// deploy smart contract to the NEO blockchain.
type InvocationTX struct {
	// Script output of the smart contract.
	Script []byte

	// Gas cost of the smart contract.
	Gas     util.Fixed8
	Version uint8
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
	br := util.BinReader{R: r}
	tx.Script = br.ReadBytes()
	if tx.Version >= 1 {
		br.ReadLE(&tx.Gas)
	} else {
		tx.Gas = util.Fixed8FromInt64(0)
	}
	return br.Err
}

// EncodeBinary implements the Payload interface.
func (tx *InvocationTX) EncodeBinary(w io.Writer) error {
	bw := util.BinWriter{W: w}
	bw.WriteBytes(tx.Script)
	if tx.Version >= 1 {
		bw.WriteLE(tx.Gas)
	}
	return bw.Err
}

// Size returns serialized binary size for this transaction.
func (tx *InvocationTX) Size() int {
	sz := util.GetVarSize(tx.Script)
	if tx.Version >= 1 {
		sz += tx.Gas.Size()
	}
	return sz
}
