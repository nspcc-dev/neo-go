package vm

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type arrayIterator struct {
	index  int
	values []stackitem.Item
}

func TestCreateCallAndUnwrapIteratorScript(t *testing.T) {
	ctrHash := random.Uint160()
	ctrMethod := "mymethod"
	param := stackitem.NewBigInteger(big.NewInt(42))

	const totalItems = 8
	values := make([]stackitem.Item, totalItems)
	for i := range values {
		values[i] = stackitem.NewBigInteger(big.NewInt(int64(i)))
	}

	checkStack := func(t *testing.T, script []byte, index int, prefetch bool) {
		v := load(script)
		it := &arrayIterator{index: -1, values: values}
		v.SyscallHandler = func(v *VM, id uint32) error {
			switch id {
			case interopnames.ToID([]byte(interopnames.SystemContractCall)):
				require.Equal(t, ctrHash.BytesBE(), v.Estack().Pop().Value())
				require.Equal(t, []byte(ctrMethod), v.Estack().Pop().Value())
				require.Equal(t, big.NewInt(int64(callflag.All)), v.Estack().Pop().Value())
				require.Equal(t, []stackitem.Item{param}, v.Estack().Pop().Value())
				v.Estack().PushItem(stackitem.NewInterop(it))
			case interopnames.ToID([]byte(interopnames.SystemIteratorNext)):
				require.Equal(t, it, v.Estack().Pop().Value())
				it.index++
				v.Estack().PushVal(it.index < len(it.values))
			case interopnames.ToID([]byte(interopnames.SystemIteratorValue)):
				require.Equal(t, it, v.Estack().Pop().Value())
				v.Estack().PushVal(it.values[it.index])
			default:
				return fmt.Errorf("unexpected syscall: %d", id)
			}
			return nil
		}
		require.NoError(t, v.Run())

		if prefetch && index <= len(values) {
			require.Equal(t, 2, v.Estack().Len())

			it, ok := v.Estack().Pop().Interop().Value().(*arrayIterator)
			require.True(t, ok)
			require.Equal(t, index-1, it.index)
			require.Equal(t, values[:index], v.Estack().Pop().Array())
			return
		}
		if len(values) < index {
			index = len(values)
		}
		require.Equal(t, 1, v.Estack().Len())
		require.Equal(t, values[:index], v.Estack().Pop().Array())
	}

	t.Run("truncate", func(t *testing.T) {
		t.Run("zero", func(t *testing.T) {
			const index = 0
			script, err := smartcontract.CreateCallAndUnwrapIteratorScript(ctrHash, ctrMethod, index, param)
			require.NoError(t, err)

			// The behaviour is a bit unexpected, but not a problem (why would anyone fetch 0 items).
			// Let's have test, to make it obvious.
			checkStack(t, script, index+1, false)
		})
		t.Run("all", func(t *testing.T) {
			const index = totalItems + 1
			script, err := smartcontract.CreateCallAndUnwrapIteratorScript(ctrHash, ctrMethod, index, param)
			require.NoError(t, err)

			checkStack(t, script, index, false)
		})
		t.Run("partial", func(t *testing.T) {
			const index = totalItems / 2
			script, err := smartcontract.CreateCallAndUnwrapIteratorScript(ctrHash, ctrMethod, index, param)
			require.NoError(t, err)

			checkStack(t, script, index, false)
		})
	})
	t.Run("prefetch", func(t *testing.T) {
		t.Run("zero", func(t *testing.T) {
			const index = 0
			script, err := smartcontract.CreateCallAndPrefetchIteratorScript(ctrHash, ctrMethod, index, param)
			require.NoError(t, err)

			checkStack(t, script, index+1, true)
		})
		t.Run("all", func(t *testing.T) {
			const index = totalItems + 1 // +1 to test with iterator dropped
			script, err := smartcontract.CreateCallAndPrefetchIteratorScript(ctrHash, ctrMethod, index, param)
			require.NoError(t, err)

			checkStack(t, script, index, true)
		})
		t.Run("partial", func(t *testing.T) {
			const index = totalItems / 2
			script, err := smartcontract.CreateCallAndPrefetchIteratorScript(ctrHash, ctrMethod, index, param)
			require.NoError(t, err)

			checkStack(t, script, index, true)
		})
	})
}
