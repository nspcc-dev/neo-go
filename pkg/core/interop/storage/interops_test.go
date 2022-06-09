package storage_test

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/stretchr/testify/require"
)

func TestUnexpectedNonInterops(t *testing.T) {
	vals := map[string]interface{}{
		"int":    1,
		"bool":   false,
		"string": "smth",
		"array":  []int{1, 2, 3},
	}

	// All of these functions expect an interop item on the stack.
	funcs := []func(*interop.Context) error{
		storage.ContextAsReadOnly,
		storage.Delete,
		storage.Find,
		storage.Get,
		storage.Put,
	}
	for _, f := range funcs {
		for k, v := range vals {
			fname := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
			t.Run(k+"/"+fname, func(t *testing.T) {
				vm, ic, _ := createVM(t)
				vm.Estack().PushVal(v)
				require.Error(t, f(ic))
			})
		}
	}
}
