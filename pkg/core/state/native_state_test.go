package state

import (
	"testing"

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
}

func BenchmarkNEP17BalanceBytes(b *testing.B) {
	var bl NEP17Balance
	bl.Balance.SetInt64(0x12345678910)

	b.Run("stackitem", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = stackitem.SerializeConvertible(&bl)
		}
	})
	b.Run("bytes", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bl.Bytes(nil)
		}
	})
	b.Run("bytes, prealloc", func(b *testing.B) {
		bs := bl.Bytes(nil)

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = bl.Bytes(bs[:0])
		}
	})
}
