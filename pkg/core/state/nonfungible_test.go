package state

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newStruct(args ...interface{}) *stackitem.Struct {
	arr := make([]stackitem.Item, len(args))
	for i := range args {
		arr[i] = stackitem.Make(args[i])
	}
	return stackitem.NewStruct(arr)
}

func TestNFTTokenState_Serializable(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		s := &NFTTokenState{
			Owner: random.Uint160(),
			Name:  "random name",
		}
		id := s.ID()
		actual := new(NFTTokenState)
		testserdes.EncodeDecodeBinary(t, s, actual)
		require.Equal(t, id, actual.ID())
	})
	t.Run("invalid", func(t *testing.T) {
		errCases := []struct {
			name string
			item stackitem.Item
		}{
			{"invalid type", stackitem.NewByteArray([]byte{1, 2, 3})},
			{"invalid owner type",
				newStruct(stackitem.NewArray(nil), "name", "desc")},
			{"invalid owner uint160", newStruct("123", "name", "desc")},
			{"invalid name",
				newStruct(random.Uint160().BytesBE(), []byte{0x80}, "desc")},
		}

		for _, tc := range errCases {
			t.Run(tc.name, func(t *testing.T) {
				w := io.NewBufBinWriter()
				stackitem.EncodeBinaryStackItem(tc.item, w.BinWriter)
				require.NoError(t, w.Err)
				require.Error(t, testserdes.DecodeBinary(w.Bytes(), new(NFTTokenState)))
			})
		}
	})
}

func TestNFTTokenState_ToMap(t *testing.T) {
	s := &NFTTokenState{
		Owner: random.Uint160(),
		Name:  "random name",
	}
	m := s.ToMap()

	elems := m.Value().([]stackitem.MapElement)
	i := m.Index(stackitem.Make("name"))
	require.True(t, i < len(elems))
	require.Equal(t, []byte("random name"), elems[i].Value.Value())
}

func TestNFTAccountState_Serializable(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		s := &NFTAccountState{
			NEP17BalanceState: NEP17BalanceState{
				Balance: *big.NewInt(10),
			},
			Tokens: [][]byte{
				{1, 2, 3},
				{3},
			},
		}
		testserdes.EncodeDecodeBinary(t, s, new(NFTAccountState))
	})
	t.Run("invalid", func(t *testing.T) {
		errCases := []struct {
			name string
			item stackitem.Item
		}{
			{"small size", newStruct(42)},
			{"not an array", newStruct(11, stackitem.NewByteArray([]byte{}))},
			{"not an array",
				newStruct(11, stackitem.NewArray([]stackitem.Item{
					stackitem.NewArray(nil),
				}))},
		}

		for _, tc := range errCases {
			t.Run(tc.name, func(t *testing.T) {
				w := io.NewBufBinWriter()
				stackitem.EncodeBinaryStackItem(tc.item, w.BinWriter)
				require.NoError(t, w.Err)
				require.Error(t, testserdes.DecodeBinary(w.Bytes(), new(NFTAccountState)))
			})
		}
	})
}

func TestNFTAccountState_AddRemove(t *testing.T) {
	var s NFTAccountState
	require.True(t, s.Add([]byte{1, 2, 3}))
	require.EqualValues(t, 1, s.Balance.Int64())
	require.True(t, s.Add([]byte{1}))
	require.EqualValues(t, 2, s.Balance.Int64())

	require.False(t, s.Add([]byte{1, 2, 3}))
	require.EqualValues(t, 2, s.Balance.Int64())

	require.True(t, s.Remove([]byte{1}))
	require.EqualValues(t, 1, s.Balance.Int64())
	require.False(t, s.Remove([]byte{1}))
	require.EqualValues(t, 1, s.Balance.Int64())
	require.True(t, s.Remove([]byte{1, 2, 3}))
	require.EqualValues(t, 0, s.Balance.Int64())
}
