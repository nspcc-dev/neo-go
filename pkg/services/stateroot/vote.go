package stateroot

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Vote represents vote message.
type Vote struct {
	ValidatorIndex int32
	Height         uint32
	Signature      []byte
}

// EncodeBinary implements io.Serializable interface.
func (p *Vote) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(uint32(p.ValidatorIndex))
	w.WriteU32LE(p.Height)
	w.WriteVarBytes(p.Signature)
}

// DecodeBinary implements io.Serializable interface.
func (p *Vote) DecodeBinary(r *io.BinReader) {
	p.ValidatorIndex = int32(r.ReadU32LE())
	p.Height = r.ReadU32LE()
	p.Signature = r.ReadVarBytes(keys.SignatureLen)
}
