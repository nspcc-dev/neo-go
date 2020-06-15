package native

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeBinary(t *testing.T) {
	expected := &BlockedAccounts{
		util.Uint160{1, 2, 3},
		util.Uint160{4, 5, 6},
	}
	actual := new(BlockedAccounts)
	testserdes.EncodeDecodeBinary(t, expected, actual)

	expected = &BlockedAccounts{}
	actual = new(BlockedAccounts)
	testserdes.EncodeDecodeBinary(t, expected, actual)
}

func TestBytesFromBytes(t *testing.T) {
	expected := BlockedAccounts{
		util.Uint160{1, 2, 3},
		util.Uint160{4, 5, 6},
	}
	actual, err := BlockedAccountsFromBytes(expected.Bytes())
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	expected = BlockedAccounts{}
	actual, err = BlockedAccountsFromBytes(expected.Bytes())
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestToStackItem(t *testing.T) {
	u1 := util.Uint160{1, 2, 3}
	u2 := util.Uint160{4, 5, 6}
	expected := BlockedAccounts{u1, u2}
	actual := stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(u1.BytesLE()),
		stackitem.NewByteArray(u2.BytesLE()),
	})
	require.Equal(t, expected.ToStackItem(), actual)

	expected = BlockedAccounts{}
	actual = stackitem.NewArray([]stackitem.Item{})
	require.Equal(t, expected.ToStackItem(), actual)
}
