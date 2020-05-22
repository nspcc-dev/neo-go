package mpt

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MaxKeyLength is the max length of the extension node key.
const MaxKeyLength = 1125

// ExtensionNode represents MPT's extension node.
type ExtensionNode struct {
	hash  util.Uint256
	valid bool

	key  []byte
	next Node
}

var _ Node = (*ExtensionNode)(nil)

// NewExtensionNode returns hash node with the specified key and next node.
// Note: because it is a part of Trie, key must be mangled, i.e. must contain only bytes with high half = 0.
func NewExtensionNode(key []byte, next Node) *ExtensionNode {
	return &ExtensionNode{
		key:  key,
		next: next,
	}
}

// Type implements Node interface.
func (e ExtensionNode) Type() NodeType { return ExtensionT }

// Hash implements Node interface.
func (e *ExtensionNode) Hash() util.Uint256 {
	if !e.valid {
		e.hash = hash.DoubleSha256(toBytes(e))
		e.valid = true
	}
	return e.hash
}

// invalidateHash invalidates node hash.
func (e *ExtensionNode) invalidateHash() {
	e.valid = false
}

// DecodeBinary implements io.Serializable.
func (e *ExtensionNode) DecodeBinary(r *io.BinReader) {
	sz := r.ReadVarUint()
	if sz > MaxKeyLength {
		r.Err = fmt.Errorf("extension node key is too big: %d", sz)
		return
	}
	e.valid = false
	e.key = make([]byte, sz)
	r.ReadBytes(e.key)
	e.next = new(HashNode)
	e.next.DecodeBinary(r)
}

// EncodeBinary implements io.Serializable.
func (e ExtensionNode) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(e.key)
	n := NewHashNode(e.next.Hash())
	n.EncodeBinary(w)
}
