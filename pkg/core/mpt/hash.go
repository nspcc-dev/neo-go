package mpt

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// HashNode represents MPT's hash node.
type HashNode struct {
	BaseNode
	Collapsed bool
}

var _ Node = (*HashNode)(nil)

// NewHashNode returns hash node with the specified hash.
func NewHashNode(h util.Uint256) *HashNode {
	return &HashNode{
		BaseNode: BaseNode{
			hash:      h,
			hashValid: true,
		},
	}
}

// Type implements Node interface.
func (h *HashNode) Type() NodeType { return HashT }

// Size implements Node interface.
func (h *HashNode) Size() int {
	return util.Uint256Size
}

// Hash implements Node interface.
func (h *HashNode) Hash() util.Uint256 {
	if !h.hashValid {
		panic("can't get hash of an empty HashNode")
	}
	return h.hash
}

// Bytes returns serialized HashNode.
func (h *HashNode) Bytes() []byte {
	return h.getBytes(h)
}

// DecodeBinary implements io.Serializable.
func (h *HashNode) DecodeBinary(r *io.BinReader) {
	if h.hashValid {
		h.hash.DecodeBinary(r)
	}
}

// EncodeBinary implements io.Serializable.
func (h HashNode) EncodeBinary(w *io.BinWriter) {
	if !h.hashValid {
		return
	}
	w.WriteBytes(h.hash[:])
}

// MarshalJSON implements json.Marshaler.
func (h *HashNode) MarshalJSON() ([]byte, error) {
	return []byte(`{"hash":"` + h.hash.StringLE() + `"}`), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (h *HashNode) UnmarshalJSON(data []byte) error {
	var obj NodeObject
	if err := obj.UnmarshalJSON(data); err != nil {
		return err
	} else if u, ok := obj.Node.(*HashNode); ok {
		*h = *u
		return nil
	}
	return errors.New("expected hash node")
}

// Clone implements Node interface.
func (h *HashNode) Clone() Node {
	res := *h
	res.Collapsed = false
	return &res
}
