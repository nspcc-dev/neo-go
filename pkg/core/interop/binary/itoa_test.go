package binary

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

func TestItoa(t *testing.T) {
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
		ic := &interop.Context{VM: vm.New()}
		ic.VM.Estack().PushVal(tc.base)
		ic.VM.Estack().PushVal(tc.num)
		require.NoError(t, Itoa(ic))
		require.Equal(t, tc.result, ic.VM.Estack().Pop().String())

		ic = &interop.Context{VM: vm.New()}
		ic.VM.Estack().PushVal(tc.base)
		ic.VM.Estack().PushVal(tc.result)

		require.NoError(t, Atoi(ic))
		require.Equal(t, tc.num, ic.VM.Estack().Pop().BigInt())
	}

	t.Run("-1", func(t *testing.T) {
		for _, s := range []string{"FF", "FFF", "FFFF"} {
			ic := &interop.Context{VM: vm.New()}
			ic.VM.Estack().PushVal(16)
			ic.VM.Estack().PushVal(s)

			require.NoError(t, Atoi(ic))
			require.Equal(t, big.NewInt(-1), ic.VM.Estack().Pop().BigInt())
		}
	})
}

func TestItoaError(t *testing.T) {
	var testCases = []struct {
		num  *big.Int
		base *big.Int
		err  error
	}{
		{big.NewInt(1), big.NewInt(13), ErrInvalidBase},
		{big.NewInt(-1), new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(10)), ErrInvalidBase},
	}

	for _, tc := range testCases {
		ic := &interop.Context{VM: vm.New()}
		ic.VM.Estack().PushVal(tc.base)
		ic.VM.Estack().PushVal(tc.num)
		err := Itoa(ic)
		require.True(t, errors.Is(err, tc.err), "got: %v", err)
	}
}

func TestAtoiError(t *testing.T) {
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
		ic := &interop.Context{VM: vm.New()}
		ic.VM.Estack().PushVal(tc.base)
		ic.VM.Estack().PushVal(tc.num)
		err := Atoi(ic)
		require.True(t, errors.Is(err, tc.err), "got: %v", err)
	}
}
