package mpt

import (
	"testing"

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

	tr := NewTrie(b, ModeAll, newTestStore())
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
