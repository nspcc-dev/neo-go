package iterator

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

// Iterator is thoroughly tested in VM package, these are smoke tests.
func TestIterator(t *testing.T) {
	ic := &interop.Context{VM: vm.New()}
	full := []byte{4, 8, 15}
	ic.VM.Estack().PushVal(full)
	require.NoError(t, Create(ic))

	res := ic.VM.Estack().Pop().Item()
	for i := range full {
		ic.VM.Estack().PushVal(res)
		require.NoError(t, Next(ic))
		require.True(t, ic.VM.Estack().Pop().Bool())

		ic.VM.Estack().PushVal(res)
		require.NoError(t, Value(ic))

		value := ic.VM.Estack().Pop().Item().Value()
		require.Equal(t, big.NewInt(int64(full[i])), value)
	}

	ic.VM.Estack().PushVal(res)
	require.NoError(t, Next(ic))
	require.False(t, false, ic.VM.Estack().Pop().Bool())
}
