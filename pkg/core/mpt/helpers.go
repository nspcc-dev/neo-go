package mpt

import "github.com/nspcc-dev/neo-go/pkg/util"

// lcp returns longest common prefix of a and b.
// Note: it does no allocations.
func lcp(a, b []byte) []byte {
	if len(a) < len(b) {
		return lcp(b, a)
	}

	var i int
	for i = 0; i < len(b); i++ {
		if a[i] != b[i] {
			break
		}
	}

	return a[:i]
}

func lcpMany(kv []keyValue) []byte {
	if len(kv) == 1 {
		return kv[0].key
	}
	p := lcp(kv[0].key, kv[1].key)
	if len(p) == 0 {
		return p
	}
	for i := range kv[2:] {
		p = lcp(p, kv[2+i].key)
	}
	return p
}

// toNibbles mangles path by splitting every byte into 2 containing low- and high- 4-byte part.
func toNibbles(path []byte) []byte {
	result := make([]byte, len(path)*2)
	for i := range path {
		result[i*2] = path[i] >> 4
		result[i*2+1] = path[i] & 0x0F
	}
	return result
}

// fromNibbles performs operation opposite to toNibbles and does no path validity checks.
func fromNibbles(path []byte) []byte {
	result := make([]byte, len(path)/2)
	for i := range result {
		result[i] = path[2*i]<<4 + path[2*i+1]
	}
	return result
}

// GetChildrenPaths returns a set of paths to node's children who are non-empty HashNodes
// based on the node's path.
func GetChildrenPaths(path []byte, node Node) map[util.Uint256][][]byte {
	res := make(map[util.Uint256][][]byte)
	switch n := node.(type) {
	case *LeafNode, *HashNode:
		return nil
	case *BranchNode:
		for i, child := range n.Children {
			if child.Type() == HashT && !child.(*HashNode).IsEmpty() {
				cPath := make([]byte, len(path), len(path)+1)
				copy(cPath, path)
				if i != lastChild {
					cPath = append(cPath, byte(i))
				}
				paths := res[child.Hash()]
				paths = append(paths, cPath)
				res[child.Hash()] = paths
			}
		}
	case *ExtensionNode:
		if n.next.Type() == HashT && !n.next.(*HashNode).IsEmpty() {
			cPath := make([]byte, len(path)+len(n.key))
			copy(cPath, path)
			copy(cPath[len(path):], n.key)
			res[n.next.Hash()] = [][]byte{cPath}
		}
	default:
		panic("unknown Node type")
	}
	return res
}
