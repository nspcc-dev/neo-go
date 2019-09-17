package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
)

// MinerTX represents a miner transaction.
type MinerTX struct {
	// Random number to avoid hash collision.
	Nonce uint32
}

// DecodeBinary implements Serializable interface.
func (tx *MinerTX) DecodeBinary(r *io.BinReader) {
	r.ReadLE(&tx.Nonce)
}

// EncodeBinary implements Serializable interface.
func (tx *MinerTX) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(tx.Nonce)
}
