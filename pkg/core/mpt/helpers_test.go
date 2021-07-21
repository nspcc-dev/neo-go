package mpt

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestToNibblesFromNibbles(t *testing.T) {
	check := func(t *testing.T, expected []byte) {
		actual := fromNibbles(toNibbles(expected))
		require.Equal(t, expected, actual)
	}
	t.Run("empty path", func(t *testing.T) {
		check(t, []byte{})
	})
	t.Run("non-empty path", func(t *testing.T) {
		check(t, []byte{0x01, 0xAC, 0x8d, 0x04, 0xFF})
	})
}

func TestGetChildrenPaths(t *testing.T) {
	h1 := NewHashNode(util.Uint256{1, 2, 3})
	h2 := NewHashNode(util.Uint256{4, 5, 6})
	h3 := NewHashNode(util.Uint256{7, 8, 9})
	l := NewLeafNode([]byte{1, 2, 3})
	ext1 := NewExtensionNode([]byte{8, 9}, h1)
	ext2 := NewExtensionNode([]byte{7, 6}, l)
	branch := NewBranchNode()
	branch.Children[3] = h1
	branch.Children[5] = l
	branch.Children[6] = h1 // 3-th and 6-th children have the same hash
	branch.Children[7] = h3
	branch.Children[lastChild] = h2
	testCases := map[string]struct {
		node     Node
		expected map[util.Uint256][][]byte
	}{
		"Hash":                         {h1, nil},
		"Leaf":                         {l, nil},
		"Extension with next Hash":     {ext1, map[util.Uint256][][]byte{h1.Hash(): {ext1.key}}},
		"Extension with next non-Hash": {ext2, map[util.Uint256][][]byte{}},
		"Branch": {branch, map[util.Uint256][][]byte{
			h1.Hash(): {{0x03}, {0x06}},
			h2.Hash(): {{}},
			h3.Hash(): {{0x07}},
		}},
	}
	parentPath := []byte{4, 5, 6}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, testCase.expected, GetChildrenPaths([]byte{}, testCase.node))
			if testCase.expected != nil {
				expectedWithPrefix := make(map[util.Uint256][][]byte, len(testCase.expected))
				for h, paths := range testCase.expected {
					var res [][]byte
					for _, path := range paths {
						res = append(res, append(parentPath, path...))
					}
					expectedWithPrefix[h] = res
				}
				require.Equal(t, expectedWithPrefix, GetChildrenPaths(parentPath, testCase.node))
			}
		})
	}
}
