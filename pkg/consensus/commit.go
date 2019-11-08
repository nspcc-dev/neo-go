package consensus

import "github.com/CityOfZion/neo-go/pkg/io"

// commit represents dBFT Commit message.
type commit struct {
	Signature [signatureSize]byte
}

// signatureSize is an rfc6989 signature size in bytes
// without leading byte (0x04, uncompressed)
const signatureSize = 64

// EncodeBinary implements io.Serializable interface.
func (c *commit) EncodeBinary(w *io.BinWriter) {
	w.WriteBE(c.Signature)
}

// DecodeBinary implements io.Serializable interface.
func (c *commit) DecodeBinary(r *io.BinReader) {
	r.ReadBE(&c.Signature)
}
