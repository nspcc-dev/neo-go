package mpt

import (
	"encoding/hex"
	"encoding/json"
	"errors"

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
	json.Marshaler
	json.Unmarshaler
	BaseNodeIface
}

// EncodeBinary implements io.Serializable.
func (n NodeObject) EncodeBinary(w *io.BinWriter) {
	encodeNodeWithType(n.Node, w)
}

// DecodeBinary implements io.Serializable.
func (n *NodeObject) DecodeBinary(r *io.BinReader) {
	n.Node = DecodeNodeWithType(r)
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *NodeObject) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	err := json.Unmarshal(data, &m)
	if err != nil { // it can be a branch node
		var nodes []NodeObject
		if err := json.Unmarshal(data, &nodes); err != nil {
			return err
		} else if len(nodes) != childrenCount {
			return errors.New("invalid length of branch node")
		}

		b := NewBranchNode()
		for i := range b.Children {
			b.Children[i] = nodes[i].Node
		}
		n.Node = b
		return nil
	}

	switch len(m) {
	case 0:
		n.Node = new(HashNode)
	case 1:
		if v, ok := m["hash"]; ok {
			var h util.Uint256
			if err := json.Unmarshal(v, &h); err != nil {
				return err
			}
			n.Node = NewHashNode(h)
		} else if v, ok = m["value"]; ok {
			b, err := unmarshalHex(v)
			if err != nil {
				return err
			} else if len(b) > MaxValueLength {
				return errors.New("leaf value is too big")
			}
			n.Node = NewLeafNode(b)
		} else {
			return errors.New("invalid field")
		}
	case 2:
		keyRaw, ok1 := m["key"]
		nextRaw, ok2 := m["next"]
		if !ok1 || !ok2 {
			return errors.New("invalid field")
		}
		key, err := unmarshalHex(keyRaw)
		if err != nil {
			return err
		} else if len(key) > MaxKeyLength {
			return errors.New("extension key is too big")
		}

		var next NodeObject
		if err := json.Unmarshal(nextRaw, &next); err != nil {
			return err
		}
		n.Node = NewExtensionNode(key, next.Node)
	default:
		return errors.New("0, 1 or 2 fields expected")
	}
	return nil
}

func unmarshalHex(data json.RawMessage) ([]byte, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return hex.DecodeString(s)
}
