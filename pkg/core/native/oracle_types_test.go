package native

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func getInvalidTestFunc(actual io.Serializable, value interface{}) func(t *testing.T) {
	return func(t *testing.T) {
		w := io.NewBufBinWriter()
		it := stackitem.Make(value)
		stackitem.EncodeBinary(it, w.BinWriter)
		require.NoError(t, w.Err)
		require.Error(t, testserdes.DecodeBinary(w.Bytes(), actual))
	}
}

func TestIDList_EncodeBinary(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		l := &IDList{1, 4, 5}
		testserdes.EncodeDecodeBinary(t, l, new(IDList))
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("NotArray", getInvalidTestFunc(new(IDList), []byte{}))
		t.Run("InvalidElement", getInvalidTestFunc(new(IDList), []stackitem.Item{stackitem.Null{}}))
		t.Run("NotStackItem", func(t *testing.T) {
			require.Error(t, testserdes.DecodeBinary([]byte{0x77}, new(IDList)))
		})
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

func TestNodeList_EncodeBinary(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pub := priv.PublicKey()

	t.Run("Valid", func(t *testing.T) {
		l := &NodeList{pub}
		testserdes.EncodeDecodeBinary(t, l, new(NodeList))
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("NotArray", getInvalidTestFunc(new(NodeList), []byte{}))
		t.Run("InvalidElement", getInvalidTestFunc(new(NodeList), []stackitem.Item{stackitem.Null{}}))
		t.Run("InvalidKey", getInvalidTestFunc(new(NodeList),
			[]stackitem.Item{stackitem.NewByteArray([]byte{0x9})}))
		t.Run("NotStackItem", func(t *testing.T) {
			require.Error(t, testserdes.DecodeBinary([]byte{0x77}, new(NodeList)))
		})
	})
}
