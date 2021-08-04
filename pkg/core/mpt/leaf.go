package mpt

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MaxValueLength is a max length of a leaf node value.
const MaxValueLength = 3 + storage.MaxStorageValueLen + 1

// LeafNode represents MPT's leaf node.
type LeafNode struct {
	BaseNode
	value []byte
}

var _ Node = (*LeafNode)(nil)

// NewLeafNode returns hash node with the specified value.
func NewLeafNode(value []byte) *LeafNode {
	return &LeafNode{value: value}
}

// Type implements Node interface.
func (n LeafNode) Type() NodeType { return LeafT }

// Hash implements BaseNode interface.
func (n *LeafNode) Hash() util.Uint256 {
	return n.getHash(n)
}

// Bytes implements BaseNode interface.
func (n *LeafNode) Bytes() []byte {
	return n.getBytes(n)
}

// DecodeBinary implements io.Serializable.
func (n *LeafNode) DecodeBinary(r io.BinaryReader) {
	sz := r.ReadVarUint()
	if sz > MaxValueLength {
		r.SetError(fmt.Errorf("leaf node value is too big: %d", sz))
		return
	}
	n.value = make([]byte, sz)
	r.ReadBytes(n.value)
	n.invalidateCache()
}

// EncodeBinary implements io.Serializable.
func (n LeafNode) EncodeBinary(w io.BinaryWriter) {
	w.WriteVarBytes(n.value)
}

// EncodeBinaryAsChild implements BaseNode interface.
func (n *LeafNode) EncodeBinaryAsChild(w io.BinaryWriter) {
	no := &NodeObject{Node: NewHashNode(n.Hash())} // with type
	no.EncodeBinary(w)
}

// MarshalJSON implements json.Marshaler.
func (n *LeafNode) MarshalJSON() ([]byte, error) {
	return []byte(`{"value":"` + hex.EncodeToString(n.value) + `"}`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *LeafNode) UnmarshalJSON(data []byte) error {
	var obj NodeObject
	if err := obj.UnmarshalJSON(data); err != nil {
		return err
	} else if u, ok := obj.Node.(*LeafNode); ok {
		*n = *u
		return nil
	}
	return errors.New("expected leaf node")
}
