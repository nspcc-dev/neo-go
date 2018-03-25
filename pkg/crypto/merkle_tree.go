package crypto

import (
	"crypto/sha256"
	"errors"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// MerkleTree implementation.

type MerkleTree struct {
	root  *MerkleTreeNode
	depth int
}

// NewMerkleTree returns new MerkleTree object.
func NewMerkleTree(hashes []util.Uint256) (*MerkleTree, error) {
	if len(hashes) == 0 {
		return nil, errors.New("length of the hashes cannot be zero")
	}

	nodes := make([]*MerkleTreeNode, len(hashes))
	for i := 0; i < len(hashes); i++ {
		nodes[i] = &MerkleTreeNode{
			hash: hashes[i],
		}
	}

	root, err := buildMerkleTree(nodes)
	if err != nil {
		return nil, err
	}

	return &MerkleTree{
		root:  root,
		depth: 1,
	}, nil
}

// Root return the computed root hash of the MerkleTree.
func (t *MerkleTree) Root() util.Uint256 {
	return t.root.hash
}

func buildMerkleTree(leaves []*MerkleTreeNode) (*MerkleTreeNode, error) {
	if len(leaves) == 0 {
		return nil, errors.New("length of the leaves cannot be zero")
	}
	if len(leaves) == 1 {
		return leaves[0], nil
	}

	parents := make([]*MerkleTreeNode, (len(leaves)+1)/2)
	for i := 0; i < len(parents); i++ {
		parents[i] = &MerkleTreeNode{}
		parents[i].leftChild = leaves[i*2]
		leaves[i*2].parent = parents[i]

		if i*2+1 == len(leaves) {
			parents[i].rightChild = parents[i].leftChild
		} else {
			parents[i].rightChild = leaves[i*2+1]
			leaves[i*2+1].parent = parents[i]
		}

		b1 := parents[i].leftChild.hash.Bytes()
		b2 := parents[i].rightChild.hash.Bytes()
		b1 = append(b1, b2...)
		parents[i].hash = hash256(b1)
	}

	return buildMerkleTree(parents)
}

// MerkleTreeNode represents a node in the MerkleTree.
type MerkleTreeNode struct {
	hash       util.Uint256
	parent     *MerkleTreeNode
	leftChild  *MerkleTreeNode
	rightChild *MerkleTreeNode
}

// IsLeaf returns whether this node is a leaf node or not.
func (n *MerkleTreeNode) IsLeaf() bool {
	return n.leftChild == nil && n.rightChild == nil
}

// IsRoot returns whether this node is a root node or not.
func (n *MerkleTreeNode) IsRoot() bool {
	return n.parent == nil
}

func hash256(b []byte) util.Uint256 {
	var hash util.Uint256
	hash = sha256.Sum256(b)
	hash = sha256.Sum256(hash.Bytes())
	return hash
}
