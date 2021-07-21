package mpt

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func newTestStore() *storage.MemCachedStore {
	return storage.NewMemCachedStore(storage.NewMemoryStore())
}

func newTestTrie(t *testing.T) *Trie {
	b := NewBranchNode()

	l1 := NewLeafNode([]byte{0xAB, 0xCD})
	b.Children[0] = NewExtensionNode([]byte{0x01}, l1)

	l2 := NewLeafNode([]byte{0x22, 0x22})
	b.Children[9] = NewExtensionNode([]byte{0x09}, l2)

	v := NewLeafNode([]byte("hello"))
	h := NewHashNode(v.Hash())
	b.Children[10] = NewExtensionNode([]byte{0x0e}, h)

	e := NewExtensionNode(toNibbles([]byte{0xAC}), b)
	tr := NewTrie(e, false, newTestStore())

	tr.putToStore(e)
	tr.putToStore(b)
	tr.putToStore(l1)
	tr.putToStore(l2)
	tr.putToStore(v)
	tr.putToStore(b.Children[0])
	tr.putToStore(b.Children[9])
	tr.putToStore(b.Children[10])

	return tr
}

func testTrieRefcount(t *testing.T, key1, key2 []byte) {
	tr := NewTrie(nil, true, storage.NewMemCachedStore(storage.NewMemoryStore()))
	require.NoError(t, tr.Put(key1, []byte{1}))
	tr.Flush()
	require.NoError(t, tr.Put(key2, []byte{1}))
	tr.Flush()
	tr.testHas(t, key1, []byte{1})
	tr.testHas(t, key2, []byte{1})

	// remove first, keep second
	require.NoError(t, tr.Delete(key1))
	tr.Flush()
	tr.testHas(t, key1, nil)
	tr.testHas(t, key2, []byte{1})

	// no-op
	require.NoError(t, tr.Put(key1, []byte{1}))
	require.NoError(t, tr.Delete(key1))
	tr.Flush()
	tr.testHas(t, key1, nil)
	tr.testHas(t, key2, []byte{1})

	// delete non-existent, refcount should not be updated
	require.NoError(t, tr.Delete(key1))
	tr.Flush()
	tr.testHas(t, key1, nil)
	tr.testHas(t, key2, []byte{1})
}

func TestTrie_Refcount(t *testing.T) {
	t.Run("Leaf", func(t *testing.T) {
		testTrieRefcount(t, []byte{0x11}, []byte{0x12})
	})
	t.Run("Extension", func(t *testing.T) {
		testTrieRefcount(t, []byte{0x10, 11}, []byte{0x11, 12})
	})
}

func TestTrie_PutIntoBranchNode(t *testing.T) {
	b := NewBranchNode()
	l := NewLeafNode([]byte{0x8})
	b.Children[0x7] = NewHashNode(l.Hash())
	b.Children[0x8] = NewHashNode(random.Uint256())
	tr := NewTrie(b, false, newTestStore())

	// empty hash node child
	require.NoError(t, tr.Put([]byte{0x66}, []byte{0x56}))
	tr.testHas(t, []byte{0x66}, []byte{0x56})
	require.True(t, isValid(tr.root))

	// missing hash
	require.Error(t, tr.Put([]byte{0x70}, []byte{0x42}))
	require.True(t, isValid(tr.root))

	// hash is in store
	tr.putToStore(l)
	require.NoError(t, tr.Put([]byte{0x70}, []byte{0x42}))
	require.True(t, isValid(tr.root))
}

func TestTrie_PutIntoExtensionNode(t *testing.T) {
	l := NewLeafNode([]byte{0x11})
	key := []byte{0x12}
	e := NewExtensionNode(toNibbles(key), NewHashNode(l.Hash()))
	tr := NewTrie(e, false, newTestStore())

	// missing hash
	require.Error(t, tr.Put(key, []byte{0x42}))

	tr.putToStore(l)
	require.NoError(t, tr.Put(key, []byte{0x42}))
	tr.testHas(t, key, []byte{0x42})
	require.True(t, isValid(tr.root))
}

func TestTrie_PutIntoHashNode(t *testing.T) {
	b := NewBranchNode()
	l := NewLeafNode(random.Bytes(5))
	e := NewExtensionNode([]byte{0x02}, l)
	b.Children[1] = NewHashNode(e.Hash())
	b.Children[9] = NewHashNode(random.Uint256())
	tr := NewTrie(b, false, newTestStore())

	tr.putToStore(e)

	t.Run("MissingLeafHash", func(t *testing.T) {
		_, err := tr.Get([]byte{0x12})
		require.Error(t, err)
	})

	tr.putToStore(l)

	val := random.Bytes(3)
	require.NoError(t, tr.Put([]byte{0x12, 0x34}, val))
	tr.testHas(t, []byte{0x12, 0x34}, val)
	tr.testHas(t, []byte{0x12}, l.value)
	require.True(t, isValid(tr.root))
}

func TestTrie_Put(t *testing.T) {
	trExp := newTestTrie(t)

	trAct := NewTrie(nil, false, newTestStore())
	require.NoError(t, trAct.Put([]byte{0xAC, 0x01}, []byte{0xAB, 0xCD}))
	require.NoError(t, trAct.Put([]byte{0xAC, 0x99}, []byte{0x22, 0x22}))
	require.NoError(t, trAct.Put([]byte{0xAC, 0xAE}, []byte("hello")))

	// Note: the exact tries differ because of ("acae":"hello") node is stored as Hash node in test trie.
	require.Equal(t, trExp.root.Hash(), trAct.root.Hash())
	require.True(t, isValid(trAct.root))
}

func TestTrie_PutInvalid(t *testing.T) {
	tr := NewTrie(nil, false, newTestStore())
	key, value := []byte("key"), []byte("value")

	// empty key
	require.Error(t, tr.Put(nil, value))

	// big key
	require.Error(t, tr.Put(make([]byte, maxPathLength+1), value))

	// big value
	require.Error(t, tr.Put(key, make([]byte, MaxValueLength+1)))

	// this is ok though
	require.NoError(t, tr.Put(key, value))
	tr.testHas(t, key, value)
}

func TestTrie_BigPut(t *testing.T) {
	tr := NewTrie(nil, false, newTestStore())
	items := []struct{ k, v string }{
		{"item with long key", "value1"},
		{"item with matching prefix", "value2"},
		{"another prefix", "value3"},
		{"another prefix 2", "value4"},
		{"another ", "value5"},
	}

	for i := range items {
		require.NoError(t, tr.Put([]byte(items[i].k), []byte(items[i].v)))
	}

	for i := range items {
		tr.testHas(t, []byte(items[i].k), []byte(items[i].v))
	}

	t.Run("Rewrite", func(t *testing.T) {
		k, v := []byte(items[0].k), []byte{0x01, 0x23}
		require.NoError(t, tr.Put(k, v))
		tr.testHas(t, k, v)
	})

	t.Run("Remove", func(t *testing.T) {
		k := []byte(items[1].k)
		require.NoError(t, tr.Put(k, []byte{}))
		tr.testHas(t, k, nil)
	})
}

func (tr *Trie) putToStore(n Node) {
	if n.Type() == HashT {
		panic("can't put hash node in trie")
	}
	if tr.refcountEnabled {
		tr.refcount[n.Hash()] = &cachedNode{
			bytes:    n.Bytes(),
			refcount: 1,
		}
		tr.updateRefCount(n.Hash())
	} else {
		_ = tr.Store.Put(makeStorageKey(n.Hash().BytesBE()), n.Bytes())
	}
}

func (tr *Trie) testHas(t *testing.T, key, value []byte) {
	v, err := tr.Get(key)
	if value == nil {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)
	require.Equal(t, value, v)
}

// isValid checks for 3 invariants:
// - BranchNode contains > 1 children
// - ExtensionNode do not contain another extension node
// - ExtensionNode do not have nil key
// It is used only during testing to catch possible bugs.
func isValid(curr Node) bool {
	switch n := curr.(type) {
	case *BranchNode:
		var count int
		for i := range n.Children {
			if !isValid(n.Children[i]) {
				return false
			}
			hn, ok := n.Children[i].(*HashNode)
			if !ok || !hn.IsEmpty() {
				count++
			}
		}
		return count > 1
	case *ExtensionNode:
		_, ok := n.next.(*ExtensionNode)
		return len(n.key) != 0 && !ok
	default:
		return true
	}
}

func TestTrie_Get(t *testing.T) {
	t.Run("HashNode", func(t *testing.T) {
		tr := newTestTrie(t)
		tr.testHas(t, []byte{0xAC, 0xAE}, []byte("hello"))
	})
	t.Run("UnfoldRoot", func(t *testing.T) {
		tr := newTestTrie(t)
		single := NewTrie(NewHashNode(tr.root.Hash()), false, tr.Store)
		single.testHas(t, []byte{0xAC}, nil)
		single.testHas(t, []byte{0xAC, 0x01}, []byte{0xAB, 0xCD})
		single.testHas(t, []byte{0xAC, 0x99}, []byte{0x22, 0x22})
		single.testHas(t, []byte{0xAC, 0xAE}, []byte("hello"))
	})
}

func TestTrie_Flush(t *testing.T) {
	pairs := map[string][]byte{
		"x":    []byte("value0"),
		"key1": []byte("value1"),
		"key2": []byte("value2"),
	}

	tr := NewTrie(nil, false, newTestStore())
	for k, v := range pairs {
		require.NoError(t, tr.Put([]byte(k), v))
	}

	tr.Flush()
	tr = NewTrie(NewHashNode(tr.StateRoot()), false, tr.Store)
	for k, v := range pairs {
		actual, err := tr.Get([]byte(k))
		require.NoError(t, err)
		require.Equal(t, v, actual)
	}
}

func TestTrie_Delete(t *testing.T) {
	t.Run("No GC", func(t *testing.T) {
		testTrieDelete(t, false)
	})
	t.Run("With GC", func(t *testing.T) {
		testTrieDelete(t, true)
	})
}

func testTrieDelete(t *testing.T, enableGC bool) {
	t.Run("Hash", func(t *testing.T) {
		t.Run("FromStore", func(t *testing.T) {
			l := NewLeafNode([]byte{0x12})
			tr := NewTrie(NewHashNode(l.Hash()), enableGC, newTestStore())
			t.Run("NotInStore", func(t *testing.T) {
				require.Error(t, tr.Delete([]byte{}))
			})

			tr.putToStore(l)
			tr.testHas(t, []byte{}, []byte{0x12})
			require.NoError(t, tr.Delete([]byte{}))
			tr.testHas(t, []byte{}, nil)
		})

		t.Run("Empty", func(t *testing.T) {
			tr := NewTrie(nil, enableGC, newTestStore())
			require.NoError(t, tr.Delete([]byte{}))
		})
	})

	t.Run("Leaf", func(t *testing.T) {
		l := NewLeafNode([]byte{0x12, 0x34})
		tr := NewTrie(l, enableGC, newTestStore())
		t.Run("NonExistentKey", func(t *testing.T) {
			require.NoError(t, tr.Delete([]byte{0x12}))
			tr.testHas(t, []byte{}, []byte{0x12, 0x34})
		})
		require.NoError(t, tr.Delete([]byte{}))
		tr.testHas(t, []byte{}, nil)
	})

	t.Run("Extension", func(t *testing.T) {
		t.Run("SingleKey", func(t *testing.T) {
			l := NewLeafNode([]byte{0x12, 0x34})
			e := NewExtensionNode([]byte{0x0A, 0x0B}, l)
			tr := NewTrie(e, enableGC, newTestStore())

			t.Run("NonExistentKey", func(t *testing.T) {
				require.NoError(t, tr.Delete([]byte{}))
				tr.testHas(t, []byte{0xAB}, []byte{0x12, 0x34})
			})

			require.NoError(t, tr.Delete([]byte{0xAB}))
			require.True(t, tr.root.(*HashNode).IsEmpty())
		})

		t.Run("MultipleKeys", func(t *testing.T) {
			b := NewBranchNode()
			b.Children[0] = NewExtensionNode([]byte{0x01}, NewLeafNode([]byte{0x12, 0x34}))
			b.Children[6] = NewExtensionNode([]byte{0x07}, NewLeafNode([]byte{0x56, 0x78}))
			e := NewExtensionNode([]byte{0x01, 0x02}, b)
			tr := NewTrie(e, enableGC, newTestStore())

			h := e.Hash()
			require.NoError(t, tr.Delete([]byte{0x12, 0x01}))
			tr.testHas(t, []byte{0x12, 0x01}, nil)
			tr.testHas(t, []byte{0x12, 0x67}, []byte{0x56, 0x78})

			require.NotEqual(t, h, tr.root.Hash())
			require.Equal(t, toNibbles([]byte{0x12, 0x67}), e.key)
			require.IsType(t, (*LeafNode)(nil), e.next)
		})
	})

	t.Run("Branch", func(t *testing.T) {
		t.Run("3 Children", func(t *testing.T) {
			b := NewBranchNode()
			b.Children[lastChild] = NewLeafNode([]byte{0x12})
			b.Children[0] = NewExtensionNode([]byte{0x01}, NewLeafNode([]byte{0x34}))
			b.Children[1] = NewExtensionNode([]byte{0x06}, NewLeafNode([]byte{0x56}))
			tr := NewTrie(b, enableGC, newTestStore())
			require.NoError(t, tr.Delete([]byte{0x16}))
			tr.testHas(t, []byte{}, []byte{0x12})
			tr.testHas(t, []byte{0x01}, []byte{0x34})
			tr.testHas(t, []byte{0x16}, nil)
		})
		t.Run("2 Children", func(t *testing.T) {
			newt := func(t *testing.T) *Trie {
				b := NewBranchNode()
				b.Children[lastChild] = NewLeafNode([]byte{0x12})
				l := NewLeafNode([]byte{0x34})
				e := NewExtensionNode([]byte{0x06}, l)
				b.Children[5] = NewHashNode(e.Hash())
				tr := NewTrie(b, enableGC, newTestStore())
				tr.putToStore(l)
				tr.putToStore(e)
				return tr
			}

			t.Run("DeleteLast", func(t *testing.T) {
				t.Run("MergeExtension", func(t *testing.T) {
					tr := newt(t)
					require.NoError(t, tr.Delete([]byte{}))
					tr.testHas(t, []byte{}, nil)
					tr.testHas(t, []byte{0x56}, []byte{0x34})
					require.IsType(t, (*ExtensionNode)(nil), tr.root)

					t.Run("WithHash, branch node replaced", func(t *testing.T) {
						ch := NewLeafNode([]byte{5, 6})
						h := ch.Hash()

						b := NewBranchNode()
						b.Children[3] = NewExtensionNode([]byte{4}, NewLeafNode([]byte{1, 2, 3}))
						b.Children[lastChild] = NewHashNode(h)

						tr := NewTrie(NewExtensionNode([]byte{1, 2}, b), enableGC, newTestStore())
						tr.putToStore(ch)

						require.NoError(t, tr.Delete([]byte{0x12, 0x34}))
						tr.testHas(t, []byte{0x12, 0x34}, nil)
						tr.testHas(t, []byte{0x12}, []byte{5, 6})
						require.IsType(t, (*ExtensionNode)(nil), tr.root)
						require.Equal(t, h, tr.root.(*ExtensionNode).next.Hash())
					})
				})

				t.Run("LeaveLeaf", func(t *testing.T) {
					c := NewBranchNode()
					c.Children[5] = NewLeafNode([]byte{0x05})
					c.Children[6] = NewLeafNode([]byte{0x06})

					b := NewBranchNode()
					b.Children[lastChild] = NewLeafNode([]byte{0x12})
					b.Children[5] = c
					tr := NewTrie(b, enableGC, newTestStore())

					require.NoError(t, tr.Delete([]byte{}))
					tr.testHas(t, []byte{}, nil)
					tr.testHas(t, []byte{0x55}, []byte{0x05})
					tr.testHas(t, []byte{0x56}, []byte{0x06})
					require.IsType(t, (*ExtensionNode)(nil), tr.root)
				})
			})

			t.Run("DeleteMiddle", func(t *testing.T) {
				tr := newt(t)
				require.NoError(t, tr.Delete([]byte{0x56}))
				tr.testHas(t, []byte{}, []byte{0x12})
				tr.testHas(t, []byte{0x56}, nil)
				require.IsType(t, (*LeafNode)(nil), tr.root)
			})
		})
	})
}

func TestTrie_PanicInvalidRoot(t *testing.T) {
	tr := &Trie{Store: newTestStore()}
	require.Panics(t, func() { _ = tr.Put([]byte{1}, []byte{2}) })
	require.Panics(t, func() { _, _ = tr.Get([]byte{1}) })
	require.Panics(t, func() { _ = tr.Delete([]byte{1}) })
}

func TestTrie_Collapse(t *testing.T) {
	t.Run("PanicNegative", func(t *testing.T) {
		tr := newTestTrie(t)
		require.Panics(t, func() { tr.Collapse(-1) })
	})
	t.Run("Depth=0", func(t *testing.T) {
		tr := newTestTrie(t)
		h := tr.root.Hash()

		_, ok := tr.root.(*HashNode)
		require.False(t, ok)

		tr.Collapse(0)
		_, ok = tr.root.(*HashNode)
		require.True(t, ok)
		require.Equal(t, h, tr.root.Hash())
	})
	t.Run("Branch,Depth=1", func(t *testing.T) {
		b := NewBranchNode()
		e := NewExtensionNode([]byte{0x01}, NewLeafNode([]byte("value1")))
		he := e.Hash()
		b.Children[0] = e
		hb := b.Hash()

		tr := NewTrie(b, false, newTestStore())
		tr.Collapse(1)

		newb, ok := tr.root.(*BranchNode)
		require.True(t, ok)
		require.Equal(t, hb, newb.Hash())
		require.IsType(t, (*HashNode)(nil), b.Children[0])
		require.Equal(t, he, b.Children[0].Hash())
	})
	t.Run("Extension,Depth=1", func(t *testing.T) {
		l := NewLeafNode([]byte("value"))
		hl := l.Hash()
		e := NewExtensionNode([]byte{0x01}, l)
		h := e.Hash()
		tr := NewTrie(e, false, newTestStore())
		tr.Collapse(1)

		newe, ok := tr.root.(*ExtensionNode)
		require.True(t, ok)
		require.Equal(t, h, newe.Hash())
		require.IsType(t, (*HashNode)(nil), newe.next)
		require.Equal(t, hl, newe.next.Hash())
	})
	t.Run("Leaf", func(t *testing.T) {
		l := NewLeafNode([]byte("value"))
		tr := NewTrie(l, false, newTestStore())
		tr.Collapse(10)
		require.Equal(t, NewLeafNode([]byte("value")), tr.root)
	})
	t.Run("Hash", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			tr := NewTrie(new(HashNode), false, newTestStore())
			require.NotPanics(t, func() { tr.Collapse(1) })
			hn, ok := tr.root.(*HashNode)
			require.True(t, ok)
			require.True(t, hn.IsEmpty())
		})

		h := random.Uint256()
		hn := NewHashNode(h)
		tr := NewTrie(hn, false, newTestStore())
		tr.Collapse(10)

		newRoot, ok := tr.root.(*HashNode)
		require.True(t, ok)
		require.Equal(t, NewHashNode(h), newRoot)
	})
}

func TestTrie_RestoreHashNode(t *testing.T) {
	check := func(t *testing.T, tr *Trie, expectedRoot Node, expectedNode Node, expectedRefCount uint32) {
		_ = expectedRoot.Hash()
		require.Equal(t, expectedRoot, tr.root)
		expectedBytes, err := tr.Store.Get(makeStorageKey(expectedNode.Hash().BytesBE()))
		if expectedRefCount != 0 {
			require.NoError(t, err)
			require.Equal(t, expectedRefCount, binary.LittleEndian.Uint32(expectedBytes[len(expectedBytes)-4:]))
		} else {
			require.True(t, errors.Is(err, storage.ErrKeyNotFound))
		}
		/*	if expectedRefCount == 0 {
				require.True(t, errors.Is(err, storage.ErrKeyNotFound))
				require.Nil(t, tr.refcount[expectedNode.Hash()])
			} else {
				require.NoError(t, err)
				require.Equal(t, expectedBytes, expectedNode.Bytes())
				require.Equal(t, expectedRefCount, tr.refcount[expectedNode.Hash()].refcount)
			}*/
	}

	t.Run("parent is Extension", func(t *testing.T) {
		t.Run("restore Branch", func(t *testing.T) {
			b := NewBranchNode()
			b.Children[0] = NewExtensionNode([]byte{0x01}, NewLeafNode([]byte{0xAB, 0xCD}))
			b.Children[5] = NewExtensionNode([]byte{0x01}, NewLeafNode([]byte{0xAB, 0xDE}))
			path := toNibbles([]byte{0xAC})
			e := NewExtensionNode(path, NewHashNode(b.Hash()))
			tr := NewTrie(e, true, newTestStore())

			// OK
			n := new(NodeObject)
			n.DecodeBinary(io.NewBinReaderFromBuf(b.Bytes()))
			require.Nil(t, tr.refcount[n.Hash()])
			require.NoError(t, tr.RestoreHashNode(path, n.Node))
			expected := NewExtensionNode(path, n.Node)
			check(t, tr, expected, n.Node, 1)

			// One more time (already restored) => no error expected, no refcount changes
			require.NoError(t, tr.RestoreHashNode(path, n.Node))
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
			tr := NewTrie(e, true, newTestStore())

			// OK
			require.Nil(t, tr.refcount[l.Hash()])
			require.NoError(t, tr.RestoreHashNode(path, l))
			expected := NewExtensionNode(path, l)
			check(t, tr, expected, l, 1)

			// One more time (already restored) => no error expected, no refcount changes
			require.NoError(t, tr.RestoreHashNode(path, l))
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
			tr := NewTrie(e, true, newTestStore())

			// no-op
			require.True(t, errors.Is(tr.RestoreHashNode(path, h), ErrRestoreFailed))
			check(t, tr, e, h, 0)
		})
	})

	t.Run("parent is Leaf", func(t *testing.T) {
		l := NewLeafNode([]byte{0xAB, 0xCD})
		path := []byte{}
		tr := NewTrie(l, true, newTestStore())
		tr.refcount[l.Hash()] = &cachedNode{bytes: l.Bytes(), refcount: 1}

		// Already restored => no error expected, no refcount changes
		require.NoError(t, tr.RestoreHashNode(path, l))
		check(t, tr, l, l, 1)
		bytes, err := tr.Store.Get(append([]byte{byte(storage.STStorage)}, path...))
		require.NoError(t, err)
		require.Equal(t, bytes, l.value)

		// Same path, but wrong hash => error expected, no refcount changes
		require.True(t, errors.Is(tr.RestoreHashNode(path, NewLeafNode([]byte{0xAB, 0xEF})), ErrRestoreFailed))
		check(t, tr, l, l, 1)

		// Non-nil path, but MPT structure can't be changed => error expected, no refcount changes
		require.True(t, errors.Is(tr.RestoreHashNode(toNibbles([]byte{0xAC}), NewLeafNode([]byte{0xAB, 0xEF})), ErrRestoreFailed))
		check(t, tr, l, l, 1)
	})

	t.Run("parent is Branch", func(t *testing.T) {
		t.Run("middle child", func(t *testing.T) {
			l1 := NewLeafNode([]byte{0xAB, 0xCD})
			l2 := NewLeafNode([]byte{0xAB, 0xDE})
			b := NewBranchNode()
			b.Children[5] = NewHashNode(l1.Hash())
			b.Children[lastChild] = NewHashNode(l2.Hash())
			tr := NewTrie(b, true, newTestStore())
			tr.putToStore(b)

			// OK
			path := []byte{0x05}
			require.Nil(t, tr.refcount[l1.Hash()])
			require.NoError(t, tr.RestoreHashNode(path, l1))
			expected := NewBranchNode()
			expected.Children[5] = l1
			expected.Children[lastChild] = NewHashNode(l2.Hash())
			check(t, tr, expected, l1, 1)

			// One more time (already restored) => no error expected, no refcount changes
			require.NoError(t, tr.RestoreHashNode(path, l1))
			check(t, tr, expected, l1, 1)

			// Same path, but wrong hash => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode(path, NewLeafNode([]byte{0xAD})), ErrRestoreFailed))
			check(t, tr, expected, l1, 1)

			// New path pointing to the empty HashNode (changes in the MPT structure are not allowed) => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode([]byte{0x01}, l1), ErrRestoreFailed))
			check(t, tr, expected, l1, 1)
		})

		t.Run("last child", func(t *testing.T) {
			l1 := NewLeafNode([]byte{0xAB, 0xCD})
			l2 := NewLeafNode([]byte{0xAB, 0xDE})
			b := NewBranchNode()
			b.Children[5] = NewHashNode(l1.Hash())
			b.Children[lastChild] = NewHashNode(l2.Hash())
			tr := NewTrie(b, true, newTestStore())
			tr.putToStore(b)

			// OK
			path := []byte{}
			require.Nil(t, tr.refcount[l1.Hash()])
			require.NoError(t, tr.RestoreHashNode(path, l2))
			expected := NewBranchNode()
			expected.Children[5] = NewHashNode(l1.Hash())
			expected.Children[lastChild] = l2
			check(t, tr, expected, l2, 1)

			// One more time (already restored) => no error expected, no refcount changes
			require.NoError(t, tr.RestoreHashNode(path, l2))
			check(t, tr, expected, l2, 1)

			// Same path, but wrong hash => error expected, no refcount changes
			require.True(t, errors.Is(tr.RestoreHashNode(path, NewLeafNode([]byte{0xAD})), ErrRestoreFailed))
			check(t, tr, expected, l2, 1)
		})
	})

	t.Run("parent is Hash", func(t *testing.T) {
		l := NewLeafNode([]byte{0xAB, 0xCD})
		b := NewBranchNode()
		// two same hashnodes => leaf's refcount expected to be 2.
		b.Children[3] = NewHashNode(l.Hash())
		b.Children[4] = NewHashNode(l.Hash())
		tr := NewTrie(NewHashNode(b.Hash()), true, newTestStore())
		tr.putToStore(b)

		// OK
		require.Nil(t, tr.refcount[l.Hash()])
		require.NoError(t, tr.RestoreHashNode([]byte{0x03}, l))
		expected := NewBranchNode()
		expected.Children[3] = l
		expected.Children[4] = NewHashNode(l.Hash())
		check(t, tr, expected, l, 1)

		// Restore another node with the same hash => no error expected, refcount should be incremented
		require.NoError(t, tr.RestoreHashNode([]byte{0x04}, l))
		expected.Children[4] = l
		check(t, tr, expected, l, 2)
	})
}
