package runtime

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestGasLeft(t *testing.T) {
	t.Run("no limit", func(t *testing.T) {
		ic := &interop.Context{VM: vm.New()}
		ic.VM.GasLimit = -1
		ic.VM.AddGas(58)
		require.NoError(t, GasLeft(ic))
		checkStack(t, ic.VM, -1)
	})
	t.Run("with limit", func(t *testing.T) {
		ic := &interop.Context{VM: vm.New()}
		ic.VM.GasLimit = 100
		ic.VM.AddGas(58)
		require.NoError(t, GasLeft(ic))
		checkStack(t, ic.VM, 42)
	})
}

func TestRuntimeGetNotifications(t *testing.T) {
	v := vm.New()
	ic := &interop.Context{
		VM: v,
		Notifications: []state.NotificationEvent{
			{ScriptHash: util.Uint160{1}, Name: "Event1", Item: stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{11})})},
			{ScriptHash: util.Uint160{2}, Name: "Event2", Item: stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{22})})},
			{ScriptHash: util.Uint160{1}, Name: "Event1", Item: stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{33})})},
		},
	}

	t.Run("NoFilter", func(t *testing.T) {
		v.Estack().PushVal(stackitem.Null{})
		require.NoError(t, GetNotifications(ic))

		arr := v.Estack().Pop().Array()
		require.Equal(t, len(ic.Notifications), len(arr))
		for i := range arr {
			elem := arr[i].Value().([]stackitem.Item)
			require.Equal(t, ic.Notifications[i].ScriptHash.BytesBE(), elem[0].Value())
			name, err := stackitem.ToString(elem[1])
			require.NoError(t, err)
			require.Equal(t, ic.Notifications[i].Name, name)
			ic.Notifications[i].Item.MarkAsReadOnly() // tiny hack for test to be able to compare object references.
			require.Equal(t, ic.Notifications[i].Item, elem[2])
		}
	})

	t.Run("WithFilter", func(t *testing.T) {
		h := util.Uint160{2}.BytesBE()
		v.Estack().PushVal(h)
		require.NoError(t, GetNotifications(ic))

		arr := v.Estack().Pop().Array()
		require.Equal(t, 1, len(arr))
		elem := arr[0].Value().([]stackitem.Item)
		require.Equal(t, h, elem[0].Value())
		name, err := stackitem.ToString(elem[1])
		require.NoError(t, err)
		require.Equal(t, ic.Notifications[1].Name, name)
		require.Equal(t, ic.Notifications[1].Item, elem[2])
	})

	t.Run("Bad", func(t *testing.T) {
		t.Run("not bytes", func(t *testing.T) {
			v.Estack().PushVal(stackitem.NewInterop(util.Uint160{1}))
			require.Error(t, GetNotifications(ic))
		})
		t.Run("not uint160", func(t *testing.T) {
			v.Estack().PushVal([]byte{1, 2, 3})
			require.Error(t, GetNotifications(ic))
		})
		t.Run("too many notifications", func(t *testing.T) {
			for range vm.MaxStackSize + 1 {
				ic.Notifications = append(ic.Notifications, state.NotificationEvent{
					ScriptHash: util.Uint160{3},
					Name:       "Event3",
					Item:       stackitem.NewArray(nil),
				})
			}
			v.Estack().PushVal(stackitem.Null{})
			require.Error(t, GetNotifications(ic))
		})
	})
}

func TestRuntimeGetInvocationCounter(t *testing.T) {
	ic := &interop.Context{VM: vm.New(), Invocations: make(map[util.Uint160]int)}
	h := random.Uint160()
	ic.Invocations[h] = 42

	t.Run("No invocations", func(t *testing.T) {
		h1 := h
		h1[0] ^= 0xFF
		ic.VM.LoadScriptWithHash([]byte{1}, h1, callflag.NoneFlag)
		// do not return an error in this case.
		require.NoError(t, GetInvocationCounter(ic))
		checkStack(t, ic.VM, 1)
	})
	t.Run("NonZero", func(t *testing.T) {
		ic.VM.LoadScriptWithHash([]byte{1}, h, callflag.NoneFlag)
		require.NoError(t, GetInvocationCounter(ic))
		checkStack(t, ic.VM, 42)
	})
}

// Test compatibility with C# implementation.
// https://github.com/neo-project/neo/blob/master/tests/neo.UnitTests/Cryptography/UT_Murmur128.cs
func TestMurmurCompat(t *testing.T) {
	res := murmur128([]byte("hello"), 123)
	require.Equal(t, "0bc59d0ad25fde2982ed65af61227a0e", hex.EncodeToString(res))

	res = murmur128([]byte("world"), 123)
	require.Equal(t, "3d3810fed480472bd214a14023bb407f", hex.EncodeToString(res))

	res = murmur128([]byte("hello world"), 123)
	require.Equal(t, "e0a0632d4f51302c55e3b3e48d28795d", hex.EncodeToString(res))

	bs, _ := hex.DecodeString("718f952132679baa9c5c2aa0d329fd2a")
	res = murmur128(bs, 123)
	require.Equal(t, "9b4aa747ff0cf4e41b3d96251551c8ae", hex.EncodeToString(res))
}
