package mpt

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestBillet_RestoreHashNode(t *testing.T) {
	check := func(t *testing.T, tr *Billet, expectedRoot Node, expectedNode Node, expectedRefCount uint32) {
		_ = expectedRoot.Hash()
		_ = tr.root.Hash()
		require.Equal(t, expectedRoot, tr.root)
		expectedBytes, err := tr.Store.Get(makeStorageKey(expectedNode.Hash().BytesBE()))
		if expectedRefCount != 0 {
			require.NoError(t, err)
			require.Equal(t, expectedRefCount, binary.LittleEndian.Uint32(expectedBytes[len(expectedBytes)-4:]))
		} else {
			require.True(t, errors.Is(err, storage.ErrKeyNotFound))
		}
	}

	t.Run("parent is Extension", func(t *testing.T) {
		t.Run("restore Branch", func(t *testing.T) {
			b := NewBranchNode()
			b.Children[0] = NewExtensionNode([]byte{0x01}, NewLeafNode([]byte{0xAB, 0xCD}))
			b.Children[5] = NewExtensionNode([]byte{0x01}, NewLeafNode([]byte{0xAB, 0xDE}))
			path := toNibbles([]byte{0xAC})
			e := NewExtensionNode(path, NewHashNode(b.Hash()))
			tr := NewBillet(e.Hash(), true, storage.STTempStorage, newTestStore())
			tr.root = e

			// OK
			n := new(NodeObject)
			n.DecodeBinary(io.NewBinReaderFromBuf(b.Bytes()))
			require.NoError(t, tr.RestoreHashNode(path, n.Node))
			expected := NewExtensionNode(path, n.Node)
			check(t, tr, expected, n.Node, 1)

			// One more time (already restored) => panic expected, no refcount changes
			require.Panics(t, func() {
				_ = tr.RestoreHashNode(path, n.Node)
			})
			check(t, tr, expected, n.Node, 1)

			// Same path, but wrong hash => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode(path, NewBranchNode()), ErrRestoreFailed))
			check(t, tr, expected, n.Node, 1)

			// New path (changes in the MPT structure are not allowed) => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode(toNibbles([]byte{0xAB}), n.Node), ErrRestoreFailed))
			check(t, tr, expected, n.Node, 1)
		})

		t.Run("restore Leaf", func(t *testing.T) {
			l := NewLeafNode([]byte{0xAB, 0xCD})
			path := toNibbles([]byte{0xAC})
			e := NewExtensionNode(path, NewHashNode(l.Hash()))
			tr := NewBillet(e.Hash(), true, storage.STTempStorage, newTestStore())
			tr.root = e

			// OK
			require.NoError(t, tr.RestoreHashNode(path, l))
			expected := NewHashNode(e.Hash()) // leaf should be collapsed immediately => extension should also be collapsed
			expected.Collapsed = true
			check(t, tr, expected, l, 1)

			// One more time (already restored and collapsed) => error expected, no refcount changes
			require.Error(t, tr.RestoreHashNode(path, l))
			check(t, tr, expected, l, 1)

			// Same path, but wrong hash => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode(path, NewLeafNode([]byte{0xAB, 0xEF})), ErrRestoreFailed))
			check(t, tr, expected, l, 1)

			// New path (changes in the MPT structure are not allowed) => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode(toNibbles([]byte{0xAB}), l), ErrRestoreFailed))
			check(t, tr, expected, l, 1)
		})

		t.Run("restore Hash", func(t *testing.T) {
			h := NewHashNode(util.Uint256{1, 2, 3})
			path := toNibbles([]byte{0xAC})
			e := NewExtensionNode(path, h)
			tr := NewBillet(e.Hash(), true, storage.STTempStorage, newTestStore())
			tr.root = e

			// no-op
			require.True(t, errors.Is(tr.RestoreHashNode(path, h), ErrRestoreFailed))
			check(t, tr, e, h, 0)
		})
	})

	t.Run("parent is Leaf", func(t *testing.T) {
		l := NewLeafNode([]byte{0xAB, 0xCD})
		path := []byte{}
		tr := NewBillet(l.Hash(), true, storage.STTempStorage, newTestStore())
		tr.root = l

		// Already restored => panic expected
		require.Panics(t, func() {
			_ = tr.RestoreHashNode(path, l)
		})

		// Same path, but wrong hash => error expected, no refcount changes
		require.True(t, errors.Is(tr.RestoreHashNode(path, NewLeafNode([]byte{0xAB, 0xEF})), ErrRestoreFailed))

		// Non-nil path, but MPT structure can't be changed => error expected, no refcount changes
		require.True(t, errors.Is(tr.RestoreHashNode(toNibbles([]byte{0xAC}), NewLeafNode([]byte{0xAB, 0xEF})), ErrRestoreFailed))
	})

	t.Run("parent is Branch", func(t *testing.T) {
		t.Run("middle child", func(t *testing.T) {
			l1 := NewLeafNode([]byte{0xAB, 0xCD})
			l2 := NewLeafNode([]byte{0xAB, 0xDE})
			b := NewBranchNode()
			b.Children[5] = NewHashNode(l1.Hash())
			b.Children[lastChild] = NewHashNode(l2.Hash())
			tr := NewBillet(b.Hash(), true, storage.STTempStorage, newTestStore())
			tr.root = b

			// OK
			path := []byte{0x05}
			require.NoError(t, tr.RestoreHashNode(path, l1))
			check(t, tr, b, l1, 1)

			// One more time (already restored) => panic expected.
			// It's an MPT pool duty to avoid such situations during real restore process.
			require.Panics(t, func() {
				_ = tr.RestoreHashNode(path, l1)
			})
			// No refcount changes expected.
			check(t, tr, b, l1, 1)

			// Same path, but wrong hash => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode(path, NewLeafNode([]byte{0xAD})), ErrRestoreFailed))
			check(t, tr, b, l1, 1)

			// New path pointing to the empty HashNode (changes in the MPT structure are not allowed) => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode([]byte{0x01}, l1), ErrRestoreFailed))
			check(t, tr, b, l1, 1)
		})

		t.Run("last child", func(t *testing.T) {
			l1 := NewLeafNode([]byte{0xAB, 0xCD})
			l2 := NewLeafNode([]byte{0xAB, 0xDE})
			b := NewBranchNode()
			b.Children[5] = NewHashNode(l1.Hash())
			b.Children[lastChild] = NewHashNode(l2.Hash())
			tr := NewBillet(b.Hash(), true, storage.STTempStorage, newTestStore())
			tr.root = b

			// OK
			path := []byte{}
			require.NoError(t, tr.RestoreHashNode(path, l2))
			check(t, tr, b, l2, 1)

			// One more time (already restored) => panic expected.
			// It's an MPT pool duty to avoid such situations during real restore process.
			require.Panics(t, func() {
				_ = tr.RestoreHashNode(path, l2)
			})
			// No refcount changes expected.
			check(t, tr, b, l2, 1)

			// Same path, but wrong hash => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode(path, NewLeafNode([]byte{0xAD})), ErrRestoreFailed))
			check(t, tr, b, l2, 1)
		})

		t.Run("two children with same hash", func(t *testing.T) {
			l := NewLeafNode([]byte{0xAB, 0xCD})
			b := NewBranchNode()
			// two same hashnodes => leaf's refcount expected to be 2 in the end.
			b.Children[3] = NewHashNode(l.Hash())
			b.Children[4] = NewHashNode(l.Hash())
			tr := NewBillet(b.Hash(), true, storage.STTempStorage, newTestStore())
			tr.root = b

			// OK
			require.NoError(t, tr.RestoreHashNode([]byte{0x03}, l))
			expected := b
			expected.Children[3].(*HashNode).Collapsed = true
			check(t, tr, b, l, 1)

			// Restore another node with the same hash => no error expected, refcount should be incremented.
			// Branch node should be collapsed.
			require.NoError(t, tr.RestoreHashNode([]byte{0x04}, l))
			res := NewHashNode(b.Hash())
			res.Collapsed = true
			check(t, tr, res, l, 2)
		})
	})

	t.Run("parent is Hash", func(t *testing.T) {
		l := NewLeafNode([]byte{0xAB, 0xCD})
		b := NewBranchNode()
		b.Children[3] = NewHashNode(l.Hash())
		b.Children[4] = NewHashNode(l.Hash())
		tr := NewBillet(b.Hash(), true, storage.STTempStorage, newTestStore())

		// Should fail, because if it's a hash node with non-empty path, then the node
		// has already been collapsed.
		require.Error(t, tr.RestoreHashNode([]byte{0x03}, l))
	})
}
