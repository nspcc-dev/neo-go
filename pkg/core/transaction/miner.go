package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
)

// MinerTX represents a miner transaction.
type MinerTX struct {
	// Random number to avoid hash collision.
	Nonce uint32
}

// DecodeBinary implements the Payload interface.
func (tx *MinerTX) DecodeBinary(r *io.BinReader) error {
	r.ReadLE(&tx.Nonce)
	return r.Err
}

// EncodeBinary implements the Payload interface.
func (tx *MinerTX) EncodeBinary(w *io.BinWriter) error {
	w.WriteLE(tx.Nonce)
	return w.Err
}

// Size returns serialized binary size for this transaction.
func (tx *MinerTX) Size() int {
	return 4 // Nonce
}
