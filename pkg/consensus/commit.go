package consensus

import (
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// commit represents dBFT Commit message.
type commit struct {
	signature [signatureSize]byte
	stateSig  [signatureSize]byte

	stateRootEnabled bool
}

// signatureSize is an rfc6989 signature size in bytes
// without leading byte (0x04, uncompressed)
const signatureSize = 64

var _ payload.Commit = (*commit)(nil)

// EncodeBinary implements io.Serializable interface.
func (c *commit) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(c.signature[:])
	if c.stateRootEnabled {
		w.WriteBytes(c.stateSig[:])
	}
}

// DecodeBinary implements io.Serializable interface.
func (c *commit) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(c.signature[:])
	if c.stateRootEnabled {
		r.ReadBytes(c.stateSig[:])
	}
}

// Signature implements payload.Commit interface.
func (c commit) Signature() []byte { return c.signature[:] }

// SetSignature implements payload.Commit interface.
func (c *commit) SetSignature(signature []byte) {
	copy(c.signature[:], signature)
}
