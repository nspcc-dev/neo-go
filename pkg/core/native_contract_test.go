package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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

func (tn *testNative) OnPersist(ic *interop.Context, _ []stackitem.Item) stackitem.Item {
	if ic.Trigger != trigger.OnPersist {
		panic("invalid trigger")
	}
	select {
	case tn.blocks <- ic.Block.Index:
		return stackitem.NewBool(true)
	default:
		return stackitem.NewBool(false)
	}
}

var _ interop.Contract = (*testNative)(nil)

// registerNative registers native contract in the blockchain.
func (bc *Blockchain) registerNative(c interop.Contract) {
	bc.contracts.Contracts = append(bc.contracts.Contracts, c)
}

const testSumPrice = 1000000

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
		Price:         testSumPrice,
		RequiredFlags: smartcontract.NoneFlag,
	}
	tn.meta.AddMethod(md, desc, true)

	desc = &manifest.Method{
		Name: "callOtherContractWithoutArgs",
		Parameters: []manifest.Parameter{
			manifest.NewParameter("contractHash", smartcontract.Hash160Type),
			manifest.NewParameter("method", smartcontract.StringType),
		},
		ReturnType: smartcontract.AnyType,
	}
	md = &interop.MethodAndPrice{
		Func:          tn.callOtherContractWithoutArgs,
		Price:         testSumPrice,
		RequiredFlags: smartcontract.NoneFlag}
	tn.meta.AddMethod(md, desc, true)

	desc = &manifest.Method{
		Name: "callOtherContractWithArg",
		Parameters: []manifest.Parameter{
			manifest.NewParameter("contractHash", smartcontract.Hash160Type),
			manifest.NewParameter("method", smartcontract.StringType),
			manifest.NewParameter("arg", smartcontract.ArrayType),
		},
		ReturnType: smartcontract.AnyType,
	}
	md = &interop.MethodAndPrice{
		Func:          tn.callOtherContractWithArg,
		Price:         testSumPrice,
		RequiredFlags: smartcontract.NoneFlag}
	tn.meta.AddMethod(md, desc, true)

	desc = &manifest.Method{Name: "onPersist", ReturnType: smartcontract.BoolType}
	md = &interop.MethodAndPrice{Func: tn.OnPersist, RequiredFlags: smartcontract.AllowModifyStates}
	tn.meta.AddMethod(md, desc, false)

	return tn
}

func (tn *testNative) sum(_ *interop.Context, args []stackitem.Item) stackitem.Item {
	s1, err := args[0].TryInteger()
	if err != nil {
		panic(err)
	}
	s2, err := args[1].TryInteger()
	if err != nil {
		panic(err)
	}
	return stackitem.NewBigInteger(s1.Add(s1, s2))
}

func (tn *testNative) callOtherContractWithoutArgs(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	vm := ic.VM
	vm.Estack().PushVal(stackitem.NewArray([]stackitem.Item{})) // no args
	vm.Estack().PushVal(args[1])                                // method
	vm.Estack().PushVal(args[0])                                // contract hash
	err := contract.Call(ic)
	if err != nil {
		return stackitem.NewBigInteger(big.NewInt(-1))
	}
	_ = vm.Run()
	if vm.HasFailed() {
		return stackitem.NewBigInteger(big.NewInt(-2))
	}
	return vm.Estack().Pop().Item()
}

func (tn *testNative) callOtherContractWithArg(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	vm := ic.VM
	vm.Estack().PushVal(stackitem.NewArray([]stackitem.Item{args[2]})) // arg
	vm.Estack().PushVal(args[1])                                       // method
	vm.Estack().PushVal(args[0])                                       // contract hash
	err := contract.Call(ic)
	if err != nil {
		return stackitem.NewBigInteger(big.NewInt(-1))
	}
	_ = vm.Run()
	if vm.HasFailed() {
		return stackitem.NewBigInteger(big.NewInt(-2))
	}
	return vm.Estack().Pop().Item()
}

func TestNativeContract_Invoke(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	tn := newTestNative()
	chain.registerNative(tn)

	err := chain.dao.PutContractState(&state.Contract{
		Script:   tn.meta.Script,
		Manifest: tn.meta.Manifest,
	})
	require.NoError(t, err)

	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, tn.Metadata().Hash, "sum", int64(14), int64(28))
	script := w.Bytes()
	// System.Contract.Call + "sum" itself + opcodes for pushing arguments (PACK is 15000)
	tx := transaction.New(chain.GetConfig().Magic, script, testSumPrice*2+18000)
	validUntil := chain.blockHeight + 1
	tx.ValidUntilBlock = validUntil
	addSigners(tx)
	require.NoError(t, signTx(chain, tx))

	// Enough for Call and other opcodes, but not enough for "sum" call.
	tx2 := transaction.New(chain.GetConfig().Magic, script, testSumPrice*2+8000)
	tx2.ValidUntilBlock = chain.blockHeight + 1
	addSigners(tx2)
	require.NoError(t, signTx(chain, tx2))

	b := chain.newBlock(tx, tx2)
	require.NoError(t, chain.AddBlock(b))

	res, err := chain.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 1, len(res))
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 1, len(res[0].Stack))
	require.Equal(t, big.NewInt(42), res[0].Stack[0].Value())

	res, err = chain.GetAppExecResults(tx2.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 1, len(res))
	require.Equal(t, vm.FaultState, res[0].VMState)

	require.NoError(t, chain.persist())
	select {
	case index := <-tn.blocks:
		require.Equal(t, chain.blockHeight, index)
	default:
		require.Fail(t, "onPersist wasn't called")
	}
}

func TestNativeContract_InvokeInternal(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	tn := newTestNative()
	chain.registerNative(tn)

	err := chain.dao.PutContractState(&state.Contract{
		Script:   tn.meta.Script,
		Manifest: tn.meta.Manifest,
	})
	require.NoError(t, err)

	ic := chain.newInteropContext(trigger.Application,
		dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet), nil, nil)
	v := ic.SpawnVM()

	t.Run("fail, bad current script hash", func(t *testing.T) {
		v.LoadScriptWithHash([]byte{1}, util.Uint160{1, 2, 3}, smartcontract.All)
		v.Estack().PushVal(stackitem.NewArray([]stackitem.Item{stackitem.NewBigInteger(big.NewInt(14)), stackitem.NewBigInteger(big.NewInt(28))}))
		v.Estack().PushVal("sum")
		v.Estack().PushVal(tn.Metadata().Name)

		// it's prohibited to call natives directly
		require.Error(t, native.Call(ic))
	})

	t.Run("success", func(t *testing.T) {
		v.LoadScriptWithHash([]byte{1}, tn.Metadata().Hash, smartcontract.All)
		v.Estack().PushVal(stackitem.NewArray([]stackitem.Item{stackitem.NewBigInteger(big.NewInt(14)), stackitem.NewBigInteger(big.NewInt(28))}))
		v.Estack().PushVal("sum")
		v.Estack().PushVal(tn.Metadata().Name)

		require.NoError(t, native.Call(ic))

		value := v.Estack().Pop().BigInt()
		require.Equal(t, int64(42), value.Int64())
	})
}

func TestNativeContract_InvokeOtherContract(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	tn := newTestNative()
	chain.registerNative(tn)

	err := chain.dao.PutContractState(&state.Contract{
		Script:   tn.meta.Script,
		Manifest: tn.meta.Manifest,
	})
	require.NoError(t, err)

	t.Run("native Policy, getFeePerByte", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.AppCallWithOperationAndArgs(w.BinWriter, tn.Metadata().Hash, "callOtherContractWithoutArgs", chain.contracts.Policy.Hash, "getFeePerByte")
		require.NoError(t, w.Err)
		script := w.Bytes()
		tx := transaction.New(chain.GetConfig().Magic, script, testSumPrice*4+10000)
		validUntil := chain.blockHeight + 1
		tx.ValidUntilBlock = validUntil
		addSigners(tx)
		require.NoError(t, signTx(chain, tx))

		b := chain.newBlock(tx)
		require.NoError(t, chain.AddBlock(b))

		res, err := chain.GetAppExecResults(tx.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 1, len(res))
		// we expect it to be FeePerByte from Policy contract
		require.Equal(t, vm.HaltState, res[0].VMState)
		require.Equal(t, 1, len(res[0].Stack))
		require.Equal(t, big.NewInt(1000), res[0].Stack[0].Value())
	})

	t.Run("native Policy, setFeePerByte", func(t *testing.T) {
		w := io.NewBufBinWriter()
		emit.AppCallWithOperationAndArgs(w.BinWriter, tn.Metadata().Hash, "callOtherContractWithArg", chain.contracts.Policy.Hash, "setFeePerByte", int64(500))
		require.NoError(t, w.Err)
		script := w.Bytes()
		tx := transaction.New(chain.GetConfig().Magic, script, testSumPrice*5+18000)
		validUntil := chain.blockHeight + 1
		tx.ValidUntilBlock = validUntil
		addSigners(tx)
		// to pass policy.checkValidators
		tx.Signers[0].Scopes = transaction.Global
		require.NoError(t, signTx(chain, tx))

		b := chain.newBlock(tx)
		require.NoError(t, chain.AddBlock(b))

		require.NoError(t, chain.persist())

		res, err := chain.GetAppExecResults(tx.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 1, len(res))
		// we expect it to be `true` which means that native policy value was successfully updated
		require.Equal(t, vm.HaltState, res[0].VMState)
		require.Equal(t, 1, len(res[0].Stack))
		require.Equal(t, true, res[0].Stack[0].Value())

		require.NoError(t, chain.persist())

		// check that feePerByte was updated
		n := chain.contracts.Policy.GetFeePerByteInternal(chain.dao)
		require.Equal(t, 500, int(n))
	})

	t.Run("non-native contract", func(t *testing.T) {
		// put some other contract into chain (this contract just pushes `5` on stack)
		avm := []byte{byte(opcode.PUSH5), byte(opcode.RET)}
		contractHash := hash.Hash160(avm)
		m := manifest.NewManifest(contractHash)
		m.ABI.Methods = []manifest.Method{
			{
				Name:       "five",
				Offset:     0,
				Parameters: []manifest.Parameter{},
				ReturnType: smartcontract.IntegerType,
			},
		}

		err = chain.dao.PutContractState(&state.Contract{
			Script:   avm,
			Manifest: *m,
		})
		require.NoError(t, err)

		w := io.NewBufBinWriter()
		emit.AppCallWithOperationAndArgs(w.BinWriter, tn.Metadata().Hash, "callOtherContractWithoutArgs", contractHash, "five")
		require.NoError(t, w.Err)
		script := w.Bytes()
		tx := transaction.New(chain.GetConfig().Magic, script, testSumPrice*4+10000)
		validUntil := chain.blockHeight + 1
		tx.ValidUntilBlock = validUntil
		addSigners(tx)
		require.NoError(t, signTx(chain, tx))

		b := chain.newBlock(tx)
		require.NoError(t, chain.AddBlock(b))

		res, err := chain.GetAppExecResults(tx.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 1, len(res))
		require.Equal(t, vm.HaltState, res[0].VMState)
		require.Equal(t, 1, len(res[0].Stack))
		require.Equal(t, int64(5), res[0].Stack[0].Value().(*big.Int).Int64())
	})
}

func TestAllContractsHaveName(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()
	for _, c := range bc.contracts.Contracts {
		name := c.Metadata().Name
		t.Run(name, func(t *testing.T) {
			w := io.NewBufBinWriter()
			emit.AppCallWithOperationAndArgs(w.BinWriter, c.Metadata().Hash, "name")
			require.NoError(t, w.Err)

			tx := transaction.New(netmode.UnitTestNet, w.Bytes(), 1015570)
			tx.ValidUntilBlock = bc.blockHeight + 1
			addSigners(tx)
			require.NoError(t, signTx(bc, tx))
			require.NoError(t, bc.AddBlock(bc.newBlock(tx)))

			aers, err := bc.GetAppExecResults(tx.Hash(), trigger.Application)
			require.NoError(t, err)
			require.Equal(t, 1, len(aers))
			require.Len(t, aers[0].Stack, 1)
			require.Equal(t, []byte(name), aers[0].Stack[0].Value())
		})
	}
}
