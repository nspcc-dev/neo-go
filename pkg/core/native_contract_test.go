package core

import (
	"errors"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
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

func (tn *testNative) OnPersist(ic *interop.Context) error {
	select {
	case tn.blocks <- ic.Block.Index:
		return nil
	default:
		return errors.New("can't send index")
	}
}

func (tn *testNative) PostPersist(ic *interop.Context) error {
	return nil
}

var _ interop.Contract = (*testNative)(nil)

// registerNative registers native contract in the blockchain.
func (bc *Blockchain) registerNative(c interop.Contract) {
	bc.contracts.Contracts = append(bc.contracts.Contracts, c)
}

const testSumPrice = 1 << 15 * interop.DefaultBaseExecFee // same as contract.Call

func newTestNative() *testNative {
	tn := &testNative{
		meta:   *interop.NewContractMD("Test.Native.Sum", 0),
		blocks: make(chan uint32, 1),
	}
	desc := &manifest.Method{
		Name: "sum",
		Parameters: []manifest.Parameter{
			manifest.NewParameter("addend1", smartcontract.IntegerType),
			manifest.NewParameter("addend2", smartcontract.IntegerType),
		},
		ReturnType: smartcontract.IntegerType,
		Safe:       true,
	}
	md := &interop.MethodAndPrice{
		Func:          tn.sum,
		Price:         testSumPrice,
		RequiredFlags: callflag.NoneFlag,
	}
	tn.meta.AddMethod(md, desc)

	desc = &manifest.Method{
		Name: "callOtherContractNoReturn",
		Parameters: []manifest.Parameter{
			manifest.NewParameter("contractHash", smartcontract.Hash160Type),
			manifest.NewParameter("method", smartcontract.StringType),
			manifest.NewParameter("arg", smartcontract.ArrayType),
		},
		ReturnType: smartcontract.VoidType,
		Safe:       true,
	}
	md = &interop.MethodAndPrice{
		Func:          tn.callOtherContractNoReturn,
		Price:         testSumPrice,
		RequiredFlags: callflag.NoneFlag}
	tn.meta.AddMethod(md, desc)

	desc = &manifest.Method{
		Name: "callOtherContractWithReturn",
		Parameters: []manifest.Parameter{
			manifest.NewParameter("contractHash", smartcontract.Hash160Type),
			manifest.NewParameter("method", smartcontract.StringType),
			manifest.NewParameter("arg", smartcontract.ArrayType),
		},
		ReturnType: smartcontract.IntegerType,
	}
	md = &interop.MethodAndPrice{
		Func:          tn.callOtherContractWithReturn,
		Price:         testSumPrice,
		RequiredFlags: callflag.NoneFlag}
	tn.meta.AddMethod(md, desc)

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

func toUint160(item stackitem.Item) util.Uint160 {
	bs, err := item.TryBytes()
	if err != nil {
		panic(err)
	}
	u, err := util.Uint160DecodeBytesBE(bs)
	if err != nil {
		panic(err)
	}
	return u
}

func (tn *testNative) call(ic *interop.Context, args []stackitem.Item, hasReturn bool) {
	cs, err := ic.GetContract(toUint160(args[0]))
	if err != nil {
		panic(err)
	}
	bs, err := args[1].TryBytes()
	if err != nil {
		panic(err)
	}
	err = contract.CallFromNative(ic, tn.meta.Hash, cs, string(bs), args[2].Value().([]stackitem.Item), hasReturn)
	if err != nil {
		panic(err)
	}
}

func (tn *testNative) callOtherContractNoReturn(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	tn.call(ic, args, false)
	return stackitem.Null{}
}

func (tn *testNative) callOtherContractWithReturn(ic *interop.Context, args []stackitem.Item) stackitem.Item {
	tn.call(ic, args, true)
	bi := ic.VM.Estack().Pop().BigInt()
	return stackitem.Make(bi.Add(bi, big.NewInt(1)))
}

func TestNativeContract_Invoke(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	tn := newTestNative()
	chain.registerNative(tn)

	err := chain.contracts.Management.PutContractState(chain.dao, &state.Contract{
		ContractBase: state.ContractBase{
			ID:       1,
			NEF:      tn.meta.NEF,
			Hash:     tn.meta.Hash,
			Manifest: tn.meta.Manifest,
		},
	})
	require.NoError(t, err)

	// System.Contract.Call + "sum" itself + opcodes for pushing arguments.
	price := int64(testSumPrice * 2)
	price += 3 * fee.Opcode(chain.GetBaseExecFee(), opcode.PUSHINT8)
	price += 2 * fee.Opcode(chain.GetBaseExecFee(), opcode.SYSCALL, opcode.PUSHDATA1, opcode.PUSHINT8)
	price += fee.Opcode(chain.GetBaseExecFee(), opcode.PACK)
	res, err := invokeContractMethod(chain, price, tn.Metadata().Hash, "sum", int64(14), int64(28))
	require.NoError(t, err)
	checkResult(t, res, stackitem.Make(42))
	require.NoError(t, chain.persist())

	select {
	case index := <-tn.blocks:
		require.Equal(t, chain.blockHeight, index)
	default:
		require.Fail(t, "onPersist wasn't called")
	}

	// Enough for Call and other opcodes, but not enough for "sum" call.
	res, err = invokeContractMethod(chain, price-1, tn.Metadata().Hash, "sum", int64(14), int64(28))
	require.NoError(t, err)
	checkFAULTState(t, res)
}

func TestNativeContract_InvokeInternal(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	tn := newTestNative()
	chain.registerNative(tn)

	err := chain.contracts.Management.PutContractState(chain.dao, &state.Contract{
		ContractBase: state.ContractBase{
			ID:       1,
			NEF:      tn.meta.NEF,
			Manifest: tn.meta.Manifest,
		},
	})
	require.NoError(t, err)

	d := dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, chain.config.StateRootInHeader)
	ic := chain.newInteropContext(trigger.Application, d, nil, nil)
	v := ic.SpawnVM()

	t.Run("fail, bad current script hash", func(t *testing.T) {
		v.LoadScriptWithHash([]byte{1}, util.Uint160{1, 2, 3}, callflag.All)
		v.Estack().PushVal(14)
		v.Estack().PushVal(28)
		v.Estack().PushVal("sum")
		v.Estack().PushVal(tn.Metadata().Name)

		// it's prohibited to call natives directly
		require.Error(t, native.Call(ic))
	})

	t.Run("success", func(t *testing.T) {
		v.LoadScriptWithHash([]byte{1}, tn.Metadata().Hash, callflag.All)
		v.Estack().PushVal(14)
		v.Estack().PushVal(28)
		v.Estack().PushVal("sum")
		v.Estack().PushVal(tn.Metadata().ContractID)

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

	err := chain.contracts.Management.PutContractState(chain.dao, &state.Contract{
		ContractBase: state.ContractBase{
			ID:       1,
			Hash:     tn.meta.Hash,
			NEF:      tn.meta.NEF,
			Manifest: tn.meta.Manifest,
		},
	})
	require.NoError(t, err)

	var drainTN = func(t *testing.T) {
		select {
		case <-tn.blocks:
		default:
			require.Fail(t, "testNative didn't send us block")
		}
	}

	cs, _ := getTestContractState(chain)
	require.NoError(t, chain.contracts.Management.PutContractState(chain.dao, cs))

	t.Run("non-native, no return", func(t *testing.T) {
		res, err := invokeContractMethod(chain, testSumPrice*4+10000, tn.Metadata().Hash, "callOtherContractNoReturn", cs.Hash, "justReturn", []interface{}{})
		require.NoError(t, err)
		drainTN(t)
		require.Equal(t, vm.HaltState, res.VMState, res.FaultException)
		checkResult(t, res, stackitem.Null{}) // simple call is done with EnsureNotEmpty
	})
	t.Run("non-native, with return", func(t *testing.T) {
		res, err := invokeContractMethod(chain, testSumPrice*4+10000, tn.Metadata().Hash,
			"callOtherContractWithReturn", cs.Hash, "ret7", []interface{}{})
		require.NoError(t, err)
		drainTN(t)
		checkResult(t, res, stackitem.Make(8))
	})
}
