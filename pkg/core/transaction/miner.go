package transaction

import (
	"encoding/binary"
	"io"
)

// MinerTX represents a miner transaction.
type MinerTX struct {
	// Random number to avoid hash collision.
	Nonce uint32
}

// DecodeBinary implements the Payload interface.
func (tx *MinerTX) DecodeBinary(r io.Reader) error {
	return binary.Read(r, binary.LittleEndian, &tx.Nonce)
}

// EncodeBinary implements the Payload interface.
func (tx *MinerTX) EncodeBinary(w io.Writer) error {
	return binary.Write(w, binary.LittleEndian, tx.Nonce)
}
