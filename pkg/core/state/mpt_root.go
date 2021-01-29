package state

import (
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MPTRoot represents storage state root together with sign info.
type MPTRoot struct {
	Version byte                 `json:"version"`
	Index   uint32               `json:"index"`
	Root    util.Uint256         `json:"stateroot"`
	Witness *transaction.Witness `json:"witness,omitempty"`
}

// GetSignedPart returns part of MPTRootBase which needs to be signed.
func (s *MPTRoot) GetSignedPart() []byte {
	buf := io.NewBufBinWriter()
	s.EncodeBinaryUnsigned(buf.BinWriter)
	return buf.Bytes()
}

// GetSignedHash returns hash of MPTRootBase which needs to be signed.
func (s *MPTRoot) GetSignedHash() util.Uint256 {
	return hash.Sha256(s.GetSignedPart())
}

// Hash returns hash of s.
func (s *MPTRoot) Hash() util.Uint256 {
	return hash.DoubleSha256(s.GetSignedPart())
}

// DecodeBinaryUnsigned decodes hashable part of state root.
func (s *MPTRoot) DecodeBinaryUnsigned(r *io.BinReader) {
	s.Version = r.ReadB()
	s.Index = r.ReadU32LE()
	s.Root.DecodeBinary(r)
}

// EncodeBinaryUnsigned encodes hashable part of state root..
func (s *MPTRoot) EncodeBinaryUnsigned(w *io.BinWriter) {
	w.WriteB(s.Version)
	w.WriteU32LE(s.Index)
	s.Root.EncodeBinary(w)
}

// DecodeBinary implements io.Serializable.
func (s *MPTRoot) DecodeBinary(r *io.BinReader) {
	s.DecodeBinaryUnsigned(r)

	var ws []transaction.Witness
	r.ReadArray(&ws, 1)
	if len(ws) == 1 {
		s.Witness = &ws[0]
	}
}

// EncodeBinary implements io.Serializable.
func (s *MPTRoot) EncodeBinary(w *io.BinWriter) {
	s.EncodeBinaryUnsigned(w)
	if s.Witness == nil {
		w.WriteVarUint(0)
	} else {
		w.WriteArray([]*transaction.Witness{s.Witness})
	}
}
