package fee_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const benchGasLimit int64 = 50_0000_0000

func benchOpcodeLoop(b *testing.B, script []byte) {
	bc, _ := chain.NewSingleWithOptions(b, &chain.Options{
		BlockchainConfigHook: func(c *config.Blockchain) {
			c.Hardforks = map[string]uint32{config.HFLatestKnown.String(): 0}
		},
		Logger: zap.NewNop(),
	})
	ic, err := bc.GetTestVM(trigger.Application, &transaction.Transaction{}, nil)
	require.NoError(b, err)

	for b.Loop() {
		b.StopTimer()
		ic.VM.SetGasLimit(benchGasLimit)
		ic.VM.LoadScriptWithFlags(script, callflag.All)
		b.StartTimer()
		err := ic.VM.Run()
		b.StopTimer()
		require.ErrorContains(b, err, "GAS limit exceeded")
		ic.Finalize()
		ic.ReuseVM(ic.VM)
		b.StartTimer()
	}
}

func BenchmarkNOP(b *testing.B) {
	numberNops := 100
	w := io.NewBufBinWriter()
	for range numberNops {
		emit.Opcodes(w.BinWriter, opcode.NOP)
	}
	emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-w.Len())})
	require.NoError(b, w.Err)

	benchOpcodeLoop(b, w.Bytes())
}

func BenchmarkMODPOW(b *testing.B) {
	getScript := func(base, exp, mod *big.Int) []byte {
		w := io.NewBufBinWriter()
		emit.BigInt(w.BinWriter, base)
		emit.BigInt(w.BinWriter, exp)
		emit.BigInt(w.BinWriter, mod)
		emit.Opcodes(w.BinWriter, opcode.MODPOW, opcode.DROP)
		emit.Instruction(w.BinWriter, opcode.JMP, []byte{byte(-w.Len())})
		require.NoError(b, w.Err)
		return w.Bytes()
	}

	b.Run("best", func(b *testing.B) {
		benchOpcodeLoop(b, getScript(big.NewInt(2), big.NewInt(2), big.NewInt(3)))
	})
	b.Run("worst", func(b *testing.B) {
		maxInt := new(big.Int).Lsh(big.NewInt(1), 255)
		maxInt.Sub(maxInt, big.NewInt(1))
		// Base must not be equal to the modulus, otherwise the result is trivially 0.
		base := new(big.Int).Exp(big.NewInt(3), big.NewInt(160), nil)
		benchOpcodeLoop(b, getScript(base, maxInt, maxInt))
	})
}
