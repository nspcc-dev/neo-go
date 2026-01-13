package stackitem

import (
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

var (
	bigZero     = big.NewInt(0)
	bigOne      = big.NewInt(1)
	bigMinusOne = big.NewInt(-1)
)

func TestToUint160(t *testing.T) {
	t.Run("not a byte slice", func(t *testing.T) {
		_, err := ToUint160(NewInterop(nil))
		require.ErrorContains(t, err, "invalid conversion: InteropInterface/ByteString")
	})
	t.Run("not a uint160", func(t *testing.T) {
		_, err := ToUint160(NewByteArray([]byte{1, 2, 3}))
		require.ErrorIs(t, err, ErrInvalidValue)
	})
	t.Run("good", func(t *testing.T) {
		expected := util.Uint160{1, 2, 3}
		actual, err := ToUint160(NewByteArray(expected.BytesBE()))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

func TestToUint256(t *testing.T) {
	t.Run("not a byte slice", func(t *testing.T) {
		_, err := ToUint256(NewInterop(nil))
		require.ErrorContains(t, err, "invalid conversion: InteropInterface/ByteString")
	})
	t.Run("not a uint256", func(t *testing.T) {
		_, err := ToUint256(NewByteArray([]byte{1, 2, 3}))
		require.ErrorIs(t, err, ErrInvalidValue)
	})
	t.Run("good", func(t *testing.T) {
		expected := util.Uint256{1, 2, 3}
		actual, err := ToUint256(NewByteArray(expected.BytesBE()))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

func TestToInt32(t *testing.T) {
	t.Run("not an integer", func(t *testing.T) {
		_, err := ToInt32(NewInterop(nil))
		require.ErrorContains(t, err, "invalid conversion: InteropInterface/Integer")
	})
	t.Run("below bounds", func(t *testing.T) {
		_, err := ToInt32(NewBigInteger(big.NewInt(math.MinInt32 - 1)))
		require.ErrorContains(t, err, "bigint is not in int32 range")
	})
	t.Run("above bounds", func(t *testing.T) {
		_, err := ToInt32(NewBigInteger(big.NewInt(math.MaxInt32 + 1)))
		require.ErrorContains(t, err, "bigint is not in int32 range")
	})
	t.Run("good", func(t *testing.T) {
		actual, err := ToInt32(NewBigInteger(big.NewInt(math.MinInt32)))
		require.NoError(t, err)
		require.Equal(t, int32(math.MinInt32), actual)

		actual, err = ToInt32(NewBigInteger(big.NewInt(math.MaxInt32)))
		require.NoError(t, err)
		require.Equal(t, int32(math.MaxInt32), actual)
	})
}

func TestToInt64(t *testing.T) {
	t.Run("not an integer", func(t *testing.T) {
		_, err := ToInt64(NewInterop(nil))
		require.ErrorContains(t, err, "invalid conversion: InteropInterface/Integer")
	})
	t.Run("below bounds", func(t *testing.T) {
		_, err := ToInt64(NewBigInteger(new(big.Int).Sub(big.NewInt(math.MinInt64), bigOne)))
		require.ErrorContains(t, err, "bigint is not in int64 range")
	})
	t.Run("above bounds", func(t *testing.T) {
		_, err := ToInt64(NewBigInteger(new(big.Int).Add(big.NewInt(math.MaxInt64), bigOne)))
		require.ErrorContains(t, err, "bigint is not in int64 range")
	})
	t.Run("good", func(t *testing.T) {
		actual, err := ToInt64(NewBigInteger(big.NewInt(math.MinInt64)))
		require.NoError(t, err)
		require.Equal(t, int64(math.MinInt64), actual)

		actual, err = ToInt64(NewBigInteger(big.NewInt(math.MaxInt64)))
		require.NoError(t, err)
		require.Equal(t, int64(math.MaxInt64), actual)
	})
}

func TestToUint8(t *testing.T) {
	t.Run("not an integer", func(t *testing.T) {
		_, err := ToUint8(NewInterop(nil))
		require.ErrorContains(t, err, "invalid conversion: InteropInterface/Integer")
	})
	t.Run("below bounds", func(t *testing.T) {
		_, err := ToUint8(NewBigInteger(bigMinusOne))
		require.ErrorContains(t, err, "bigint is not in uint8 range")
	})
	t.Run("above bounds", func(t *testing.T) {
		_, err := ToUint8(NewBigInteger(big.NewInt(math.MaxUint8 + 1)))
		require.ErrorContains(t, err, "bigint is not in uint8 range")
	})
	t.Run("good", func(t *testing.T) {
		actual, err := ToUint8(NewBigInteger(bigZero))
		require.NoError(t, err)
		require.Equal(t, uint8(0), actual)

		actual, err = ToUint8(NewBigInteger(big.NewInt(math.MaxUint8)))
		require.NoError(t, err)
		require.Equal(t, uint8(math.MaxUint8), actual)
	})
}

func TestToUint16(t *testing.T) {
	t.Run("not an integer", func(t *testing.T) {
		_, err := ToUint16(NewInterop(nil))
		require.ErrorContains(t, err, "invalid conversion: InteropInterface/Integer")
	})
	t.Run("below bounds", func(t *testing.T) {
		_, err := ToUint16(NewBigInteger(bigMinusOne))
		require.ErrorContains(t, err, "bigint is not in uint16 range")
	})
	t.Run("above bounds", func(t *testing.T) {
		_, err := ToUint16(NewBigInteger(big.NewInt(math.MaxUint16 + 1)))
		require.ErrorContains(t, err, "bigint is not in uint16 range")
	})
	t.Run("good", func(t *testing.T) {
		actual, err := ToUint16(NewBigInteger(bigZero))
		require.NoError(t, err)
		require.Equal(t, uint16(0), actual)

		actual, err = ToUint16(NewBigInteger(big.NewInt(math.MaxUint16)))
		require.NoError(t, err)
		require.Equal(t, uint16(math.MaxUint16), actual)
	})
}

func TestToUint32(t *testing.T) {
	t.Run("not an integer", func(t *testing.T) {
		_, err := ToUint32(NewInterop(nil))
		require.ErrorContains(t, err, "invalid conversion: InteropInterface/Integer")
	})
	t.Run("below bounds", func(t *testing.T) {
		_, err := ToUint32(NewBigInteger(bigMinusOne))
		require.ErrorContains(t, err, "bigint is not in uint32 range")
	})
	t.Run("above bounds", func(t *testing.T) {
		_, err := ToUint32(NewBigInteger(big.NewInt(math.MaxUint32 + 1)))
		require.ErrorContains(t, err, "bigint is not in uint32 range")
	})
	t.Run("good", func(t *testing.T) {
		actual, err := ToUint32(NewBigInteger(bigZero))
		require.NoError(t, err)
		require.Equal(t, uint32(0), actual)

		actual, err = ToUint32(NewBigInteger(big.NewInt(math.MaxUint32)))
		require.NoError(t, err)
		require.Equal(t, uint32(math.MaxUint32), actual)
	})
}

func TestToUint64(t *testing.T) {
	t.Run("not an integer", func(t *testing.T) {
		_, err := ToUint64(NewInterop(nil))
		require.ErrorContains(t, err, "invalid conversion: InteropInterface/Integer")
	})
	t.Run("below bounds", func(t *testing.T) {
		_, err := ToUint64(NewBigInteger(bigMinusOne))
		require.ErrorContains(t, err, "bigint is not in uint64 range")
	})
	t.Run("above bounds", func(t *testing.T) {
		_, err := ToUint64(NewBigInteger(new(big.Int).Add(new(big.Int).SetUint64(math.MaxUint64), bigOne)))
		require.ErrorContains(t, err, "bigint is not in uint64 range")
	})
	t.Run("good", func(t *testing.T) {
		actual, err := ToUint64(NewBigInteger(bigZero))
		require.NoError(t, err)
		require.Equal(t, uint64(0), actual)

		actual, err = ToUint64(NewBigInteger(new(big.Int).SetUint64(math.MaxUint64)))
		require.NoError(t, err)
		require.Equal(t, uint64(math.MaxUint64), actual)
	})
}
