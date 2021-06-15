package mpt

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func newProofTrie(t *testing.T, missingHashNode bool) *Trie {
	l := NewLeafNode([]byte("somevalue"))
	e := NewExtensionNode([]byte{0x05, 0x06, 0x07}, l)
	l2 := NewLeafNode([]byte("invalid"))
	e2 := NewExtensionNode([]byte{0x05}, NewHashNode(l2.Hash()))
	b := NewBranchNode()
	b.Children[4] = NewHashNode(e.Hash())
	b.Children[5] = e2

	tr := NewTrie(b, false, newTestStore())
	require.NoError(t, tr.Put([]byte{0x12, 0x31}, []byte("value1")))
	require.NoError(t, tr.Put([]byte{0x12, 0x32}, []byte("value2")))
	tr.putToStore(l)
	tr.putToStore(e)
	if !missingHashNode {
		tr.putToStore(l2)
	}
	return tr
}

func TestTrie_GetProof(t *testing.T) {
	tr := newProofTrie(t, true)

	t.Run("MissingKey", func(t *testing.T) {
		_, err := tr.GetProof([]byte{0x12})
		require.Error(t, err)
	})

	t.Run("Valid", func(t *testing.T) {
		_, err := tr.GetProof([]byte{0x12, 0x31})
		require.NoError(t, err)
	})

	t.Run("MissingHashNode", func(t *testing.T) {
		_, err := tr.GetProof([]byte{0x55})
		require.Error(t, err)
	})
}

func TestVerifyProof(t *testing.T) {
	tr := newProofTrie(t, true)

	t.Run("Simple", func(t *testing.T) {
		proof, err := tr.GetProof([]byte{0x12, 0x32})
		require.NoError(t, err)

		t.Run("Good", func(t *testing.T) {
			v, ok := VerifyProof(tr.root.Hash(), []byte{0x12, 0x32}, proof)
			require.True(t, ok)
			require.Equal(t, []byte("value2"), v)
		})

		t.Run("Bad", func(t *testing.T) {
			_, ok := VerifyProof(tr.root.Hash(), []byte{0x12, 0x31}, proof)
			require.False(t, ok)
		})
	})

	t.Run("InsideHash", func(t *testing.T) {
		key := []byte{0x45, 0x67}
		proof, err := tr.GetProof(key)
		require.NoError(t, err)

		v, ok := VerifyProof(tr.root.Hash(), key, proof)
		require.True(t, ok)
		require.Equal(t, []byte("somevalue"), v)
	})
}

func TestTraverse(t *testing.T) {
	tr := newProofTrie(t, false)
	expectedRoot := tr.StateRoot()

	t.Run("Good, restrict size", func(t *testing.T) {
		const maxSize = 300
		var (
			nodes    [][]byte
			maxBytes = maxSize
		)
		stop := func(node []byte) bool {
			if len(node)+io.GetVarSize(len(node)) > maxBytes {
				return true
			}
			nodes = append(nodes, node)
			maxBytes -= len(node) + io.GetVarSize(len(node))
			return false
		}
		err := tr.Traverse(stop)
		require.NoError(t, err)

		var size int
		for _, n := range nodes {
			size += len(n) + io.GetVarSize(len(n))
		}
		require.Equal(t, maxSize, size+maxBytes)
		require.Equal(t, expectedRoot, tr.StateRoot())
	})

	t.Run("Good, no restrictions", func(t *testing.T) {
		stop := func(node []byte) bool { return false }
		err := tr.Traverse(stop)
		require.NoError(t, err)
		require.Equal(t, expectedRoot, tr.StateRoot())
	})
}

func TestTraverseAndRestore(t *testing.T) {
	expected := newProofTrie(t, false)
	var nodes [][]byte
	stop := func(node []byte) bool {
		nodes = append(nodes, node)
		return false
	}
	err := expected.Traverse(stop)
	require.NoError(t, err)

	// Start from known state root with an empty path
	actual := NewTrie(NewHashNode(expected.StateRoot()), false, storage.NewMemCachedStore(storage.NewMemoryStore()))
	toBeRestored := map[util.Uint256][]byte{expected.StateRoot(): {}}
	for _, nBytes := range nodes {
		var n NodeObject
		r := io.NewBinReaderFromBuf(nBytes)
		n.DecodeBinary(r)
		require.NoError(t, r.Err)

		path, ok := toBeRestored[n.Hash()]
		require.True(t, ok)
		require.NoError(t, actual.RestoreHashNode(path, n.Node))
		delete(toBeRestored, n.Hash())

		childrenPaths := GetChildrenPaths(path, n.Node)
		for h, paths := range childrenPaths {
			// it's not the purpose of the test to check the restoring of nodes with the same hash
			require.Equal(t, 1, len(paths))
			toBeRestored[h] = paths[0]
		}
	}
	require.Empty(t, toBeRestored)
	require.Equal(t, expected.root, actual.root)
}
