package binary

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestRuntimeSerialize(t *testing.T) {
	t.Run("recursive", func(t *testing.T) {
		arr := stackitem.NewArray(nil)
		arr.Append(arr)
		ic := &interop.Context{VM: vm.New()}
		ic.VM.Estack().PushVal(arr)
		require.Error(t, Serialize(ic))
	})
	t.Run("big item", func(t *testing.T) {
		ic := &interop.Context{VM: vm.New()}
		ic.VM.Estack().PushVal(make([]byte, stackitem.MaxSize))
		require.Error(t, Serialize(ic))
	})
	t.Run("good", func(t *testing.T) {
		ic := &interop.Context{VM: vm.New()}
		ic.VM.Estack().PushVal(42)
		require.NoError(t, Serialize(ic))

		w := io.NewBufBinWriter()
		stackitem.EncodeBinaryStackItem(stackitem.Make(42), w.BinWriter)
		require.NoError(t, w.Err)

		encoded := w.Bytes()
		require.Equal(t, encoded, ic.VM.Estack().Pop().Bytes())

		ic.VM.Estack().PushVal(encoded)
		require.NoError(t, Deserialize(ic))
		require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().BigInt())

		t.Run("bad", func(t *testing.T) {
			encoded[0] ^= 0xFF
			ic.VM.Estack().PushVal(encoded)
			require.Error(t, Deserialize(ic))
		})
	})
}
