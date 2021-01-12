package enumerator

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

// Enumerator is thoroughly tested in VM package, these are smoke tests.
func TestEnumerator(t *testing.T) {
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
		require.Equal(t, big.NewInt(int64(full[i])), ic.VM.Estack().Pop().BigInt())
	}

	ic.VM.Estack().PushVal(res)
	require.NoError(t, Next(ic))
	require.False(t, ic.VM.Estack().Pop().Bool())
}
