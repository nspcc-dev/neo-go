package interop_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/fakechain"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

const benchGasLimit int64 = 100_000

func benchInteropLoop(b *testing.B, script []byte) {
	d := dao.NewSimple(storage.NewMemoryStore(), false)
	ic := interop.NewContext(trigger.Application, &fakechain.FakeChain{}, d, interop.DefaultBaseExecFee, 0, nil, nil, nil, nil, nil, nil)
	core.SpawnVM(ic)

	for b.Loop() {
		b.StopTimer()
		ic.VM.SetGasLimit(benchGasLimit)
		ic.VM.LoadScript(script)
		b.StartTimer()
		err := ic.VM.Run()
		b.StopTimer()
		require.ErrorContains(b, err, "GAS limit exceeded")
		ic.ReuseVM(ic.VM)
		b.StartTimer()
	}
}

func BenchmarkCreateStandardAccountLoop(b *testing.B) {
	priv, err := keys.NewPrivateKey()
	require.NoError(b, err)
	pub := priv.PublicKey().Bytes()

	w := io.NewBufBinWriter()
	emit.Bytes(w.BinWriter, pub)
	emit.Syscall(w.BinWriter, interopnames.SystemContractCreateStandardAccount)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, w.Bytes())
}

func BenchmarkGetCallFlagsLoop(b *testing.B) {
	w := io.NewBufBinWriter()
	emit.Syscall(w.BinWriter, interopnames.SystemContractGetCallFlags)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	jmpPos := w.Len()
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-jmpPos)})
	require.NoError(b, w.Err)

	benchInteropLoop(b, w.Bytes())
}
