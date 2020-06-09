package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

// These tests are taken from C# code
// https://github.com/neo-project/neo/blob/master-2.x/neo.UnitTests/UT_InteropPrices.cs#L245
func TestGetPrice(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()
	sdao := dao.NewSimple(storage.NewMemoryStore())
	systemInterop := bc.newInteropContext(trigger.Application, sdao, nil, nil)

	v := SpawnVM(systemInterop)
	v.SetPriceGetter(getPrice)

	t.Run("System.Storage.Put", func(t *testing.T) {
		// System.Storage.Put: e63f1884 (requires push key and value)
		v.Load([]byte{byte(opcode.PUSH3), byte(opcode.PUSH3), byte(opcode.PUSH0),
			byte(opcode.SYSCALL), 0xe6, 0x3f, 0x18, 0x84})
		require.NoError(t, v.StepInto()) // push 03 (length 1)
		require.NoError(t, v.StepInto()) // push 03 (length 1)
		require.NoError(t, v.StepInto()) // push 00

		checkGas(t, util.Fixed8FromInt64(1), v)
	})

	t.Run("System.Storage.PutEx", func(t *testing.T) {
		// System.Storage.PutEx: 73e19b3a (requires push key and value)
		v.Load([]byte{byte(opcode.PUSH3), byte(opcode.PUSH3), byte(opcode.PUSH0),
			byte(opcode.SYSCALL), 0x73, 0xe1, 0x9b, 0x3a})
		require.NoError(t, v.StepInto()) // push 03 (length 1)
		require.NoError(t, v.StepInto()) // push 03 (length 1)
		require.NoError(t, v.StepInto()) // push 00

		checkGas(t, util.Fixed8FromInt64(1), v)
	})
}

func checkGas(t *testing.T, expected util.Fixed8, v *vm.VM) {
	op, par, err := v.Context().Next()

	require.NoError(t, err)
	require.Equal(t, expected, getPrice(v, op, par))
}
