package state

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestGASRecord_EncodeBinary(t *testing.T) {
	expected := &GASRecord{
		GASIndexPair{
			Index:       1,
			GASPerBlock: *big.NewInt(123),
		},
		GASIndexPair{
			Index:       2,
			GASPerBlock: *big.NewInt(7),
		},
	}
	testserdes.EncodeDecodeBinary(t, expected, new(GASRecord))
}

func TestGASRecord_fromStackItem(t *testing.T) {
	t.Run("NotArray", func(t *testing.T) {
		item := stackitem.Null{}
		require.Error(t, new(GASRecord).fromStackItem(item))
	})
	t.Run("InvalidFormat", func(t *testing.T) {
		item := stackitem.NewArray([]stackitem.Item{
			stackitem.NewStruct([]stackitem.Item{
				stackitem.NewBigInteger(big.NewInt(1)),
				stackitem.NewBool(true),
			}),
		})
		require.Error(t, new(GASRecord).fromStackItem(item))
	})
}
