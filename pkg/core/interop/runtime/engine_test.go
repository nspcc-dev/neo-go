package runtime

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

func checkStack(t *testing.T, v *vm.VM, args ...any) {
	require.Equal(t, len(args), v.Estack().Len())
	for i := range args {
		require.Equal(t, stackitem.Make(args[i]), v.Estack().Pop().Item(), "%d", i)
	}
}

func TestGetTrigger(t *testing.T) {
	triggers := []trigger.Type{trigger.Application, trigger.Verification}
	for _, tr := range triggers {
		ic := &interop.Context{Trigger: tr, VM: vm.New()}
		_, err := GetTrigger(ic)
		require.NoError(t, err)
		checkStack(t, ic.VM, int64(tr))
	}
}

func TestPlatform(t *testing.T) {
	ic := &interop.Context{VM: vm.New()}
	_, err := Platform(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, "NEO")
}

func TestGetTime(t *testing.T) {
	b := block.New(false)
	b.Timestamp = 1725021259
	ic := &interop.Context{VM: vm.New(), Block: b}
	_, err := GetTime(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, new(big.Int).SetUint64(1725021259))
}

func TestGetScriptHash(t *testing.T) {
	scripts := []struct {
		s []byte
		h util.Uint160
	}{
		{[]byte{1, 2, 3, 4}, hash.Hash160([]byte{1, 2, 3, 4})},
		{[]byte{1, 2, 3}, util.Uint160{4, 8, 15, 16}},
		{[]byte{1, 2}, hash.Hash160([]byte{1, 2})},
		{[]byte{1}, hash.Hash160([]byte{1})},
	}

	ic := &interop.Context{VM: vm.New()}
	var err error
	ic.VM.LoadScriptWithFlags(scripts[0].s, callflag.All)
	_, err = GetEntryScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	_, err = GetCallingScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, util.Uint160{}.BytesBE())
	_, err = GetExecutingScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[0].h.BytesBE())

	ic.VM.LoadScriptWithHash(scripts[1].s, scripts[1].h, callflag.All)
	_, err = GetEntryScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	_, err = GetCallingScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	_, err = GetExecutingScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[1].h.BytesBE())

	ic.VM.LoadScript(scripts[2].s)
	_, err = GetEntryScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	_, err = GetCallingScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[1].h.BytesBE())
	_, err = GetExecutingScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[2].h.BytesBE())

	ic.VM.LoadScript(scripts[3].s)
	_, err = GetEntryScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	_, err = GetCallingScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[2].h.BytesBE())
	_, err = GetExecutingScriptHash(ic)
	require.NoError(t, err)
	checkStack(t, ic.VM, scripts[3].h.BytesBE())
}

func TestLog(t *testing.T) {
	newL := func(l zapcore.Level) (*zap.Logger, *zaptest.Buffer) {
		enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
		w := &zaptest.Buffer{}
		zc := zapcore.NewCore(enc, w, l)
		return zap.New(zc, zap.ErrorOutput(w)), w
	}
	h := random.Uint160()

	t.Run("big message", func(t *testing.T) {
		ic := &interop.Context{Log: zap.NewNop(), VM: vm.New()}
		ic.VM.LoadScriptWithHash([]byte{1}, h, callflag.All)
		ic.VM.Estack().PushVal(string(make([]byte, MaxNotificationSize+1)))
		_, err := Log(ic)
		require.Error(t, err)
	})

	t.Run("good", func(t *testing.T) {
		log, buf := newL(zapcore.InfoLevel)
		ic := &interop.Context{Log: log, VM: vm.New()}
		ic.VM.LoadScriptWithHash([]byte{1}, h, callflag.All)
		ic.VM.Estack().PushVal("hello")
		_, err := Log(ic)
		require.NoError(t, err)

		ls := buf.Lines()
		require.Equal(t, 1, len(ls))

		var logMsg map[string]any
		require.NoError(t, json.Unmarshal([]byte(ls[0]), &logMsg))
		require.Equal(t, "info", logMsg["level"])
		require.Equal(t, "hello", logMsg["msg"])
		require.Equal(t, h.StringLE(), logMsg["script"])
	})
}

func TestCurrentSigners(t *testing.T) {
	t.Run("container is block", func(t *testing.T) {
		b := block.New(false)
		ic := &interop.Context{VM: vm.New(), Container: b}
		_, err := CurrentSigners(ic)
		require.NoError(t, err)
		checkStack(t, ic.VM, stackitem.Null{})
	})

	t.Run("container is transaction", func(t *testing.T) {
		tx := &transaction.Transaction{
			Signers: []transaction.Signer{
				{
					Account: util.Uint160{1},
					Scopes:  transaction.None,
				},
				{
					Account: util.Uint160{2},
					Scopes:  transaction.CalledByEntry,
				},
			},
		}
		ic := &interop.Context{VM: vm.New(), Container: tx}
		_, err := CurrentSigners(ic)
		require.NoError(t, err)
		checkStack(t, ic.VM, stackitem.NewArray([]stackitem.Item{
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewByteArray(util.Uint160{1}.BytesBE()),
				stackitem.NewBigInteger(big.NewInt(int64(transaction.None))),
				stackitem.NewArray([]stackitem.Item{}),
				stackitem.NewArray([]stackitem.Item{}),
				stackitem.NewArray([]stackitem.Item{}),
			}),
			stackitem.NewArray([]stackitem.Item{
				stackitem.NewByteArray(util.Uint160{2}.BytesBE()),
				stackitem.NewBigInteger(big.NewInt(int64(transaction.CalledByEntry))),
				stackitem.NewArray([]stackitem.Item{}),
				stackitem.NewArray([]stackitem.Item{}),
				stackitem.NewArray([]stackitem.Item{}),
			}),
		}))
	})
}

func TestDeepCopy(t *testing.T) {
	t.Run("Buffer", func(t *testing.T) {
		actual, count := deepCopy(stackitem.NewBuffer([]byte{1, 2, 3}))
		require.Equal(t, stackitem.NewByteArray([]byte{1, 2, 3}), actual)
		require.Equal(t, 1, count)
	})

	t.Run("not deeply copied", func(t *testing.T) {
		for _, item := range []stackitem.Item{
			stackitem.NewBigInteger(big.NewInt(1)),
			stackitem.NewByteArray([]byte{1, 2, 3}),
			stackitem.NewBool(true),
			stackitem.NewPointer(1, []byte{1, 2, 3}),
			stackitem.NewInterop(&[]byte{1, 2}),
		} {
			actual, count := deepCopy(item)
			require.True(t, item == actual)
			require.Equal(t, 1, count)
		}
	})

	t.Run("Null", func(t *testing.T) {
		actual, count := deepCopy(stackitem.Null{})
		require.Equal(t, stackitem.Null{}, actual)
		require.Equal(t, 1, count)
	})

	t.Run("Array", func(t *testing.T) {
		arr := stackitem.NewArray(make([]stackitem.Item, 2))
		items := arr.Value().([]stackitem.Item)
		items[0] = stackitem.NewBool(true)
		items[1] = arr

		actual, count := deepCopy(arr)
		arr.MarkAsReadOnly() // tiny hack for test to be able to compare object references.
		require.Equal(t, arr, actual)
		require.Equal(t, 3, count)
		require.False(t, arr == actual)
		require.True(t, actual == actual.Value().([]stackitem.Item)[1])
	})

	t.Run("Struct", func(t *testing.T) {
		st := stackitem.NewStruct(make([]stackitem.Item, 2))
		items := st.Value().([]stackitem.Item)
		items[0] = stackitem.NewBool(true)
		items[1] = st

		actual, count := deepCopy(st)
		st.MarkAsReadOnly() // tiny hack for test to be able to compare object references.
		require.Equal(t, st, actual)
		require.Equal(t, 3, count)
		require.False(t, st == actual)
		require.True(t, actual == actual.Value().([]stackitem.Item)[1])
	})

	t.Run("Map", func(t *testing.T) {
		m := stackitem.NewMap()
		m.Add(stackitem.NewBool(true), m)
		m.Add(stackitem.NewBigInteger(big.NewInt(1)), stackitem.NewByteArray([]byte{1, 2, 3}))

		actual, count := deepCopy(m)
		m.MarkAsReadOnly() // tiny hack for test to be able to compare object references.
		require.Equal(t, m, actual)
		require.Equal(t, 5, count)
		require.False(t, m == actual)
		require.True(t, actual == actual.Value().([]stackitem.MapElement)[0].Value)
	})
}
