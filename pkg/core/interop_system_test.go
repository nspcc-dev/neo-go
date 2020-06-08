package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestBCGetTransaction(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()

	t.Run("success", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 0))
		v.Estack().PushVal(tx.Hash().BytesBE())
		err := bcGetTransaction(context, v)
		require.NoError(t, err)

		value := v.Estack().Pop().Value()
		actual, ok := value.([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, 8, len(actual))
		require.Equal(t, tx.Hash().BytesBE(), actual[0].Value().([]byte))
		require.Equal(t, int64(tx.Version), actual[1].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.Nonce), actual[2].Value().(*big.Int).Int64())
		require.Equal(t, tx.Sender.BytesBE(), actual[3].Value().([]byte))
		require.Equal(t, int64(tx.SystemFee), actual[4].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.NetworkFee), actual[5].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.ValidUntilBlock), actual[6].Value().(*big.Int).Int64())
		require.Equal(t, tx.Script, actual[7].Value().([]byte))
	})

	t.Run("isn't traceable", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 1))
		v.Estack().PushVal(tx.Hash().BytesBE())
		err := bcGetTransaction(context, v)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad hash", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 1))
		v.Estack().PushVal(tx.Hash().BytesLE())
		err := bcGetTransaction(context, v)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})
}
