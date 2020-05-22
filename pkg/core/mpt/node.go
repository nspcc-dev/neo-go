package mpt

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NodeType represents node type..
type NodeType byte

// Node types definitions.
const (
	BranchT    NodeType = 0x00
	ExtensionT NodeType = 0x01
	HashT      NodeType = 0x02
	LeafT      NodeType = 0x03
)

// NodeObject represents Node together with it's type.
// It is used for serialization/deserialization where type info
// is also expected.
type NodeObject struct {
	Node
}

// Node represents common interface of all MPT nodes.
type Node interface {
	io.Serializable
	Hash() util.Uint256
	Type() NodeType
}

// EncodeBinary implements io.Serializable.
func (n NodeObject) EncodeBinary(w *io.BinWriter) {
	encodeNodeWithType(n.Node, w)
}

// DecodeBinary implements io.Serializable.
func (n *NodeObject) DecodeBinary(r *io.BinReader) {
	typ := NodeType(r.ReadB())
	switch typ {
	case BranchT:
		n.Node = new(BranchNode)
	case ExtensionT:
		n.Node = new(ExtensionNode)
	case HashT:
		n.Node = new(HashNode)
	case LeafT:
		n.Node = new(LeafNode)
	default:
		r.Err = fmt.Errorf("invalid node type: %x", typ)
		return
	}
	n.Node.DecodeBinary(r)
}

// encodeNodeWithType encodes node together with it's type.
func encodeNodeWithType(n Node, w *io.BinWriter) {
	w.WriteB(byte(n.Type()))
	n.EncodeBinary(w)
}

// toBytes is a helper for serializing node.
func toBytes(n Node) []byte {
	buf := io.NewBufBinWriter()
	encodeNodeWithType(n, buf.BinWriter)
	return buf.Bytes()
}
