package native

import (
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestStdLibItoaAtoi(t *testing.T) {
	s := newStd()
	ic := &interop.Context{VM: vm.New()}
	var actual stackitem.Item

	t.Run("itoa-atoi", func(t *testing.T) {
		var testCases = []struct {
			num    *big.Int
			base   *big.Int
			result string
		}{
			{big.NewInt(0), big.NewInt(10), "0"},
			{big.NewInt(0), big.NewInt(16), "0"},
			{big.NewInt(1), big.NewInt(10), "1"},
			{big.NewInt(-1), big.NewInt(10), "-1"},
			{big.NewInt(1), big.NewInt(16), "1"},
			{big.NewInt(7), big.NewInt(16), "7"},
			{big.NewInt(8), big.NewInt(16), "08"},
			{big.NewInt(65535), big.NewInt(16), "0FFFF"},
			{big.NewInt(15), big.NewInt(16), "0F"},
			{big.NewInt(-1), big.NewInt(16), "F"},
		}

		for _, tc := range testCases {
			require.NotPanics(t, func() {
				actual = s.itoa(ic, []stackitem.Item{stackitem.Make(tc.num), stackitem.Make(tc.base)})
			})
			require.Equal(t, stackitem.Make(tc.result), actual)

			require.NotPanics(t, func() {
				actual = s.atoi(ic, []stackitem.Item{stackitem.Make(tc.result), stackitem.Make(tc.base)})
			})
			require.Equal(t, stackitem.Make(tc.num), actual)
		}

		t.Run("-1", func(t *testing.T) {
			for _, str := range []string{"FF", "FFF", "FFFF"} {
				require.NotPanics(t, func() {
					actual = s.atoi(ic, []stackitem.Item{stackitem.Make(str), stackitem.Make(16)})
				})

				require.Equal(t, stackitem.Make(-1), actual)
			}
		})
	})

	t.Run("itoa error", func(t *testing.T) {
		var testCases = []struct {
			num  *big.Int
			base *big.Int
			err  error
		}{
			{big.NewInt(1), big.NewInt(13), ErrInvalidBase},
			{big.NewInt(-1), new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(10)), ErrInvalidBase},
		}

		for _, tc := range testCases {
			require.PanicsWithError(t, tc.err.Error(), func() {
				_ = s.itoa(ic, []stackitem.Item{stackitem.Make(tc.num), stackitem.Make(tc.base)})
			})
		}
	})

	t.Run("atoi error", func(t *testing.T) {
		var testCases = []struct {
			num  string
			base *big.Int
			err  error
		}{
			{"1", big.NewInt(13), ErrInvalidBase},
			{"1", new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(16)), ErrInvalidBase},
			{"1_000", big.NewInt(10), ErrInvalidFormat},
			{"FE", big.NewInt(10), ErrInvalidFormat},
			{"XD", big.NewInt(16), ErrInvalidFormat},
		}

		for _, tc := range testCases {
			require.PanicsWithError(t, tc.err.Error(), func() {
				_ = s.atoi(ic, []stackitem.Item{stackitem.Make(tc.num), stackitem.Make(tc.base)})
			})
		}
	})
}

func TestStdLibJSON(t *testing.T) {
	s := newStd()
	ic := &interop.Context{VM: vm.New()}
	var actual stackitem.Item

	t.Run("JSONSerialize", func(t *testing.T) {
		t.Run("Good", func(t *testing.T) {
			require.NotPanics(t, func() {
				actual = s.jsonSerialize(ic, []stackitem.Item{stackitem.Make(42)})
			})

			require.Equal(t, stackitem.Make([]byte("42")), actual)
		})

		t.Run("Bad", func(t *testing.T) {
			arr := stackitem.NewArray([]stackitem.Item{
				stackitem.NewByteArray(make([]byte, stackitem.MaxSize/2)),
				stackitem.NewByteArray(make([]byte, stackitem.MaxSize/2)),
			})
			require.Panics(t, func() {
				_ = s.jsonSerialize(ic, []stackitem.Item{arr})
			})
		})
	})

	t.Run("JSONDeserialize", func(t *testing.T) {
		t.Run("Good", func(t *testing.T) {
			require.NotPanics(t, func() {
				actual = s.jsonDeserialize(ic, []stackitem.Item{stackitem.Make("42")})
			})

			require.Equal(t, stackitem.Make(42), actual)
		})
		t.Run("Bad", func(t *testing.T) {
			require.Panics(t, func() {
				_ = s.jsonDeserialize(ic, []stackitem.Item{stackitem.Make("{]")})
			})
			require.Panics(t, func() {
				_ = s.jsonDeserialize(ic, []stackitem.Item{stackitem.NewInterop(nil)})
			})
		})
	})
}
