package state

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestWhitelistedFeeContract_ToFromStackItem(t *testing.T) {
	c := &WhitelistFeeContract{
		Hash:   util.Uint160{1, 2, 3},
		Method: "doSmth",
		ArgCnt: 2,
		Fee:    3,
	}
	testserdes.ToFromStackItem(t, new(WhitelistFeeContract), c)

	t.Run("not a struct", func(t *testing.T) {
		c = new(WhitelistFeeContract)
		require.ErrorContains(t, c.FromStackItem(stackitem.Null{}), "not a struct")
	})
	t.Run("invalid struct", func(t *testing.T) {
		c = new(WhitelistFeeContract)
		require.ErrorContains(t, c.FromStackItem(stackitem.Make([]stackitem.Item{})), "invalid struct")
	})
	t.Run("invalid struct", func(t *testing.T) {
		c = new(WhitelistFeeContract)
		require.ErrorContains(t, c.FromStackItem(stackitem.Make([]stackitem.Item{})), "invalid struct")
	})
	t.Run("invalid hash", func(t *testing.T) {
		c = new(WhitelistFeeContract)
		require.ErrorContains(t, c.FromStackItem(stackitem.Make([]stackitem.Item{
			stackitem.Make([]byte{1, 2, 3}),
			stackitem.Make("doSmth"),
			stackitem.Make(3),
			stackitem.Make(4),
		})), "invalid hash")
	})
	t.Run("invalid method", func(t *testing.T) {
		c = new(WhitelistFeeContract)
		require.ErrorContains(t, c.FromStackItem(stackitem.Make([]stackitem.Item{
			stackitem.Make(util.Uint160{1, 2, 3}),
			stackitem.NewInterop(nil),
			stackitem.Make(3),
			stackitem.Make(4),
		})), "invalid method")
	})
	t.Run("invalid args", func(t *testing.T) {
		c = new(WhitelistFeeContract)
		require.ErrorContains(t, c.FromStackItem(stackitem.Make([]stackitem.Item{
			stackitem.Make(util.Uint160{1, 2, 3}),
			stackitem.Make("doSmth"),
			stackitem.Null{},
			stackitem.Make(4),
		})), "invalid argument count")
	})
	t.Run("invalid fee", func(t *testing.T) {
		c = new(WhitelistFeeContract)
		require.ErrorContains(t, c.FromStackItem(stackitem.Make([]stackitem.Item{
			stackitem.Make(util.Uint160{1, 2, 3}),
			stackitem.Make("doSmth"),
			stackitem.Make(4),
			stackitem.Null{},
		})), "invalid fee")
	})
}

func TestWhitelistFeeContract_ToSCParameter(t *testing.T) {
	c := &WhitelistFeeContract{
		Hash:   util.Uint160{1, 2, 3},
		Method: "doSmth",
		ArgCnt: 2,
		Fee:    3,
	}

	prm, err := c.ToSCParameter()
	require.NoError(t, err)
	require.Equal(t, smartcontract.ArrayType, prm.Type)
	arr, ok := prm.Value.([]smartcontract.Parameter)
	require.True(t, ok)
	require.Len(t, arr, 4)

	require.Equal(t, smartcontract.Parameter{Type: smartcontract.Hash160Type, Value: c.Hash}, arr[0])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.StringType, Value: c.Method}, arr[1])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(int64(c.ArgCnt))}, arr[2])
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.IntegerType, Value: big.NewInt(c.Fee)}, arr[3])

	t.Run("nil", func(t *testing.T) {
		var nilContract *WhitelistFeeContract
		prm, err := nilContract.ToSCParameter()
		require.NoError(t, err)
		require.Equal(t, smartcontract.Parameter{Type: smartcontract.AnyType}, prm)
	})

	t.Run("round trip via stack item", func(t *testing.T) {
		item, err := prm.ToStackItem()
		require.NoError(t, err)

		actual := new(WhitelistFeeContract)
		require.NoError(t, actual.FromStackItem(item))
		require.Equal(t, c, actual)
	})
}
