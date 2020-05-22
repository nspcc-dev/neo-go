package mpt

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MaxValueLength is a max length of a leaf node value.
const MaxValueLength = 1024 * 1024

// LeafNode represents MPT's leaf node.
type LeafNode struct {
	hash  util.Uint256
	valid bool

	value []byte
}

var _ Node = (*LeafNode)(nil)

// NewLeafNode returns hash node with the specified value.
func NewLeafNode(value []byte) *LeafNode {
	return &LeafNode{value: value}
}

// Type implements Node interface.
func (n LeafNode) Type() NodeType { return LeafT }

// Hash implements Node interface.
func (n *LeafNode) Hash() util.Uint256 {
	if !n.valid {
		n.hash = hash.DoubleSha256(toBytes(n))
		n.valid = true
	}
	return n.hash
}

// DecodeBinary implements io.Serializable.
func (n *LeafNode) DecodeBinary(r *io.BinReader) {
	sz := r.ReadVarUint()
	if sz > MaxValueLength {
		r.Err = fmt.Errorf("leaf node value is too big: %d", sz)
		return
	}
	n.valid = false
	n.value = make([]byte, sz)
	r.ReadBytes(n.value)
}

// EncodeBinary implements io.Serializable.
func (n LeafNode) EncodeBinary(w *io.BinWriter) {
	w.WriteVarBytes(n.value)
}
