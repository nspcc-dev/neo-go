package mpt

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrieRemoveUntraceable(t *testing.T) {
	cfg := Config{
		Store:             newTestStore(),
		RefCountEnabled:   true,
		RemoveUntraceable: true,
	}
	tr := NewTrie(EmptyNode{}, cfg)

	var b Batch

	b.Add([]byte{1, 2}, []byte{3, 4})
	b.Add([]byte{1, 3}, []byte{3, 4})
	tr.Generation++
	_, err := tr.PutBatch(b)
	require.NoError(t, err)
	tr.Flush()
	r1 := tr.StateRoot()

	b.kv = b.kv[:0]
	b.Add([]byte{1, 2}, []byte{5, 6})
	tr.Generation++
	_, err = tr.PutBatch(b)
	require.NoError(t, err)
	tr.Flush()
	r2 := tr.StateRoot()

	b.kv = b.kv[:0]
	b.Add([]byte{1, 3}, []byte{7, 8})
	tr.Generation++
	_, err = tr.PutBatch(b)
	require.NoError(t, err)
	tr.Flush()
	r3 := tr.StateRoot()

	cfg.Generation = 1
	require.NoError(t, RemoveRoot(r1, cfg))
	_, err = tr.getFromStore(r1)
	require.Error(t, err)

	tr2 := NewTrie(NewHashNode(r2), cfg)
	tr2.testHas(t, []byte{1, 2}, []byte{5, 6})
	tr2.testHas(t, []byte{1, 3}, []byte{3, 4})

	tr3 := NewTrie(NewHashNode(r3), cfg)
	tr3.testHas(t, []byte{1, 2}, []byte{5, 6})
	tr3.testHas(t, []byte{1, 3}, []byte{7, 8})

	cfg.Generation = 2
	require.NoError(t, RemoveRoot(r2, cfg))
	_, err = tr.getFromStore(r2)
	require.Error(t, err)

	tr3 = NewTrie(NewHashNode(r3), cfg)
	tr3.testHas(t, []byte{1, 2}, []byte{5, 6})
	tr3.testHas(t, []byte{1, 3}, []byte{7, 8})
}
