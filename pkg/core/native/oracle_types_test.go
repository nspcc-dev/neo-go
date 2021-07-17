package native

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func getInvalidTestFunc(actual stackitem.Convertible, value interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		it := stackitem.Make(value)
		require.Error(t, actual.FromStackItem(it))
	}
}

func TestIDListToFromSI(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		l := &IDList{1, 4, 5}
		var l2 = new(IDList)
		testserdes.ToFromStackItem(t, l, l2)
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("NotArray", getInvalidTestFunc(new(IDList), []byte{}))
		t.Run("InvalidElement", getInvalidTestFunc(new(IDList), []stackitem.Item{stackitem.Null{}}))
	})
}

func TestIDList_Remove(t *testing.T) {
	l := IDList{1, 4, 5}

	// missing
	require.False(t, l.Remove(2))
	require.Equal(t, IDList{1, 4, 5}, l)

	// middle
	require.True(t, l.Remove(4))
	require.Equal(t, IDList{1, 5}, l)

	// last
	require.True(t, l.Remove(5))
	require.Equal(t, IDList{1}, l)
}

func TestNodeListToFromSI(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()

	t.Run("Valid", func(t *testing.T) {
		l := &NodeList{pub}
		var l2 = new(NodeList)
		testserdes.ToFromStackItem(t, l, l2)
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("NotArray", getInvalidTestFunc(new(NodeList), []byte{}))
		t.Run("InvalidElement", getInvalidTestFunc(new(NodeList), []stackitem.Item{stackitem.Null{}}))
		t.Run("InvalidKey", getInvalidTestFunc(new(NodeList),
			[]stackitem.Item{stackitem.NewByteArray([]byte{0x9})}))
	})
}
