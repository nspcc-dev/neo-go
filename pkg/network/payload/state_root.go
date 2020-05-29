package payload

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// MaxStateRootsAllowed is a maxumum amount of state roots
// which can be sent in a single payload.
const MaxStateRootsAllowed = 2000

// StateRoots contains multiple StateRoots.
type StateRoots struct {
	Roots []state.MPTRoot
}

// GetStateRoots represents request for state roots.
type GetStateRoots struct {
	Start uint32
	Count uint32
}

// EncodeBinary implements io.Serializable.
func (s *StateRoots) EncodeBinary(w *io.BinWriter) {
	w.WriteArray(s.Roots)
}

// DecodeBinary implements io.Serializable.
func (s *StateRoots) DecodeBinary(r *io.BinReader) {
	r.ReadArray(&s.Roots, MaxStateRootsAllowed)
}

// DecodeBinary implements io.Serializable.
func (g *GetStateRoots) DecodeBinary(r *io.BinReader) {
	g.Start = r.ReadU32LE()
	g.Count = r.ReadU32LE()
}

// EncodeBinary implements io.Serializable.
func (g *GetStateRoots) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(g.Start)
	w.WriteU32LE(g.Count)
}
