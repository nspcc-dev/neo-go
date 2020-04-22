package core

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/stretchr/testify/require"
)

type testNative struct {
	meta   interop.ContractMD
	blocks chan uint32
}

func (tn *testNative) Initialize(_ *interop.Context) error {
	return nil
}

func (tn *testNative) Metadata() *interop.ContractMD {
	return &tn.meta
}

func (tn *testNative) OnPersist(ic *interop.Context) error {
	select {
	case tn.blocks <- ic.Block.Index:
		return nil
	default:
		return errors.New("error on persist")
	}
}

var _ interop.Contract = (*testNative)(nil)

// registerNative registers native contract in the blockchain.
func (bc *Blockchain) registerNative(c interop.Contract) {
	bc.contracts.Contracts = append(bc.contracts.Contracts, c)
}

func newTestNative() *testNative {
	tn := &testNative{
		meta:   *interop.NewContractMD("Test.Native.Sum"),
		blocks: make(chan uint32, 1),
	}
	desc := &manifest.Method{
		Name: "sum",
		Parameters: []manifest.Parameter{
			manifest.NewParameter("addend1", smartcontract.IntegerType),
			manifest.NewParameter("addend2", smartcontract.IntegerType),
		},
		ReturnType: smartcontract.IntegerType,
	}
	md := &interop.MethodAndPrice{
		Func:          tn.sum,
		Price:         1,
		RequiredFlags: smartcontract.NoneFlag,
	}
	tn.meta.AddMethod(md, desc, true)

	return tn
}

func (tn *testNative) sum(_ *interop.Context, args []vm.StackItem) vm.StackItem {
	s1, err := args[0].TryInteger()
	if err != nil {
		panic(err)
	}
	s2, err := args[1].TryInteger()
	if err != nil {
		panic(err)
	}
	return vm.NewBigIntegerItem(s1.Add(s1, s2))
}

func TestNativeContract_Invoke(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	tn := newTestNative()
	chain.registerNative(tn)

	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, tn.Metadata().Hash, "sum", int64(14), int64(28))
	script := w.Bytes()
	tx := transaction.NewInvocationTX(script, 0)
	mn := transaction.NewMinerTXWithNonce(rand.Uint32())
	validUntil := chain.blockHeight + 1
	tx.ValidUntilBlock = validUntil
	mn.ValidUntilBlock = validUntil
	require.NoError(t, addSender(tx, mn))
	require.NoError(t, signTx(chain, tx, mn))
	b := chain.newBlock(mn, tx)
	require.NoError(t, chain.AddBlock(b))

	res, err := chain.GetAppExecResult(tx.Hash())
	require.NoError(t, err)
	require.Equal(t, "HALT", res.VMState)
	require.Equal(t, 1, len(res.Stack))
	require.Equal(t, smartcontract.IntegerType, res.Stack[0].Type)
	require.EqualValues(t, 42, res.Stack[0].Value)

	require.NoError(t, chain.persist())
	select {
	case index := <-tn.blocks:
		require.Equal(t, chain.blockHeight, index)
	default:
		require.Fail(t, "onPersist wasn't called")
	}
}
