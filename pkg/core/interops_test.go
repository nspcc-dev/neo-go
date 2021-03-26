package core

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

func testNonInterop(t *testing.T, value interface{}, f func(*interop.Context) error) {
	v := vm.New()
	v.Estack().PushVal(value)
	chain := newTestChain(t)
	d := dao.NewSimple(storage.NewMemoryStore(), chain.config.StateRootInHeader)
	context := chain.newInteropContext(trigger.Application, d, nil, nil)
	context.VM = v
	require.Error(t, f(context))
}

func TestUnexpectedNonInterops(t *testing.T) {
	vals := map[string]interface{}{
		"int":    1,
		"bool":   false,
		"string": "smth",
		"array":  []int{1, 2, 3},
	}

	// All of these functions expect an interop item on the stack.
	funcs := []func(*interop.Context) error{
		storageContextAsReadOnly,
		storageDelete,
		storageFind,
		storageGet,
		storagePut,
	}
	for _, f := range funcs {
		for k, v := range vals {
			fname := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
			t.Run(k+"/"+fname, func(t *testing.T) {
				testNonInterop(t, v, f)
			})
		}
	}
}
