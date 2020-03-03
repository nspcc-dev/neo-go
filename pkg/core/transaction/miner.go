package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// MinerTX represents a miner transaction.
type MinerTX struct {
	// Random number to avoid hash collision.
	Nonce uint32
}

// DecodeBinary implements Serializable interface.
func (tx *MinerTX) DecodeBinary(r *io.BinReader) {
	tx.Nonce = r.ReadU32LE()
}

// EncodeBinary implements Serializable interface.
func (tx *MinerTX) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(tx.Nonce)
}
