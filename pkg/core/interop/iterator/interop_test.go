package iterator

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type testIter struct {
	index int
	arr   []int
}

func (t *testIter) Next() bool {
	if t.index < len(t.arr) {
		t.index++
	}
	return t.index < len(t.arr)
}

func (t testIter) Value() stackitem.Item {
	return stackitem.NewBigIntegerFromInt64(int64(t.arr[t.index]))
}

// Iterator is thoroughly tested in VM package, these are smoke tests.
func TestIterator(t *testing.T) {
	ic := &interop.Context{VM: vm.New()}
	full := []int{4, 8, 15}
	ic.VM.Estack().PushVal(stackitem.NewInterop(&testIter{index: -1, arr: full}))

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
