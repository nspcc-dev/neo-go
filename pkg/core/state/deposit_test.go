package state

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeDeposit(t *testing.T) {
	d := &Deposit{Amount: big.NewInt(100500), Till: 888}

	depo := new(Deposit)
	testserdes.EncodeDecodeBinary(t, d, depo)
	item := stackitem.Make(42)
	data, err := stackitem.Serialize(item)
	require.NoError(t, err)
	require.Error(t, testserdes.DecodeBinary(data, depo))
}

func TestDepositFromStackItem(t *testing.T) {
	var d Deposit

	item := stackitem.Make(42)
	require.Error(t, d.fromStackItem(item))

	item = stackitem.NewStruct(nil)
	require.Error(t, d.fromStackItem(item))

	item = stackitem.NewStruct([]stackitem.Item{
		stackitem.NewStruct(nil),
		stackitem.NewStruct(nil),
	})
	require.Error(t, d.fromStackItem(item))

	item = stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(777),
		stackitem.NewStruct(nil),
	})
	require.Error(t, d.fromStackItem(item))

	item = stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(777),
		stackitem.Make(-1),
	})
	require.Error(t, d.fromStackItem(item))
	item = stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(777),
		stackitem.Make("somenonu64value"),
	})
	require.Error(t, d.fromStackItem(item))
	item = stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(777),
		stackitem.Make(888),
	})
	require.NoError(t, d.fromStackItem(item))
}
