package compiler_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

func TestPanic(t *testing.T) {
	t.Run("no panic", func(t *testing.T) {
		src := getPanicSource(false, `"execution fault"`)
		eval(t, src, big.NewInt(7))
	})

	t.Run("panic with message", func(t *testing.T) {
		var logs []string
		src := getPanicSource(true, `"execution fault"`)
		v := vmAndCompile(t, src)
		v.RegisterInteropGetter(logGetter(&logs))

		require.Error(t, v.Run())
		require.True(t, v.HasFailed())
		require.Equal(t, 1, len(logs))
		require.Equal(t, "execution fault", logs[0])
	})

	t.Run("panic with nil", func(t *testing.T) {
		var logs []string
		src := getPanicSource(true, `nil`)
		v := vmAndCompile(t, src)
		v.RegisterInteropGetter(logGetter(&logs))

		require.Error(t, v.Run())
		require.True(t, v.HasFailed())
		require.Equal(t, 0, len(logs))
	})
}

func getPanicSource(need bool, message string) string {
	return fmt.Sprintf(`
	package main
	func Main() int {
		needPanic := %#v
		if needPanic {
			panic(%s)
			return 5
		}
		return 7
	}
	`, need, message)
}

func logGetter(logs *[]string) vm.InteropGetterFunc {
	logID := vm.InteropNameToID([]byte("Neo.Runtime.Log"))
	return func(id uint32) *vm.InteropFuncPrice {
		if id != logID {
			return nil
		}

		return &vm.InteropFuncPrice{
			Func: func(v *vm.VM) error {
				msg := string(v.Estack().Pop().Bytes())
				*logs = append(*logs, msg)
				return nil
			},
		}
	}
}
