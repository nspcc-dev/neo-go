package state

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Deposit represents GAS deposit from Notary contract.
type Deposit struct {
	Amount *big.Int
	Till   uint32
}

// EncodeBinary implements io.Serializable interface.
func (d *Deposit) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(bigint.ToBytes(d.Amount))
	w.WriteU32LE(d.Till)
}

// DecodeBinary implements io.Serializable interface.
func (d *Deposit) DecodeBinary(r *io.BinReader) {
	d.Amount = bigint.FromBytes(r.ReadVarBytes())
	d.Till = r.ReadU32LE()
}
