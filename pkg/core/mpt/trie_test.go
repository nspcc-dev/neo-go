package mpt

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/stretchr/testify/require"
)

func newTestStore() *storage.MemCachedStore {
	return storage.NewMemCachedStore(storage.NewMemoryStore())
}

func newTestTrie(t *testing.T) *Trie {
	b := NewBranchNode()

	l1 := NewLeafNode([]byte{0xAB, 0xCD})
	b.Children[0] = NewExtensionNode([]byte{0x01}, l1)

	l3 := NewLeafNode([]byte{})
	b.Children[1] = NewExtensionNode([]byte{0x03}, l3)

	l2 := NewLeafNode([]byte{0x22, 0x22})
	b.Children[9] = NewExtensionNode([]byte{0x09}, l2)

	v := NewLeafNode([]byte("hello"))
	h := NewHashNode(v.Hash())
	b.Children[10] = NewExtensionNode([]byte{0x0e}, h)

	e := NewExtensionNode(toNibbles([]byte{0xAC}), b)
	tr := NewTrie(e, Config{Store: newTestStore()})

	tr.putToStore(e)
	tr.putToStore(b)
	tr.putToStore(l1)
	tr.putToStore(l2)
	tr.putToStore(l3)
	tr.putToStore(v)
	tr.putToStore(b.Children[0])
	tr.putToStore(b.Children[1])
	tr.putToStore(b.Children[9])
	tr.putToStore(b.Children[10])

	return tr
}

func testTrieRefcount(t *testing.T, key1, key2 []byte) {
	tr := NewTrie(nil, Config{Store: newTestStore(), RefCountEnabled: true})
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
	check := func(t *testing.T, value []byte) {
		b := NewBranchNode()
		l := NewLeafNode([]byte{0x8})
		b.Children[0x7] = NewHashNode(l.Hash())
		b.Children[0x8] = NewHashNode(random.Uint256())
		tr := NewTrie(b, Config{Store: newTestStore()})

		// empty hash node child
		require.NoError(t, tr.Put([]byte{0x66}, value))
		tr.testHas(t, []byte{0x66}, value)
		require.True(t, isValid(tr.root))

		// missing hash
		require.Error(t, tr.Put([]byte{0x70}, value))
		require.True(t, isValid(tr.root))

		// hash is in store
		tr.putToStore(l)
		require.NoError(t, tr.Put([]byte{0x70}, value))
		require.True(t, isValid(tr.root))
	}

	t.Run("non-empty value", func(t *testing.T) {
		check(t, []byte{0x42})
	})
	t.Run("empty value", func(t *testing.T) {
		check(t, []byte{})
	})
}

func TestTrie_PutIntoExtensionNode(t *testing.T) {
	check := func(t *testing.T, value []byte) {
		l := NewLeafNode([]byte{0x11})
		key := []byte{0x12}
		e := NewExtensionNode(toNibbles(key), NewHashNode(l.Hash()))
		tr := NewTrie(e, Config{Store: newTestStore()})

		// missing hash
		require.Error(t, tr.Put(key, value))

		tr.putToStore(l)
		require.NoError(t, tr.Put(key, value))
		tr.testHas(t, key, value)
		require.True(t, isValid(tr.root))
	}

	t.Run("non-empty value", func(t *testing.T) {
		check(t, []byte{0x42})
	})
	t.Run("empty value", func(t *testing.T) {
		check(t, []byte{})
	})
}

func TestTrie_PutIntoHashNode(t *testing.T) {
	check := func(t *testing.T, value []byte) {
		b := NewBranchNode()
		l := NewLeafNode(random.Bytes(5))
		e := NewExtensionNode([]byte{0x02}, l)
		b.Children[1] = NewHashNode(e.Hash())
		b.Children[9] = NewHashNode(random.Uint256())
		tr := NewTrie(b, Config{Store: newTestStore()})

		tr.putToStore(e)

		t.Run("MissingLeafHash", func(t *testing.T) {
			_, err := tr.Get([]byte{0x12})
			require.Error(t, err)
		})

		tr.putToStore(l)

		require.NoError(t, tr.Put([]byte{0x12, 0x34}, value))
		tr.testHas(t, []byte{0x12, 0x34}, value)
		tr.testHas(t, []byte{0x12}, l.value)
		require.True(t, isValid(tr.root))
	}

	t.Run("non-empty value", func(t *testing.T) {
		val := random.Bytes(3)
		check(t, val)
	})
	t.Run("empty value", func(t *testing.T) {
		check(t, []byte{})
	})
}

func TestTrie_Put(t *testing.T) {
	trExp := newTestTrie(t)

	trAct := NewTrie(nil, Config{Store: newTestStore()})
	require.NoError(t, trAct.Put([]byte{0xAC, 0x01}, []byte{0xAB, 0xCD}))
	require.NoError(t, trAct.Put([]byte{0xAC, 0x13}, []byte{}))
	require.NoError(t, trAct.Put([]byte{0xAC, 0x99}, []byte{0x22, 0x22}))
	require.NoError(t, trAct.Put([]byte{0xAC, 0xAE}, []byte("hello")))

	// Note: the exact tries differ because of ("acae":"hello") node is stored as Hash node in test trie.
	require.Equal(t, trExp.root.Hash(), trAct.root.Hash())
	require.True(t, isValid(trAct.root))
}

func TestTrie_PutInvalid(t *testing.T) {
	tr := NewTrie(nil, Config{Store: newTestStore()})
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
	tr := NewTrie(nil, Config{Store: newTestStore()})
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

	t.Run("Rewrite to empty", func(t *testing.T) {
		k, v := []byte(items[0].k), []byte{}
		require.NoError(t, tr.Put(k, v))
		tr.testHas(t, k, v)
	})

	t.Run("Remove", func(t *testing.T) {
		k := []byte(items[1].k)
		require.NoError(t, tr.Delete(k))
		tr.testHas(t, k, nil)
	})
}

func (tr *Trie) putToStore(n Node) {
	if n.Type() == HashT {
		panic("can't put hash node in trie")
	}
	if tr.RefCountEnabled {
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
			if !isEmpty(n.Children[i]) {
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
		single := NewTrie(NewHashNode(tr.root.Hash()), Config{Store: tr.Store})
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

	tr := NewTrie(nil, Config{Store: newTestStore()})
	for k, v := range pairs {
		require.NoError(t, tr.Put([]byte(k), v))
	}

	tr.Flush()
	tr = NewTrie(NewHashNode(tr.StateRoot()), Config{Store: tr.Store})
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
			tr := NewTrie(NewHashNode(l.Hash()), Config{Store: newTestStore(), RefCountEnabled: enableGC})
			t.Run("NotInStore", func(t *testing.T) {
				require.Error(t, tr.Delete([]byte{}))
			})

			tr.putToStore(l)
			tr.testHas(t, []byte{}, []byte{0x12})
			require.NoError(t, tr.Delete([]byte{}))
			tr.testHas(t, []byte{}, nil)
		})

		t.Run("Empty", func(t *testing.T) {
			tr := NewTrie(nil, Config{Store: newTestStore(), RefCountEnabled: enableGC})
			require.NoError(t, tr.Delete([]byte{}))
		})
	})

	t.Run("Leaf", func(t *testing.T) {
		check := func(t *testing.T, value []byte) {
			l := NewLeafNode(value)
			tr := NewTrie(l, Config{
				RefCountEnabled: enableGC,
				Store:           newTestStore(),
			})
			t.Run("NonExistentKey", func(t *testing.T) {
				require.NoError(t, tr.Delete([]byte{0x12}))
				tr.testHas(t, []byte{}, value)
			})
			require.NoError(t, tr.Delete([]byte{}))
			tr.testHas(t, []byte{}, nil)
		}
		t.Run("non-empty value", func(t *testing.T) {
			check(t, []byte{0x12, 0x34})
		})
		t.Run("empty value", func(t *testing.T) {
			check(t, []byte{})
		})
	})

	t.Run("Extension", func(t *testing.T) {
		t.Run("SingleKey", func(t *testing.T) {
			check := func(t *testing.T, value []byte) {
				l := NewLeafNode(value)
				e := NewExtensionNode([]byte{0x0A, 0x0B}, l)
				tr := NewTrie(e, Config{
					RefCountEnabled: enableGC,
					Store:           newTestStore(),
				})

				t.Run("NonExistentKey", func(t *testing.T) {
					require.NoError(t, tr.Delete([]byte{}))
					tr.testHas(t, []byte{0xAB}, value)
				})

				require.NoError(t, tr.Delete([]byte{0xAB}))
				require.IsType(t, EmptyNode{}, tr.root)
			}
			t.Run("non-empty value", func(t *testing.T) {
				check(t, []byte{0x12, 0x34})
			})
			t.Run("empty value", func(t *testing.T) {
				check(t, []byte{})
			})
		})

		t.Run("MultipleKeys", func(t *testing.T) {
			check := func(t *testing.T, value []byte) {
				b := NewBranchNode()
				b.Children[0] = NewExtensionNode([]byte{0x01}, NewLeafNode(value))
				b.Children[6] = NewExtensionNode([]byte{0x07}, NewLeafNode([]byte{0x56, 0x78}))
				e := NewExtensionNode([]byte{0x01, 0x02}, b)
				tr := NewTrie(e, Config{
					RefCountEnabled: enableGC,
					Store:           newTestStore(),
				})

				h := e.Hash()
				require.NoError(t, tr.Delete([]byte{0x12, 0x01}))
				tr.testHas(t, []byte{0x12, 0x01}, nil)
				tr.testHas(t, []byte{0x12, 0x67}, []byte{0x56, 0x78})

				require.NotEqual(t, h, tr.root.Hash())
				require.Equal(t, toNibbles([]byte{0x12, 0x67}), e.key)
				require.IsType(t, (*LeafNode)(nil), e.next)
			}
			t.Run("non-empty value", func(t *testing.T) {
				check(t, []byte{0x12, 0x34})
			})
			t.Run("empty value", func(t *testing.T) {
				check(t, []byte{})
			})
		})
	})

	t.Run("Branch", func(t *testing.T) {
		t.Run("3 Children", func(t *testing.T) {
			check := func(t *testing.T, value []byte) {
				b := NewBranchNode()
				b.Children[lastChild] = NewLeafNode([]byte{0x12})
				b.Children[0] = NewExtensionNode([]byte{0x01}, NewLeafNode([]byte{0x34}))
				b.Children[1] = NewExtensionNode([]byte{0x06}, NewLeafNode(value))
				tr := NewTrie(b, Config{
					RefCountEnabled: enableGC,
					Store:           newTestStore(),
				})
				require.NoError(t, tr.Delete([]byte{0x16}))
				tr.testHas(t, []byte{}, []byte{0x12})
				tr.testHas(t, []byte{0x01}, []byte{0x34})
				tr.testHas(t, []byte{0x16}, nil)
			}
			t.Run("non-empty value", func(t *testing.T) {
				check(t, []byte{0x56})
			})
			t.Run("empty value", func(t *testing.T) {
				check(t, []byte{})
			})
		})
		t.Run("2 Children", func(t *testing.T) {
			t.Run("DeleteLast", func(t *testing.T) {
				t.Run("MergeExtension", func(t *testing.T) {
					check := func(t *testing.T, value []byte) {
						b := NewBranchNode()
						b.Children[lastChild] = NewLeafNode(value)
						l := NewLeafNode([]byte{0x34})
						e := NewExtensionNode([]byte{0x06}, l)
						b.Children[5] = NewHashNode(e.Hash())
						tr := NewTrie(b, Config{
							RefCountEnabled: enableGC,
							Store:           newTestStore(),
						})
						tr.putToStore(l)
						tr.putToStore(e)
						require.NoError(t, tr.Delete([]byte{}))
						tr.testHas(t, []byte{}, nil)
						tr.testHas(t, []byte{0x56}, []byte{0x34})
						require.IsType(t, (*ExtensionNode)(nil), tr.root)
					}
					t.Run("non-empty value", func(t *testing.T) {
						check(t, []byte{0x12})
					})
					t.Run("empty value", func(t *testing.T) {
						check(t, []byte{})
					})

					t.Run("WithHash, branch node replaced", func(t *testing.T) {
						check := func(t *testing.T, value []byte) {
							ch := NewLeafNode([]byte{5, 6})
							h := ch.Hash()

							b := NewBranchNode()
							b.Children[3] = NewExtensionNode([]byte{4}, NewLeafNode(value))
							b.Children[lastChild] = NewHashNode(h)

							tr := NewTrie(NewExtensionNode([]byte{1, 2}, b), Config{
								RefCountEnabled: enableGC,
								Store:           newTestStore(),
							})
							tr.putToStore(ch)

							require.NoError(t, tr.Delete([]byte{0x12, 0x34}))
							tr.testHas(t, []byte{0x12, 0x34}, nil)
							tr.testHas(t, []byte{0x12}, []byte{5, 6})
							require.IsType(t, (*ExtensionNode)(nil), tr.root)
							require.Equal(t, h, tr.root.(*ExtensionNode).next.Hash())
						}
						t.Run("non-empty value", func(t *testing.T) {
							check(t, []byte{1, 2, 3})
						})
						t.Run("empty value", func(t *testing.T) {
							check(t, []byte{})
						})
					})
				})

				t.Run("LeaveLeaf", func(t *testing.T) {
					check := func(t *testing.T, value []byte) {
						c := NewBranchNode()
						c.Children[5] = NewLeafNode([]byte{0x05})
						c.Children[6] = NewLeafNode([]byte{0x06})

						b := NewBranchNode()
						b.Children[lastChild] = NewLeafNode(value)
						b.Children[5] = c
						tr := NewTrie(b, Config{
							RefCountEnabled: enableGC,
							Store:           newTestStore(),
						})

						require.NoError(t, tr.Delete([]byte{}))
						tr.testHas(t, []byte{}, nil)
						tr.testHas(t, []byte{0x55}, []byte{0x05})
						tr.testHas(t, []byte{0x56}, []byte{0x06})
						require.IsType(t, (*ExtensionNode)(nil), tr.root)
					}
					t.Run("non-empty value", func(t *testing.T) {
						check(t, []byte{0x12})
					})
					t.Run("empty value", func(t *testing.T) {
						check(t, []byte{})
					})
				})
			})

			t.Run("DeleteMiddle", func(t *testing.T) {
				check := func(t *testing.T, value []byte) {
					b := NewBranchNode()
					b.Children[lastChild] = NewLeafNode([]byte{0x12})
					l := NewLeafNode(value)
					e := NewExtensionNode([]byte{0x06}, l)
					b.Children[5] = NewHashNode(e.Hash())
					tr := NewTrie(b, Config{
						RefCountEnabled: enableGC,
						Store:           newTestStore(),
					})
					tr.putToStore(l)
					tr.putToStore(e)
					require.NoError(t, tr.Delete([]byte{0x56}))
					tr.testHas(t, []byte{}, []byte{0x12})
					tr.testHas(t, []byte{0x56}, nil)
					require.IsType(t, (*LeafNode)(nil), tr.root)
				}
				t.Run("non-empty value", func(t *testing.T) {
					check(t, []byte{0x34})
				})
				t.Run("empty value", func(t *testing.T) {
					check(t, []byte{})
				})
			})
		})
	})
}

func TestTrie_PanicInvalidRoot(t *testing.T) {
	tr := &Trie{Config: Config{Store: newTestStore()}}
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

		tr := NewTrie(b, Config{Store: newTestStore()})
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
		tr := NewTrie(e, Config{Store: newTestStore()})
		tr.Collapse(1)

		newe, ok := tr.root.(*ExtensionNode)
		require.True(t, ok)
		require.Equal(t, h, newe.Hash())
		require.IsType(t, (*HashNode)(nil), newe.next)
		require.Equal(t, hl, newe.next.Hash())
	})
	t.Run("Leaf", func(t *testing.T) {
		l := NewLeafNode([]byte("value"))
		tr := NewTrie(l, Config{Store: newTestStore()})
		tr.Collapse(10)
		require.Equal(t, NewLeafNode([]byte("value")), tr.root)
	})
	t.Run("Empty Leaf", func(t *testing.T) {
		l := NewLeafNode([]byte{})
		tr := NewTrie(l, Config{Store: newTestStore()})
		tr.Collapse(10)
		require.Equal(t, NewLeafNode([]byte{}), tr.root)
	})
	t.Run("Hash", func(t *testing.T) {
		t.Run("EmptyNode", func(t *testing.T) {
			tr := NewTrie(EmptyNode{}, Config{Store: newTestStore()})
			require.NotPanics(t, func() { tr.Collapse(1) })
			_, ok := tr.root.(EmptyNode)
			require.True(t, ok)
		})

		h := random.Uint256()
		hn := NewHashNode(h)
		tr := NewTrie(hn, Config{Store: newTestStore()})
		tr.Collapse(10)

		newRoot, ok := tr.root.(*HashNode)
		require.True(t, ok)
		require.Equal(t, NewHashNode(h), newRoot)
	})
}
