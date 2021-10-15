package mpt

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func prepareMPTCompat() *Trie {
	b := NewBranchNode()
	r := NewExtensionNode([]byte{0x0a, 0x0c}, b)
	v1 := NewLeafNode([]byte{0xab, 0xcd}) //key=ac01
	v2 := NewLeafNode([]byte{0x22, 0x22}) //key=ac
	v3 := NewLeafNode([]byte("existing")) //key=acae
	v4 := NewLeafNode([]byte("missing"))
	h3 := NewHashNode(v3.Hash())
	e1 := NewExtensionNode([]byte{0x01}, v1)
	e3 := NewExtensionNode([]byte{0x0e}, h3)
	e4 := NewExtensionNode([]byte{0x01}, v4)
	b.Children[0] = e1
	b.Children[10] = e3
	b.Children[16] = v2
	b.Children[15] = NewHashNode(e4.Hash())

	tr := NewTrie(r, true, newTestStore())
	tr.putToStore(r)
	tr.putToStore(b)
	tr.putToStore(e1)
	tr.putToStore(e3)
	tr.putToStore(v1)
	tr.putToStore(v2)
	tr.putToStore(v3)

	return tr
}

// TestCompatibility contains tests present in C# implementation.
// https://github.com/neo-project/neo-modules/blob/master/tests/Neo.Plugins.StateService.Tests/MPT/UT_MPTTrie.cs
// There are some differences, though:
// 1. In our implementation delete is silent, i.e. we do not return an error is the key is missing or empty.
//    However, we do return error when contents of hash node are missing from the store
//    (corresponds to exception in C# implementation). However, if the key is too big, an error is returned
//    (corresponds to exception in C# implementation).
// 2. In our implementation put returns error if something goes wrong, while C# implementation throws
//    an exception and returns nothing.
// 3. In our implementation get does not immediately return error in case of an empty key. An error is returned
//    only if value is missing from the storage. C# implementation checks that key is not empty and throws an error
//    otherwice. However, if the key is too big, an error is returned (corresponds to exception in C# implementation).
func TestCompatibility(t *testing.T) {
	mainTrie := prepareMPTCompat()

	t.Run("TryGet", func(t *testing.T) {
		tr := copyTrie(mainTrie)
		tr.testHas(t, []byte{0xac, 0x01}, []byte{0xab, 0xcd})
		tr.testHas(t, []byte{0xac}, []byte{0x22, 0x22})
		tr.testHas(t, []byte{0xab, 0x99}, nil)
		tr.testHas(t, []byte{0xac, 0x39}, nil)
		tr.testHas(t, []byte{0xac, 0x02}, nil)
		tr.testHas(t, []byte{0xac, 0x01, 0x00}, nil)
		tr.testHas(t, []byte{0xac, 0x99, 0x10}, nil)
		tr.testHas(t, []byte{0xac, 0xf1}, nil)
		tr.testHas(t, make([]byte, MaxKeyLength), nil)
	})

	t.Run("TryGetResolve", func(t *testing.T) {
		tr := copyTrie(mainTrie)
		tr.testHas(t, []byte{0xac, 0xae}, []byte("existing"))
	})

	t.Run("TryPut", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xac, 0x01}, []byte{0xab, 0xcd},
			[]byte{0xac}, []byte{0x22, 0x22},
			[]byte{0xac, 0xae}, []byte("existing"),
			[]byte{0xac, 0xf1}, []byte("missing"))

		require.Equal(t, mainTrie.root.Hash(), tr.root.Hash())
		require.Error(t, tr.Put(nil, []byte{0x01}))
		require.Error(t, tr.Put([]byte{0x01}, nil))
		require.Error(t, tr.Put(make([]byte, MaxKeyLength+1), nil))
		require.Error(t, tr.Put([]byte{0x01}, make([]byte, MaxValueLength+1)))
		require.Equal(t, mainTrie.root.Hash(), tr.root.Hash())
		require.NoError(t, tr.Put([]byte{0x01}, []byte{}))
		require.NoError(t, tr.Put([]byte{0xac, 0x01}, []byte{0xab}))
	})

	t.Run("PutCantResolve", func(t *testing.T) {
		tr := copyTrie(mainTrie)
		require.Error(t, tr.Put([]byte{0xac, 0xf1, 0x11}, []byte{1}))
	})

	t.Run("TryDelete", func(t *testing.T) {
		tr := copyTrie(mainTrie)
		tr.testHas(t, []byte{0xac}, []byte{0x22, 0x22})
		require.NoError(t, tr.Delete([]byte{0x0c, 0x99}))
		require.NoError(t, tr.Delete(nil))
		require.NoError(t, tr.Delete([]byte{0xac, 0x20}))

		require.Error(t, tr.Delete([]byte{0xac, 0xf1}))           // error for can't resolve
		require.Error(t, tr.Delete(make([]byte, MaxKeyLength+1))) // error for too big key

		// In our implementation missing keys are ignored.
		require.NoError(t, tr.Delete([]byte{0xac}))
		require.NoError(t, tr.Delete([]byte{0xac, 0xae, 0x01}))
		require.NoError(t, tr.Delete([]byte{0xac, 0xae}))

		require.Equal(t, "cb06925428b7c727375c7fdd943a302fe2c818cf2e2eaf63a7932e3fd6cb3408",
			tr.root.Hash().StringLE())
	})

	t.Run("DeleteRemainCanResolve", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xac, 0x00}, []byte{0xab, 0xcd},
			[]byte{0xac, 0x10}, []byte{0xab, 0xcd})
		tr.Flush()

		tr2 := copyTrie(tr)
		require.NoError(t, tr2.Delete([]byte{0xac, 0x00}))

		tr2.Flush()
		require.NoError(t, tr2.Delete([]byte{0xac, 0x10}))
	})

	t.Run("DeleteRemainCantResolve", func(t *testing.T) {
		b := NewBranchNode()
		r := NewExtensionNode([]byte{0x0a, 0x0c}, b)
		v1 := NewLeafNode([]byte{0xab, 0xcd})
		v4 := NewLeafNode([]byte("missing"))
		e1 := NewExtensionNode([]byte{0x01}, v1)
		e4 := NewExtensionNode([]byte{0x01}, v4)
		b.Children[0] = e1
		b.Children[15] = NewHashNode(e4.Hash())

		tr := NewTrie(NewHashNode(r.Hash()), false, newTestStore())
		tr.putToStore(r)
		tr.putToStore(b)
		tr.putToStore(e1)
		tr.putToStore(v1)

		require.Error(t, tr.Delete([]byte{0xac, 0x01}))
	})

	t.Run("DeleteSameValue", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xac, 0x01}, []byte{0xab, 0xcd},
			[]byte{0xac, 0x02}, []byte{0xab, 0xcd})
		tr.testHas(t, []byte{0xac, 0x01}, []byte{0xab, 0xcd})
		tr.testHas(t, []byte{0xac, 0x02}, []byte{0xab, 0xcd})

		require.NoError(t, tr.Delete([]byte{0xac, 0x01}))
		tr.testHas(t, []byte{0xac, 0x02}, []byte{0xab, 0xcd})
		tr.Flush()

		tr2 := NewTrie(NewHashNode(tr.root.Hash()), false, tr.Store)
		tr2.testHas(t, []byte{0xac, 0x02}, []byte{0xab, 0xcd})
	})

	t.Run("BranchNodeRemainValue", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xac, 0x11}, []byte{0xac, 0x11},
			[]byte{0xac, 0x22}, []byte{0xac, 0x22},
			[]byte{0xac}, []byte{0xac})
		tr.Flush()
		checkBatchSize(t, tr, 7)

		require.NoError(t, tr.Delete([]byte{0xac, 0x11}))
		tr.Flush()
		checkBatchSize(t, tr, 5)

		require.NoError(t, tr.Delete([]byte{0xac, 0x22}))
		tr.Flush()
		checkBatchSize(t, tr, 2)
	})

	t.Run("GetProof", func(t *testing.T) {
		b := NewBranchNode()
		r := NewExtensionNode([]byte{0x0a, 0x0c}, b)
		v1 := NewLeafNode([]byte{0xab, 0xcd}) //key=ac01
		v2 := NewLeafNode([]byte{0x22, 0x22}) //key=ac
		v3 := NewLeafNode([]byte("existing")) //key=acae
		v4 := NewLeafNode([]byte("missing"))
		h3 := NewHashNode(v3.Hash())
		e1 := NewExtensionNode([]byte{0x01}, v1)
		e3 := NewExtensionNode([]byte{0x0e}, h3)
		e4 := NewExtensionNode([]byte{0x01}, v4)
		b.Children[0] = e1
		b.Children[10] = e3
		b.Children[16] = v2
		b.Children[15] = NewHashNode(e4.Hash())

		tr := NewTrie(NewHashNode(r.Hash()), true, mainTrie.Store)
		require.Equal(t, r.Hash(), tr.root.Hash())

		// Tail bytes contain reference counter thus check for prefix.
		proof := testGetProof(t, tr, []byte{0xac, 0x01}, 4)
		require.True(t, bytes.HasPrefix(r.Bytes(), proof[0]))
		require.True(t, bytes.HasPrefix(b.Bytes(), proof[1]))
		require.True(t, bytes.HasPrefix(e1.Bytes(), proof[2]))
		require.True(t, bytes.HasPrefix(v1.Bytes(), proof[3]))

		testGetProof(t, tr, []byte{0xac}, 3)
		testGetProof(t, tr, []byte{0xac, 0x10}, 0)
		testGetProof(t, tr, []byte{0xac, 0xae}, 4)
		testGetProof(t, tr, nil, 0)
		testGetProof(t, tr, []byte{0xac, 0x01, 0x00}, 0)
		testGetProof(t, tr, []byte{0xac, 0xf1}, 0)
		testGetProof(t, tr, make([]byte, MaxKeyLength), 0)
	})

	t.Run("VerifyProof", func(t *testing.T) {
		tr := copyTrie(mainTrie)
		proof := testGetProof(t, tr, []byte{0xac, 0x01}, 4)
		value, ok := VerifyProof(tr.root.Hash(), []byte{0xac, 0x01}, proof)
		require.True(t, ok)
		require.Equal(t, []byte{0xab, 0xcd}, value)
	})

	t.Run("AddLongerKey", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xab}, []byte{0x01},
			[]byte{0xab, 0xcd}, []byte{0x02})
		tr.testHas(t, []byte{0xab}, []byte{0x01})
	})

	t.Run("SplitKey", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xab, 0xcd}, []byte{0x01},
			[]byte{0xab}, []byte{0x02})
		testGetProof(t, tr, []byte{0xab, 0xcd}, 4)

		tr2 := newFilledTrie(t,
			[]byte{0xab}, []byte{0x02},
			[]byte{0xab, 0xcd}, []byte{0x01})
		testGetProof(t, tr, []byte{0xab, 0xcd}, 4)

		require.Equal(t, tr.root.Hash(), tr2.root.Hash())
	})

	t.Run("Reference", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xa1, 0x01}, []byte{0x01},
			[]byte{0xa2, 0x01}, []byte{0x01},
			[]byte{0xa3, 0x01}, []byte{0x01})
		tr.Flush()

		tr2 := copyTrie(tr)
		require.NoError(t, tr2.Delete([]byte{0xa3, 0x01}))
		tr2.Flush()

		tr3 := copyTrie(tr2)
		require.NoError(t, tr3.Delete([]byte{0xa2, 0x01}))
		tr3.testHas(t, []byte{0xa1, 0x01}, []byte{0x01})
	})

	t.Run("Reference2", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xa1, 0x01}, []byte{0x01},
			[]byte{0xa2, 0x01}, []byte{0x01},
			[]byte{0xa3, 0x01}, []byte{0x01})
		tr.Flush()
		checkBatchSize(t, tr, 4)

		require.NoError(t, tr.Delete([]byte{0xa3, 0x01}))
		tr.Flush()
		checkBatchSize(t, tr, 4)

		require.NoError(t, tr.Delete([]byte{0xa2, 0x01}))
		tr.Flush()
		checkBatchSize(t, tr, 2)
		tr.testHas(t, []byte{0xa1, 0x01}, []byte{0x01})
	})

	t.Run("ExtensionDeleteDirty", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xa1}, []byte{0x01},
			[]byte{0xa2}, []byte{0x02})
		tr.Flush()
		checkBatchSize(t, tr, 4)

		tr1 := copyTrie(tr)
		require.NoError(t, tr1.Delete([]byte{0xa1}))
		tr1.Flush()
		require.Equal(t, 2, len(tr1.Store.GetBatch().Put))

		tr2 := copyTrie(tr1)
		require.NoError(t, tr2.Delete([]byte{0xa2}))
		tr2.Flush()
		require.Equal(t, 0, len(tr2.Store.GetBatch().Put))
	})

	t.Run("BranchDeleteDirty", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0x10}, []byte{0x01},
			[]byte{0x20}, []byte{0x02},
			[]byte{0x30}, []byte{0x03})
		tr.Flush()
		checkBatchSize(t, tr, 7)

		tr1 := copyTrie(tr)
		require.NoError(t, tr1.Delete([]byte{0x10}))
		tr1.Flush()

		tr2 := copyTrie(tr1)
		require.NoError(t, tr2.Delete([]byte{0x20}))
		tr2.Flush()
		require.Equal(t, 2, len(tr2.Store.GetBatch().Put))

		tr3 := copyTrie(tr2)
		require.NoError(t, tr3.Delete([]byte{0x30}))
		tr3.Flush()
		require.Equal(t, 0, len(tr3.Store.GetBatch().Put))
	})

	t.Run("ExtensionPutDirty", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0xa1}, []byte{0x01},
			[]byte{0xa2}, []byte{0x02})
		tr.Flush()
		checkBatchSize(t, tr, 4)

		tr1 := copyTrie(tr)
		require.NoError(t, tr1.Put([]byte{0xa3}, []byte{0x03}))
		tr1.Flush()
		require.Equal(t, 5, len(tr1.Store.GetBatch().Put))
	})

	t.Run("BranchPutDirty", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0x10}, []byte{0x01},
			[]byte{0x20}, []byte{0x02})
		tr.Flush()
		checkBatchSize(t, tr, 5)

		tr1 := copyTrie(tr)
		require.NoError(t, tr1.Put([]byte{0x30}, []byte{0x03}))
		tr1.Flush()
		checkBatchSize(t, tr1, 7)
	})

	t.Run("EmptyValueIssue633", func(t *testing.T) {
		tr := newFilledTrie(t,
			[]byte{0x01}, []byte{})
		tr.Flush()
		checkBatchSize(t, tr, 2)

		proof := testGetProof(t, tr, []byte{0x01}, 2)
		value, ok := VerifyProof(tr.root.Hash(), []byte{0x01}, proof)
		require.True(t, ok)
		require.Equal(t, []byte{}, value)
	})
}

func copyTrie(t *Trie) *Trie {
	return NewTrie(NewHashNode(t.root.Hash()), t.refcountEnabled, t.Store)
}

func checkBatchSize(t *testing.T, tr *Trie, n int) {
	require.Equal(t, n, len(tr.Store.GetBatch().Put))
}

func testGetProof(t *testing.T, tr *Trie, key []byte, size int) [][]byte {
	proof, err := tr.GetProof(key)
	if size == 0 {
		require.Error(t, err)
		return proof
	}

	require.NoError(t, err)
	require.Equal(t, size, len(proof))
	return proof
}

func newFilledTrie(t *testing.T, args ...[]byte) *Trie {
	tr := NewTrie(nil, true, newTestStore())
	for i := 0; i < len(args); i += 2 {
		require.NoError(t, tr.Put(args[i], args[i+1]))
	}
	return tr
}

func TestCompatibility_Find(t *testing.T) {
	check := func(t *testing.T, from []byte, expectedResLen int) {
		tr := NewTrie(nil, false, newTestStore())
		require.NoError(t, tr.Put([]byte("aa"), []byte("02")))
		require.NoError(t, tr.Put([]byte("aa10"), []byte("03")))
		require.NoError(t, tr.Put([]byte("aa50"), []byte("04")))
		res, err := tr.Find([]byte("aa"), from, 10)
		require.NoError(t, err)
		require.Equal(t, expectedResLen, len(res))
	}
	t.Run("no from", func(t *testing.T) {
		check(t, nil, 3)
	})
	t.Run("from is not in tree", func(t *testing.T) {
		t.Run("matching", func(t *testing.T) {
			check(t, []byte("30"), 1)
		})
		t.Run("non-matching", func(t *testing.T) {
			check(t, []byte("60"), 0)
		})
	})
	t.Run("from is in tree", func(t *testing.T) {
		check(t, []byte("10"), 1) // without `from` key
	})
	t.Run("from matching start", func(t *testing.T) {
		check(t, []byte{}, 2) // without `from` key
	})
	t.Run("TestFindStatesIssue652", func(t *testing.T) {
		tr := NewTrie(nil, false, newTestStore())
		// root is an extension node with key=abc; next=branch
		require.NoError(t, tr.Put([]byte("abc1"), []byte("01")))
		require.NoError(t, tr.Put([]byte("abc3"), []byte("02")))
		tr.Flush()
		// find items with extension's key prefix
		t.Run("from > start", func(t *testing.T) {
			res, err := tr.Find([]byte("ab"), []byte("d2"), 100)
			require.NoError(t, err)
			// nothing should be found, because from[0]=`d` > key[2]=`c`
			require.Equal(t, 0, len(res))
		})

		t.Run("from < start", func(t *testing.T) {
			res, err := tr.Find([]byte("ab"), []byte("b2"), 100)
			require.NoError(t, err)
			// all items should be included into the result, because from[0]=`b` < key[2]=`c`
			require.Equal(t, 2, len(res))
		})

		t.Run("from and start have common prefix", func(t *testing.T) {
			res, err := tr.Find([]byte("ab"), []byte("c"), 100)
			require.NoError(t, err)
			// all items should be included into the result, because from[0] == key[2]
			require.Equal(t, 2, len(res))
		})

		t.Run("from equals to item key", func(t *testing.T) {
			res, err := tr.Find([]byte("ab"), []byte("c1"), 100)
			require.NoError(t, err)
			require.Equal(t, 1, len(res))
		})
	})
}
