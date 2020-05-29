package payload

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// GetStateRoot represents request for state roots.
type GetStateRoot struct {
	Index uint32
}

// DecodeBinary implements io.Serializable.
func (g *GetStateRoot) DecodeBinary(r *io.BinReader) {
	g.Index = r.ReadU32LE()
}

// EncodeBinary implements io.Serializable.
func (g *GetStateRoot) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(g.Index)
}
