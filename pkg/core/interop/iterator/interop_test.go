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
	ic.VM.Estack().PushVal(res)
	require.NoError(t, vm.EnumeratorNext(ic.VM))
	require.True(t, ic.VM.Estack().Pop().Bool())

	ic.VM.Estack().PushVal(res)
	require.NoError(t, Key(ic))
	require.Equal(t, big.NewInt(0), ic.VM.Estack().Pop().BigInt())

	ic.VM.Estack().PushVal(res)
	require.NoError(t, vm.EnumeratorValue(ic.VM))
	require.Equal(t, big.NewInt(int64(full[0])), ic.VM.Estack().Pop().BigInt())

	ic.VM.Estack().PushVal(res)
	require.NoError(t, vm.EnumeratorNext(ic.VM))
	require.True(t, ic.VM.Estack().Pop().Bool())

	ic.VM.Estack().PushVal(res)
	require.NoError(t, Keys(ic))
	require.NoError(t, vm.EnumeratorValue(ic.VM))
	require.Equal(t, big.NewInt(1), ic.VM.Estack().Pop().BigInt())

	ic.VM.Estack().PushVal(res)
	require.NoError(t, Values(ic))
	require.NoError(t, vm.EnumeratorValue(ic.VM))
	require.Equal(t, big.NewInt(int64(full[1])), ic.VM.Estack().Pop().BigInt())
}
