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

	// Gas cost of the smart contract.
	Gas     util.Fixed8
	Version uint8
}

// NewInvocationTX returns a new invocation transaction.
func NewInvocationTX(script []byte, gas util.Fixed8) *Transaction {
	return &Transaction{
		Type:    InvocationType,
		Version: 1,
		Nonce:   rand.Uint32(),
		Data: &InvocationTX{
			Script:  script,
			Gas:     gas,
			Version: 1,
		},
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
	if tx.Version >= 1 {
		tx.Gas.DecodeBinary(br)
		if br.Err == nil && tx.Gas.LessThan(0) {
			br.Err = errors.New("negative gas")
			return
		}
	} else {
		tx.Gas = util.Fixed8FromInt64(0)
	}
}

// EncodeBinary implements Serializable interface.
func (tx *InvocationTX) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarBytes(tx.Script)
	if tx.Version >= 1 {
		tx.Gas.EncodeBinary(bw)
	}
}
