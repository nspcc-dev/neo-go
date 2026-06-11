package runtime_test

import (
	"math"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

func TestRuntime_BurnGas(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	script := io.NewBufBinWriter()
	emit.Int(script.BinWriter, math.MaxInt64)
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeBurnGas)

	tx := transaction.New(script.Bytes(), 0)
	tx.Nonce = neotest.Nonce()
	tx.ValidUntilBlock = e.Chain.BlockHeight() + 1
	e.SignTx(t, tx, 1_0000_0000, acc)
	e.AddNewBlock(t, tx)
	e.CheckFault(t, tx.Hash(), "System.Runtime.BurnGas failed: GAS limit exceeded")
}
