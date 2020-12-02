package binary

import (
	"encoding/base64"
	"math/big"
	"testing"

	"github.com/mr-tron/base58"
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

func TestRuntimeEncodeDecode(t *testing.T) {
	original := []byte("my pretty string")
	encoded64 := base64.StdEncoding.EncodeToString(original)
	encoded58 := base58.Encode(original)
	v := vm.New()
	ic := &interop.Context{VM: v}

	t.Run("Encode64", func(t *testing.T) {
		v.Estack().PushVal(original)
		require.NoError(t, EncodeBase64(ic))
		actual := v.Estack().Pop().Bytes()
		require.Equal(t, []byte(encoded64), actual)
	})
	t.Run("Encode58", func(t *testing.T) {
		v.Estack().PushVal(original)
		require.NoError(t, EncodeBase58(ic))
		actual := v.Estack().Pop().Bytes()
		require.Equal(t, []byte(encoded58), actual)
	})
	t.Run("Decode64/positive", func(t *testing.T) {
		v.Estack().PushVal(encoded64)
		require.NoError(t, DecodeBase64(ic))
		actual := v.Estack().Pop().Bytes()
		require.Equal(t, original, actual)
	})
	t.Run("Decode64/error", func(t *testing.T) {
		v.Estack().PushVal(encoded64 + "%")
		require.Error(t, DecodeBase64(ic))
	})
	t.Run("Decode58/positive", func(t *testing.T) {
		v.Estack().PushVal(encoded58)
		require.NoError(t, DecodeBase58(ic))
		actual := v.Estack().Pop().Bytes()
		require.Equal(t, original, actual)
	})
	t.Run("Decode58/error", func(t *testing.T) {
		v.Estack().PushVal(encoded58 + "%")
		require.Error(t, DecodeBase58(ic))
	})
}
