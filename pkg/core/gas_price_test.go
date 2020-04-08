package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
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

	t.Run("Neo.Asset.Create", func(t *testing.T) {
		// Neo.Asset.Create: 83c5c61f
		v.Load([]byte{0x68, 0x04, 0x83, 0xc5, 0xc6, 0x1f})
		checkGas(t, util.Fixed8FromInt64(5000), v)
	})

	t.Run("Neo.Asset.Renew", func(t *testing.T) {
		// Neo.Asset.Renew: 78849071 (requires push 09 push 09 before)
		v.Load([]byte{0x59, 0x59, 0x68, 0x04, 0x78, 0x84, 0x90, 0x71})
		require.NoError(t, v.StepInto()) // push 9
		require.NoError(t, v.StepInto()) // push 9

		checkGas(t, util.Fixed8FromInt64(9*5000), v)
	})

	t.Run("Neo.Contract.Create (no props)", func(t *testing.T) {
		// Neo.Contract.Create: f66ca56e (requires push properties on fourth position)
		v.Load([]byte{0x00, 0x00, 0x00, 0x00, 0x68, 0x04, 0xf6, 0x6c, 0xa5, 0x6e})
		require.NoError(t, v.StepInto()) // push 0 - ContractPropertyState.NoProperty
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0

		checkGas(t, util.Fixed8FromInt64(100), v)
	})

	t.Run("Neo.Contract.Create (has storage)", func(t *testing.T) {
		// Neo.Contract.Create: f66ca56e (requires push properties on fourth position)
		v.Load([]byte{0x51, 0x00, 0x00, 0x00, 0x68, 0x04, 0xf6, 0x6c, 0xa5, 0x6e})
		require.NoError(t, v.StepInto()) // push 01 - ContractPropertyState.HasStorage
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0

		checkGas(t, util.Fixed8FromInt64(500), v)
	})

	t.Run("Neo.Contract.Create (has dynamic invoke)", func(t *testing.T) {
		// Neo.Contract.Create: f66ca56e (requires push properties on fourth position)
		v.Load([]byte{0x52, 0x00, 0x00, 0x00, 0x68, 0x04, 0xf6, 0x6c, 0xa5, 0x6e})
		require.NoError(t, v.StepInto()) // push 02 - ContractPropertyState.HasDynamicInvoke
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0

		checkGas(t, util.Fixed8FromInt64(600), v)
	})

	t.Run("Neo.Contract.Create (has both storage and dynamic invoke)", func(t *testing.T) {
		// Neo.Contract.Create: f66ca56e (requires push properties on fourth position)
		v.Load([]byte{0x53, 0x00, 0x00, 0x00, 0x68, 0x04, 0xf6, 0x6c, 0xa5, 0x6e})
		require.NoError(t, v.StepInto()) // push 03 - HasStorage and HasDynamicInvoke
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0

		checkGas(t, util.Fixed8FromInt64(1000), v)
	})

	t.Run("Neo.Contract.Migrate", func(t *testing.T) {
		// Neo.Contract.Migrate: 471b6290 (requires push properties on fourth position)
		v.Load([]byte{0x00, 0x00, 0x00, 0x00, 0x68, 0x04, 0x47, 0x1b, 0x62, 0x90})
		require.NoError(t, v.StepInto()) // push 0 - ContractPropertyState.NoProperty
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0
		require.NoError(t, v.StepInto()) // push 0

		checkGas(t, util.Fixed8FromInt64(100), v)
	})

	t.Run("System.Storage.Put", func(t *testing.T) {
		// System.Storage.Put: e63f1884 (requires push key and value)
		v.Load([]byte{0x53, 0x53, 0x00, 0x68, 0x04, 0xe6, 0x3f, 0x18, 0x84})
		require.NoError(t, v.StepInto()) // push 03 (length 1)
		require.NoError(t, v.StepInto()) // push 03 (length 1)
		require.NoError(t, v.StepInto()) // push 00

		checkGas(t, util.Fixed8FromInt64(1), v)
	})

	t.Run("System.Storage.PutEx", func(t *testing.T) {
		// System.Storage.PutEx: 73e19b3a (requires push key and value)
		v.Load([]byte{0x53, 0x53, 0x00, 0x68, 0x04, 0x73, 0xe1, 0x9b, 0x3a})
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
