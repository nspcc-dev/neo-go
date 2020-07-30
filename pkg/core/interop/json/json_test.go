package json

import (
	"encoding/binary"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

var (
	serializeID   = emit.InteropNameToID([]byte("System.Json.Serialize"))
	deserializeID = emit.InteropNameToID([]byte("System.Json.Deserialize"))
)

var jsonInterops = []interop.Function{
	{ID: serializeID, Func: Serialize},
	{ID: deserializeID, Func: Deserialize},
}

func init() {
	interop.Sort(jsonInterops)
}

func getTestFunc(id uint32, arg interface{}, result interface{}) func(t *testing.T) {
	prog := make([]byte, 5)
	prog[0] = byte(opcode.SYSCALL)
	binary.LittleEndian.PutUint32(prog[1:], id)

	return func(t *testing.T) {
		ic := &interop.Context{}
		ic.Functions = append(ic.Functions, jsonInterops)
		v := ic.SpawnVM()
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
