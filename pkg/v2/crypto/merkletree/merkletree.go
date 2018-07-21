package merkle

import (
	"crypto/sha256"
	"errors"

	"github.com/CityOfZion/neo-go/pkg/v2/util"
)

// MerkleTree implementation.

type Tree struct {
	root  *TreeNode
	depth int
}

// NewTree returns new MerkleTree object.
func NewTree(hashes []util.Uint256) (*Tree, error) {
	if len(hashes) == 0 {
		return nil, errors.New("length of the hashes cannot be zero")
	}

	nodes := make([]*TreeNode, len(hashes))
	for i := 0; i < len(hashes); i++ {
		nodes[i] = &TreeNode{
			hash: hashes[i],
		}
	}

	root, err := buildTree(nodes)
	if err != nil {
		return nil, err
	}

	return &Tree{
		root:  root,
		depth: 1,
	}, nil
}

// Root return the computed root hash of the MerkleTree.
func (t *Tree) Root() util.Uint256 {
	return t.root.hash
}

func buildTree(leaves []*TreeNode) (*TreeNode, error) {
	if len(leaves) == 0 {
		return nil, errors.New("length of the leaves cannot be zero")
	}
	if len(leaves) == 1 {
		return leaves[0], nil
	}

	parents := make([]*TreeNode, (len(leaves)+1)/2)
	for i := 0; i < len(parents); i++ {
		parents[i] = &TreeNode{}
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

		// var err error
		parents[i].hash = hash256(b1)
		// if err != nil {
		// 	return nil, err
		// }

	}

	return buildTree(parents)
}

// TreeNode represents a node in the MerkleTree.
type TreeNode struct {
	hash       util.Uint256
	parent     *TreeNode
	leftChild  *TreeNode
	rightChild *TreeNode
}

// IsLeaf returns whether this node is a leaf node or not.
func (n *TreeNode) IsLeaf() bool {
	return n.leftChild == nil && n.rightChild == nil
}

// IsRoot returns whether this node is a root node or not.
func (n *TreeNode) IsRoot() bool {
	return n.parent == nil
}

func hash256(b []byte) util.Uint256 {
	var hash util.Uint256
	hash = sha256.Sum256(b)
	hash = sha256.Sum256(hash.Bytes())
	return hash
}
