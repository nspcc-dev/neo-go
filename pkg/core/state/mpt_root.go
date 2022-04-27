package state

import (
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MPTRoot represents the storage state root together with sign info.
type MPTRoot struct {
	Version byte                  `json:"version"`
	Index   uint32                `json:"index"`
	Root    util.Uint256          `json:"roothash"`
	Witness []transaction.Witness `json:"witnesses"`
}

// Hash returns the hash of s.
func (s *MPTRoot) Hash() util.Uint256 {
	buf := io.NewBufBinWriter()
	s.EncodeBinaryUnsigned(buf.BinWriter)
	return hash.Sha256(buf.Bytes())
}

// DecodeBinaryUnsigned decodes the hashable part of the state root.
func (s *MPTRoot) DecodeBinaryUnsigned(r *io.BinReader) {
	s.Version = r.ReadB()
	s.Index = r.ReadU32LE()
	s.Root.DecodeBinary(r)
}

// EncodeBinaryUnsigned encodes the hashable part of the state root..
func (s *MPTRoot) EncodeBinaryUnsigned(w *io.BinWriter) {
	w.WriteB(s.Version)
	w.WriteU32LE(s.Index)
	s.Root.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable.
func (s *MPTRoot) DecodeBinary(r *io.BinReader) {
	s.DecodeBinaryUnsigned(r)
	r.ReadArray(&s.Witness, 1)
}

// EncodeBinary implements io.Serializable.
func (s *MPTRoot) EncodeBinary(w *io.BinWriter) {
	s.EncodeBinaryUnsigned(w)
	w.WriteArray(s.Witness)
}
