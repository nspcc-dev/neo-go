package json

import (
	"encoding/binary"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

var (
	serializeID   = emit.InteropNameToID([]byte("System.Json.Serialize"))
	deserializeID = emit.InteropNameToID([]byte("System.Json.Deserialize"))
)

func getInterop(id uint32) *vm.InteropFuncPrice {
	switch id {
	case serializeID:
		return &vm.InteropFuncPrice{
			Func: func(v *vm.VM) error { return Serialize(nil, v) },
		}
	case deserializeID:
		return &vm.InteropFuncPrice{
			Func: func(v *vm.VM) error { return Deserialize(nil, v) },
		}
	default:
		return nil
	}
}

func getTestFunc(id uint32, arg interface{}, result interface{}) func(t *testing.T) {
	prog := make([]byte, 5)
	prog[0] = byte(opcode.SYSCALL)
	binary.LittleEndian.PutUint32(prog[1:], id)

	return func(t *testing.T) {
		v := vm.New()
		v.RegisterInteropGetter(getInterop)
		v.LoadScript(prog)
		v.Estack().PushVal(arg)
		if result == nil {
			require.Error(t, v.Run())
			return
		}
		require.NoError(t, v.Run())
		require.Equal(t, stackitem.Make(result), v.Estack().Pop().Item())
	}
}

func TestSerialize(t *testing.T) {
	t.Run("Serialize", func(t *testing.T) {
		t.Run("Good", getTestFunc(serializeID, 42, []byte("42")))
		t.Run("Bad", func(t *testing.T) {
			arr := stackitem.NewArray([]stackitem.Item{
				stackitem.NewByteArray(make([]byte, stackitem.MaxSize/2)),
				stackitem.NewByteArray(make([]byte, stackitem.MaxSize/2)),
			})
			getTestFunc(serializeID, arr, nil)(t)
		})
	})
	t.Run("Deserialize", func(t *testing.T) {
		t.Run("Good", getTestFunc(deserializeID, []byte("42"), 42))
		t.Run("Bad", getTestFunc(deserializeID, []byte("{]"), nil))
	})
}
