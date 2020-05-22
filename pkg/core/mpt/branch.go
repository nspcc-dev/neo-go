package mpt

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// childrenCount represents a number of children of a branch node.
	childrenCount = 17
	// lastChild is the index of the last child.
	lastChild = childrenCount - 1
)

// BranchNode represents MPT's branch node.
type BranchNode struct {
	hash  util.Uint256
	valid bool

	Children [childrenCount]Node
}

var _ Node = (*BranchNode)(nil)

// NewBranchNode returns new branch node.
func NewBranchNode() *BranchNode {
	b := new(BranchNode)
	for i := 0; i < childrenCount; i++ {
		b.Children[i] = new(HashNode)
	}
	return b
}

// Type implements Node interface.
func (b *BranchNode) Type() NodeType { return BranchT }

// Hash implements Node interface.
func (b *BranchNode) Hash() util.Uint256 {
	if !b.valid {
		b.hash = hash.DoubleSha256(toBytes(b))
		b.valid = true
	}
	return b.hash
}

// invalidateHash invalidates node hash.
func (b *BranchNode) invalidateHash() {
	b.valid = false
}

// EncodeBinary implements io.Serializable.
func (b *BranchNode) EncodeBinary(w *io.BinWriter) {
	for i := 0; i < childrenCount; i++ {
		if hn, ok := b.Children[i].(*HashNode); ok {
			hn.EncodeBinary(w)
			continue
		}
		n := NewHashNode(b.Children[i].Hash())
		n.EncodeBinary(w)
	}
}

// DecodeBinary implements io.Serializable.
func (b *BranchNode) DecodeBinary(r *io.BinReader) {
	for i := 0; i < childrenCount; i++ {
		b.Children[i] = new(HashNode)
		b.Children[i].DecodeBinary(r)
	}
}

// splitPath splits path for a branch node.
func splitPath(path []byte) (byte, []byte) {
	if len(path) != 0 {
		return path[0], path[1:]
	}
	return lastChild, path
}
