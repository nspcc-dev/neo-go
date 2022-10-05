package runtime

import (
	"encoding/json"
	"math/big"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
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

func checkStack(t *testing.T, v *vm.VM, args ...interface{}) {
	require.Equal(t, len(args), v.Estack().Len())
	for i := range args {
		require.Equal(t, stackitem.Make(args[i]), v.Estack().Pop().Item(), "%d", i)
	}
}

func TestGetTrigger(t *testing.T) {
	triggers := []trigger.Type{trigger.Application, trigger.Verification}
	for _, tr := range triggers {
		ic := &interop.Context{Trigger: tr, VM: vm.New()}
		require.NoError(t, GetTrigger(ic))
		checkStack(t, ic.VM, int64(tr))
	}
}

func TestPlatform(t *testing.T) {
	ic := &interop.Context{VM: vm.New()}
	require.NoError(t, Platform(ic))
	checkStack(t, ic.VM, "NEO")
}

func TestGetTime(t *testing.T) {
	b := block.New(false)
	b.Timestamp = rand.Uint64()
	ic := &interop.Context{VM: vm.New(), Block: b}
	require.NoError(t, GetTime(ic))
	checkStack(t, ic.VM, new(big.Int).SetUint64(b.Timestamp))
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
	ic.VM.LoadScriptWithFlags(scripts[0].s, callflag.All)
	require.NoError(t, GetEntryScriptHash(ic))
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	require.NoError(t, GetCallingScriptHash(ic))
	checkStack(t, ic.VM, util.Uint160{}.BytesBE())
	require.NoError(t, GetExecutingScriptHash(ic))
	checkStack(t, ic.VM, scripts[0].h.BytesBE())

	ic.VM.LoadScriptWithHash(scripts[1].s, scripts[1].h, callflag.All)
	require.NoError(t, GetEntryScriptHash(ic))
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	require.NoError(t, GetCallingScriptHash(ic))
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	require.NoError(t, GetExecutingScriptHash(ic))
	checkStack(t, ic.VM, scripts[1].h.BytesBE())

	ic.VM.LoadScript(scripts[2].s)
	require.NoError(t, GetEntryScriptHash(ic))
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	require.NoError(t, GetCallingScriptHash(ic))
	checkStack(t, ic.VM, scripts[1].h.BytesBE())
	require.NoError(t, GetExecutingScriptHash(ic))
	checkStack(t, ic.VM, scripts[2].h.BytesBE())

	ic.VM.LoadScript(scripts[3].s)
	require.NoError(t, GetEntryScriptHash(ic))
	checkStack(t, ic.VM, scripts[0].h.BytesBE())
	require.NoError(t, GetCallingScriptHash(ic))
	checkStack(t, ic.VM, scripts[2].h.BytesBE())
	require.NoError(t, GetExecutingScriptHash(ic))
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
		require.Error(t, Log(ic))
	})

	t.Run("good", func(t *testing.T) {
		log, buf := newL(zapcore.InfoLevel)
		ic := &interop.Context{Log: log, VM: vm.New()}
		ic.VM.LoadScriptWithHash([]byte{1}, h, callflag.All)
		ic.VM.Estack().PushVal("hello")
		require.NoError(t, Log(ic))

		ls := buf.Lines()
		require.Equal(t, 1, len(ls))

		var logMsg map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(ls[0]), &logMsg))
		require.Equal(t, "info", logMsg["level"])
		require.Equal(t, "hello", logMsg["msg"])
		require.Equal(t, h.StringLE(), logMsg["script"])
	})
}
