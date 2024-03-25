package consensus

import (
	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// commit represents dBFT Commit message.
type commit struct {
	signature [signatureSize]byte
}

// signatureSize is an rfc6989 signature size in bytes
// without a leading byte (0x04, uncompressed).
const signatureSize = 64

var _ dbft.Commit = (*commit)(nil)

// EncodeBinary implements the io.Serializable interface.
func (c *commit) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(c.signature[:])
}

// DecodeBinary implements the io.Serializable interface.
func (c *commit) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(c.signature[:])
}

// Signature implements the payload.Commit interface.
func (c commit) Signature() []byte { return c.signature[:] }
