package state

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestNEP17Balance_Bytes(t *testing.T) {
	var b NEP17Balance
	b.Balance.SetInt64(0x12345678910)

	data, err := stackitem.SerializeConvertible(&b)
	require.NoError(t, err)
	require.Equal(t, data, b.Bytes(nil))

	t.Run("reuse buffer", func(t *testing.T) {
		buf := make([]byte, 100)
		ret := b.Bytes(buf[:0])
		require.Equal(t, ret, buf[:len(ret)])
	})

	actual, err := NEP17BalanceFromBytes(data)
	require.NoError(t, err)
	require.Equal(t, &b, actual)
}

func TestNEP17BalanceFromBytesInvalid(t *testing.T) {
	b, err := NEP17BalanceFromBytes(nil) // 0 is ok
	require.NoError(t, err)
	require.Equal(t, int64(0), b.Balance.Int64())

	_, err = NEP17BalanceFromBytes([]byte{byte(stackitem.StructT)})
	require.Error(t, err)

	_, err = NEP17BalanceFromBytes([]byte{byte(stackitem.IntegerT), 4, 0, 1, 2, 3})
	require.Error(t, err)

	_, err = NEP17BalanceFromBytes([]byte{byte(stackitem.StructT), 0, byte(stackitem.IntegerT), 1, 1})
	require.Error(t, err)

	_, err = NEP17BalanceFromBytes([]byte{byte(stackitem.StructT), 1, byte(stackitem.ByteArrayT), 1, 1})
	require.Error(t, err)

	_, err = NEP17BalanceFromBytes([]byte{byte(stackitem.StructT), 1, byte(stackitem.IntegerT), 2, 1})
	require.Error(t, err)
}

func TestNEOBalanceSerialization(t *testing.T) {
	var b = NEOBalance{
		NEP17Balance:  NEP17Balance{*big.NewInt(100500)},
		BalanceHeight: 42,
	}
	si, err := b.ToStackItem()
	require.NoError(t, err)

	var bb NEOBalance
	require.NoError(t, bb.FromStackItem(si))
	require.Equal(t, b, bb)

	b.VoteTo, err = keys.NewPublicKeyFromString("03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c")
	require.NoError(t, err)
	b.LastGasPerVote = *big.NewInt(100500)

	si, err = b.ToStackItem()
	require.NoError(t, err)
	bb = NEOBalance{}
	require.NoError(t, bb.FromStackItem(si))
	require.Equal(t, b, bb)

	b.VoteTo = nil
	si, err = b.ToStackItem()
	require.NoError(t, err)
	bb = NEOBalance{}
	require.NoError(t, bb.FromStackItem(si))
	require.Equal(t, b, bb)
}

func BenchmarkNEP17BalanceBytes(b *testing.B) {
	var bl NEP17Balance
	bl.Balance.SetInt64(0x12345678910)

	b.Run("stackitem", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_, _ = stackitem.SerializeConvertible(&bl)
		}
	})
	b.Run("bytes", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = bl.Bytes(nil)
		}
	})
	b.Run("bytes, prealloc", func(b *testing.B) {
		bs := bl.Bytes(nil)

		b.ResetTimer()
		b.ReportAllocs()
		for range b.N {
			_ = bl.Bytes(bs[:0])
		}
	})
}

func BenchmarkNEP17BalanceFromBytes(b *testing.B) {
	var bl NEP17Balance
	bl.Balance.SetInt64(0x12345678910)

	buf := bl.Bytes(nil)

	b.Run("stackitem", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = stackitem.DeserializeConvertible(buf, new(NEP17Balance))
		}
	})
	b.Run("from bytes", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_, _ = NEP17BalanceFromBytes(buf)
		}
	})
}
