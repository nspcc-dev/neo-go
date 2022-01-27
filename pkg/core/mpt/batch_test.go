package mpt

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/stretchr/testify/require"
)

func TestBatchAdd(t *testing.T) {
	b := new(Batch)
	b.Add([]byte{1}, []byte{2})
	b.Add([]byte{2, 16}, []byte{3})
	b.Add([]byte{2, 0}, []byte{4})
	b.Add([]byte{0, 1}, []byte{5})
	b.Add([]byte{2, 0}, []byte{6})
	expected := []keyValue{
		{[]byte{0, 0, 0, 1}, []byte{5}},
		{[]byte{0, 1}, []byte{2}},
		{[]byte{0, 2, 0, 0}, []byte{6}},
		{[]byte{0, 2, 1, 0}, []byte{3}},
	}
	require.Equal(t, expected, b.kv)
}

type pairs = [][2][]byte

func testIncompletePut(t *testing.T, ps pairs, n int, tr1, tr2 *Trie) {
	var b Batch
	for i, p := range ps {
		if i < n {
			if p[1] == nil {
				require.NoError(t, tr1.Delete(p[0]), "item %d", i)
			} else {
				require.NoError(t, tr1.Put(p[0], p[1]), "item %d", i)
			}
		} else if i == n {
			if p[1] == nil {
				require.Error(t, tr1.Delete(p[0]), "item %d", i)
			} else {
				require.Error(t, tr1.Put(p[0], p[1]), "item %d", i)
			}
		}
		b.Add(p[0], p[1])
	}

	num, err := tr2.PutBatch(b)
	if n == len(ps) {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
	}
	require.Equal(t, n, num)
	require.Equal(t, tr1.StateRoot(), tr2.StateRoot())

	t.Run("test restore", func(t *testing.T) {
		tr2.Flush()
		tr3 := NewTrie(NewHashNode(tr2.StateRoot()), false, storage.NewMemCachedStore(tr2.Store))
		for _, p := range ps[:n] {
			val, err := tr3.Get(p[0])
			if p[1] == nil {
				require.Error(t, err)
				continue
			}
			require.NoError(t, err, "key: %s", hex.EncodeToString(p[0]))
			require.Equal(t, p[1], val)
		}
	})
}

func testPut(t *testing.T, ps pairs, tr1, tr2 *Trie) {
	testIncompletePut(t, ps, len(ps), tr1, tr2)
}

func TestTrie_PutBatchLeaf(t *testing.T) {
	prepareLeaf := func(t *testing.T) (*Trie, *Trie) {
		tr1 := NewTrie(EmptyNode{}, false, newTestStore())
		tr2 := NewTrie(EmptyNode{}, false, newTestStore())
		require.NoError(t, tr1.Put([]byte{0}, []byte("value")))
		require.NoError(t, tr2.Put([]byte{0}, []byte("value")))
		return tr1, tr2
	}

	t.Run("remove", func(t *testing.T) {
		tr1, tr2 := prepareLeaf(t)
		var ps = pairs{{[]byte{0}, nil}}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("empty value", func(t *testing.T) {
		tr1, tr2 := prepareLeaf(t)
		var ps = pairs{{[]byte{0}, []byte{}}}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("replace", func(t *testing.T) {
		tr1, tr2 := prepareLeaf(t)
		var ps = pairs{{[]byte{0}, []byte("replace")}}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("remove and replace", func(t *testing.T) {
		tr1, tr2 := prepareLeaf(t)
		var ps = pairs{
			{[]byte{0}, nil},
			{[]byte{0, 2}, []byte("replace2")},
		}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("empty value and replace", func(t *testing.T) {
		tr1, tr2 := prepareLeaf(t)
		var ps = pairs{
			{[]byte{0}, []byte{}},
			{[]byte{0, 2}, []byte("replace2")},
		}
		testPut(t, ps, tr1, tr2)
	})
}

func TestTrie_PutBatchExtension(t *testing.T) {
	prepareExtension := func(t *testing.T) (*Trie, *Trie) {
		tr1 := NewTrie(EmptyNode{}, false, newTestStore())
		tr2 := NewTrie(EmptyNode{}, false, newTestStore())
		require.NoError(t, tr1.Put([]byte{1, 2}, []byte("value1")))
		require.NoError(t, tr2.Put([]byte{1, 2}, []byte("value1")))
		return tr1, tr2
	}

	t.Run("split, key len > 1", func(t *testing.T) {
		tr1, tr2 := prepareExtension(t)
		var ps = pairs{{[]byte{2, 3}, []byte("value2")}}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("split, key len = 1", func(t *testing.T) {
		tr1, tr2 := prepareExtension(t)
		var ps = pairs{{[]byte{1, 3}, []byte("value2")}}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("add to next", func(t *testing.T) {
		tr1, tr2 := prepareExtension(t)
		var ps = pairs{{[]byte{1, 2, 3}, []byte("value2")}}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("add to next with leaf", func(t *testing.T) {
		tr1, tr2 := prepareExtension(t)
		var ps = pairs{
			{[]byte{0}, []byte("value3")},
			{[]byte{1, 2, 3}, []byte("value2")},
		}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("remove value", func(t *testing.T) {
		tr1, tr2 := prepareExtension(t)
		var ps = pairs{{[]byte{1, 2}, nil}}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("empty value", func(t *testing.T) {
		tr1, tr2 := prepareExtension(t)
		var ps = pairs{{[]byte{1, 2}, []byte{}}}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("add to next, merge extension", func(t *testing.T) {
		tr1, tr2 := prepareExtension(t)
		var ps = pairs{
			{[]byte{1, 2}, nil},
			{[]byte{1, 2, 3}, []byte("value2")},
		}
		testPut(t, ps, tr1, tr2)
	})
}

func TestTrie_PutBatchBranch(t *testing.T) {
	prepareBranch := func(t *testing.T) (*Trie, *Trie) {
		tr1 := NewTrie(EmptyNode{}, false, newTestStore())
		tr2 := NewTrie(EmptyNode{}, false, newTestStore())
		require.NoError(t, tr1.Put([]byte{0x00, 2}, []byte("value1")))
		require.NoError(t, tr2.Put([]byte{0x00, 2}, []byte("value1")))
		require.NoError(t, tr1.Put([]byte{0x10, 3}, []byte("value2")))
		require.NoError(t, tr2.Put([]byte{0x10, 3}, []byte("value2")))
		return tr1, tr2
	}

	t.Run("simple add", func(t *testing.T) {
		tr1, tr2 := prepareBranch(t)
		var ps = pairs{{[]byte{0x20, 4}, []byte("value3")}}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("remove 1, transform to extension", func(t *testing.T) {
		tr1, tr2 := prepareBranch(t)
		var ps = pairs{{[]byte{0x00, 2}, nil}}
		testPut(t, ps, tr1, tr2)

		t.Run("non-empty child is hash node", func(t *testing.T) {
			tr1, tr2 := prepareBranch(t)
			tr1.Flush()
			tr1.Collapse(1)
			tr2.Flush()
			tr2.Collapse(1)

			var ps = pairs{{[]byte{0x00, 2}, nil}}
			testPut(t, ps, tr1, tr2)
			require.IsType(t, (*ExtensionNode)(nil), tr1.root)
		})
		t.Run("non-empty child is last node", func(t *testing.T) {
			tr1 := NewTrie(EmptyNode{}, false, newTestStore())
			tr2 := NewTrie(EmptyNode{}, false, newTestStore())
			require.NoError(t, tr1.Put([]byte{0x00, 2}, []byte("value1")))
			require.NoError(t, tr2.Put([]byte{0x00, 2}, []byte("value1")))
			require.NoError(t, tr1.Put([]byte{0x00}, []byte("value2")))
			require.NoError(t, tr2.Put([]byte{0x00}, []byte("value2")))

			tr1.Flush()
			tr1.Collapse(1)
			tr2.Flush()
			tr2.Collapse(1)

			var ps = pairs{{[]byte{0x00, 2}, nil}}
			testPut(t, ps, tr1, tr2)
		})
	})
	t.Run("incomplete put, transform to extension", func(t *testing.T) {
		tr1, tr2 := prepareBranch(t)
		var ps = pairs{
			{[]byte{0x00, 2}, nil},
			{[]byte{0x20, 2}, nil},
			{[]byte{0x30, 3}, []byte("won't be put")},
		}
		testIncompletePut(t, ps, 3, tr1, tr2)
	})
	t.Run("incomplete put, transform to empty", func(t *testing.T) {
		tr1, tr2 := prepareBranch(t)
		var ps = pairs{
			{[]byte{0x00, 2}, nil},
			{[]byte{0x10, 3}, nil},
			{[]byte{0x20, 2}, nil},
			{[]byte{0x30, 3}, []byte("won't be put")},
		}
		testIncompletePut(t, ps, 4, tr1, tr2)
	})
	t.Run("remove 2, become empty", func(t *testing.T) {
		tr1, tr2 := prepareBranch(t)
		var ps = pairs{
			{[]byte{0x00, 2}, nil},
			{[]byte{0x10, 3}, nil},
		}
		testPut(t, ps, tr1, tr2)
	})
}

func TestTrie_PutBatchHash(t *testing.T) {
	prepareHash := func(t *testing.T) (*Trie, *Trie) {
		tr1 := NewTrie(EmptyNode{}, false, newTestStore())
		tr2 := NewTrie(EmptyNode{}, false, newTestStore())
		require.NoError(t, tr1.Put([]byte{0x10}, []byte("value1")))
		require.NoError(t, tr2.Put([]byte{0x10}, []byte("value1")))
		require.NoError(t, tr1.Put([]byte{0x20}, []byte("value2")))
		require.NoError(t, tr2.Put([]byte{0x20}, []byte("value2")))
		tr1.Flush()
		tr2.Flush()
		return tr1, tr2
	}

	t.Run("good", func(t *testing.T) {
		tr1, tr2 := prepareHash(t)
		var ps = pairs{{[]byte{2}, []byte("value2")}}
		tr1.Collapse(0)
		tr1.Collapse(0)
		testPut(t, ps, tr1, tr2)
	})
	t.Run("incomplete, second hash not found", func(t *testing.T) {
		tr1, tr2 := prepareHash(t)
		var ps = pairs{
			{[]byte{0x10}, []byte("replace1")},
			{[]byte{0x20}, []byte("replace2")},
		}
		tr1.Collapse(1)
		tr2.Collapse(1)
		key := makeStorageKey(tr1.root.(*BranchNode).Children[2].Hash())
		require.NoError(t, tr1.Store.Delete(key))
		require.NoError(t, tr2.Store.Delete(key))
		testIncompletePut(t, ps, 1, tr1, tr2)
	})
}

func TestTrie_PutBatchEmpty(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		tr1 := NewTrie(EmptyNode{}, false, newTestStore())
		tr2 := NewTrie(EmptyNode{}, false, newTestStore())
		var ps = pairs{
			{[]byte{0}, []byte("value0")},
			{[]byte{1}, []byte("value1")},
			{[]byte{3}, []byte("value3")},
		}
		testPut(t, ps, tr1, tr2)
	})
	t.Run("incomplete", func(t *testing.T) {
		var ps = pairs{
			{[]byte{0}, []byte("replace0")},
			{[]byte{1}, []byte("replace1")},
			{[]byte{2}, nil},
			{[]byte{3}, []byte("replace3")},
		}
		tr1 := NewTrie(EmptyNode{}, false, newTestStore())
		tr2 := NewTrie(EmptyNode{}, false, newTestStore())
		testIncompletePut(t, ps, 4, tr1, tr2)
	})
}

// For the sake of coverage.
func TestTrie_InvalidNodeType(t *testing.T) {
	tr := NewTrie(EmptyNode{}, false, newTestStore())
	var b Batch
	b.Add([]byte{1}, []byte("value"))
	tr.root = Node(nil)
	require.Panics(t, func() { _, _ = tr.PutBatch(b) })
}

func TestTrie_PutBatch(t *testing.T) {
	tr1 := NewTrie(EmptyNode{}, false, newTestStore())
	tr2 := NewTrie(EmptyNode{}, false, newTestStore())
	var ps = pairs{
		{[]byte{1}, []byte{1}},
		{[]byte{2}, []byte{3}},
		{[]byte{4}, []byte{5}},
	}
	testPut(t, ps, tr1, tr2)

	ps = pairs{[2][]byte{{4}, {6}}}
	testPut(t, ps, tr1, tr2)

	ps = pairs{[2][]byte{{4}, nil}}
	testPut(t, ps, tr1, tr2)

	testPut(t, pairs{}, tr1, tr2)
}

var _ = printNode

// This function is unused, but is helpful for debugging
// as it provides more readable Trie representation compared to
// `spew.Dump()`.
func printNode(prefix string, n Node) {
	switch tn := n.(type) {
	case EmptyNode:
		fmt.Printf("%s empty\n", prefix)
		return
	case *HashNode:
		fmt.Printf("%s %s\n", prefix, tn.Hash().StringLE())
	case *BranchNode:
		for i, c := range tn.Children {
			if isEmpty(c) {
				continue
			}
			fmt.Printf("%s [%2d] ->\n", prefix, i)
			printNode(prefix+" ", c)
		}
	case *ExtensionNode:
		fmt.Printf("%s extension-> %s\n", prefix, hex.EncodeToString(tn.key))
		printNode(prefix+" ", tn.next)
	case *LeafNode:
		fmt.Printf("%s leaf-> %s\n", prefix, hex.EncodeToString(tn.value))
	}
}
