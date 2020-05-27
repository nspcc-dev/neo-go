package transaction

import (
	"errors"
	"math/rand"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// InvocationTX represents a invocation transaction and is used to
// deploy smart contract to the NEO blockchain.
type InvocationTX struct {
	// Script output of the smart contract.
	Script []byte
}

// NewInvocationTX returns a new invocation transaction.
func NewInvocationTX(script []byte, gas util.Fixed8) *Transaction {
	return &Transaction{
		Type:    InvocationType,
		Version: 1,
		Nonce:   rand.Uint32(),
		Data: &InvocationTX{
			Script: script,
		},
		SystemFee:  gas,
		Attributes: []Attribute{},
		Cosigners:  []Cosigner{},
		Inputs:     []Input{},
		Outputs:    []Output{},
		Scripts:    []Witness{},
	}
}

// DecodeBinary implements Serializable interface.
func (tx *InvocationTX) DecodeBinary(br *io.BinReader) {
	tx.Script = br.ReadVarBytes()
	if br.Err == nil && len(tx.Script) == 0 {
		br.Err = errors.New("no script")
		return
	}
}

// EncodeBinary implements Serializable interface.
func (tx *InvocationTX) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarBytes(tx.Script)
}
