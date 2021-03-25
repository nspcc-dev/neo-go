package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

/*  Missing tests:
 *  TestTxGetWitnesses
 *  TestAccountIsStandard
 *  TestCreateContractStateFromVM
 *  TestContractCreate
 *  TestContractMigrate
 *  TestRuntimeSerialize
 *  TestRuntimeDeserialize
 */

func TestStorageFind(t *testing.T) {
	v, contractState, context, chain := createVMAndContractState(t)

	arr := []stackitem.Item{
		stackitem.NewBigInteger(big.NewInt(42)),
		stackitem.NewByteArray([]byte("second")),
		stackitem.Null{},
	}
	rawArr, err := stackitem.SerializeItem(stackitem.NewArray(arr))
	require.NoError(t, err)
	rawArr0, err := stackitem.SerializeItem(stackitem.NewArray(arr[:0]))
	require.NoError(t, err)
	rawArr1, err := stackitem.SerializeItem(stackitem.NewArray(arr[:1]))
	require.NoError(t, err)

	skeys := [][]byte{{0x01, 0x02}, {0x02, 0x01}, {0x01, 0x01},
		{0x04, 0x00}, {0x05, 0x00}, {0x06}, {0x07}, {0x08}}
	items := []state.StorageItem{
		[]byte{0x01, 0x02, 0x03, 0x04},
		[]byte{0x04, 0x03, 0x02, 0x01},
		[]byte{0x03, 0x04, 0x05, 0x06},
		[]byte{byte(stackitem.ByteArrayT), 2, 0xCA, 0xFE},
		[]byte{0xFF, 0xFF},
		rawArr,
		rawArr0,
		rawArr1,
	}

	require.NoError(t, chain.contracts.Management.PutContractState(chain.dao, contractState))

	id := contractState.ID

	for i := range skeys {
		err := context.DAO.PutStorageItem(id, skeys[i], items[i])
		require.NoError(t, err)
	}

	testFind := func(t *testing.T, prefix byte, opts int64, expected []stackitem.Item) {
		v.Estack().PushVal(opts)
		v.Estack().PushVal([]byte{prefix})
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: id}))

		err := storageFind(context)
		require.NoError(t, err)

		var iter *stackitem.Interop
		require.NotPanics(t, func() { iter = v.Estack().Pop().Interop() })

		for i := range expected { // sorted indices with mathing prefix
			v.Estack().PushVal(iter)
			require.NoError(t, iterator.Next(context))
			require.True(t, v.Estack().Pop().Bool())

			v.Estack().PushVal(iter)
			if expected[i] == nil {
				require.Panics(t, func() { _ = iterator.Value(context) })
				return
			}
			require.NoError(t, iterator.Value(context))
			require.Equal(t, expected[i], v.Estack().Pop().Item())
		}

		v.Estack().PushVal(iter)
		require.NoError(t, iterator.Next(context))
		require.False(t, v.Estack().Pop().Bool())
	}

	t.Run("normal invocation", func(t *testing.T) {
		testFind(t, 0x01, istorage.FindDefault, []stackitem.Item{
			stackitem.NewStruct([]stackitem.Item{
				stackitem.NewByteArray(skeys[2]),
				stackitem.NewByteArray(items[2]),
			}),
			stackitem.NewStruct([]stackitem.Item{
				stackitem.NewByteArray(skeys[0]),
				stackitem.NewByteArray(items[0]),
			}),
		})
	})

	t.Run("keys only", func(t *testing.T) {
		testFind(t, 0x01, istorage.FindKeysOnly, []stackitem.Item{
			stackitem.NewByteArray(skeys[2]),
			stackitem.NewByteArray(skeys[0]),
		})
	})
	t.Run("remove prefix", func(t *testing.T) {
		testFind(t, 0x01, istorage.FindKeysOnly|istorage.FindRemovePrefix, []stackitem.Item{
			stackitem.NewByteArray(skeys[2][1:]),
			stackitem.NewByteArray(skeys[0][1:]),
		})
	})
	t.Run("values only", func(t *testing.T) {
		testFind(t, 0x01, istorage.FindValuesOnly, []stackitem.Item{
			stackitem.NewByteArray(items[2]),
			stackitem.NewByteArray(items[0]),
		})
	})
	t.Run("deserialize values", func(t *testing.T) {
		testFind(t, 0x04, istorage.FindValuesOnly|istorage.FindDeserialize, []stackitem.Item{
			stackitem.NewByteArray(items[3][2:]),
		})
		t.Run("invalid", func(t *testing.T) {
			v.Estack().PushVal(istorage.FindDeserialize)
			v.Estack().PushVal([]byte{0x05})
			v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: id}))
			err := storageFind(context)
			require.NoError(t, err)

			var iter *stackitem.Interop
			require.NotPanics(t, func() { iter = v.Estack().Pop().Interop() })

			v.Estack().PushVal(iter)
			require.NoError(t, iterator.Next(context))

			v.Estack().PushVal(iter)
			require.Panics(t, func() { _ = iterator.Value(context) })
		})
	})
	t.Run("PickN", func(t *testing.T) {
		testFind(t, 0x06, istorage.FindPick0|istorage.FindValuesOnly|istorage.FindDeserialize, arr[:1])
		testFind(t, 0x06, istorage.FindPick1|istorage.FindValuesOnly|istorage.FindDeserialize, arr[1:2])
		// Array with 0 elements.
		testFind(t, 0x07, istorage.FindPick0|istorage.FindValuesOnly|istorage.FindDeserialize,
			[]stackitem.Item{nil})
		// Array with 1 element.
		testFind(t, 0x08, istorage.FindPick1|istorage.FindValuesOnly|istorage.FindDeserialize,
			[]stackitem.Item{nil})
		// Not an array, but serialized ByteArray.
		testFind(t, 0x04, istorage.FindPick1|istorage.FindValuesOnly|istorage.FindDeserialize,
			[]stackitem.Item{nil})
	})

	t.Run("normal invocation, empty result", func(t *testing.T) {
		testFind(t, 0x03, istorage.FindDefault, nil)
	})

	t.Run("invalid options", func(t *testing.T) {
		invalid := []int64{
			istorage.FindKeysOnly | istorage.FindValuesOnly,
			^istorage.FindAll,
			istorage.FindKeysOnly | istorage.FindDeserialize,
			istorage.FindPick0,
			istorage.FindPick0 | istorage.FindPick1 | istorage.FindDeserialize,
			istorage.FindPick0 | istorage.FindPick1,
		}
		for _, opts := range invalid {
			v.Estack().PushVal(opts)
			v.Estack().PushVal([]byte{0x01})
			v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: id}))
			require.Error(t, storageFind(context))
		}
	})
	t.Run("invalid type for StorageContext", func(t *testing.T) {
		v.Estack().PushVal(istorage.FindDefault)
		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(nil))

		require.Error(t, storageFind(context))
	})

	t.Run("invalid id", func(t *testing.T) {
		invalidID := id + 1

		v.Estack().PushVal(istorage.FindDefault)
		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: invalidID}))

		require.NoError(t, storageFind(context))
		require.NoError(t, iterator.Next(context))
		require.False(t, v.Estack().Pop().Bool())
	})
}

// Helper functions to create VM, InteropContext, TX, Account, Contract.

func createVM(t *testing.T) (*vm.VM, *interop.Context, *Blockchain) {
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application,
		dao.NewSimple(storage.NewMemoryStore(), chain.config.StateRootInHeader), nil, nil)
	v := context.SpawnVM()
	return v, context, chain
}

func createVMAndPushBlock(t *testing.T) (*vm.VM, *block.Block, *interop.Context, *Blockchain) {
	v, block, context, chain := createVMAndBlock(t)
	v.Estack().PushVal(stackitem.NewInterop(block))
	return v, block, context, chain
}

func createVMAndBlock(t *testing.T) (*vm.VM, *block.Block, *interop.Context, *Blockchain) {
	block := newDumbBlock()
	chain := newTestChain(t)
	d := dao.NewSimple(storage.NewMemoryStore(), chain.GetConfig().StateRootInHeader)
	context := chain.newInteropContext(trigger.Application, d, block, nil)
	v := context.SpawnVM()
	return v, block, context, chain
}

func createVMAndPushTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interop.Context, *Blockchain) {
	v, tx, context, chain := createVMAndTX(t)
	v.Estack().PushVal(stackitem.NewInterop(tx))
	return v, tx, context, chain
}

func createVMAndContractState(t *testing.T) (*vm.VM, *state.Contract, *interop.Context, *Blockchain) {
	script := []byte("testscript")
	m := manifest.NewManifest("Test")
	ne, err := nef.NewFile(script)
	require.NoError(t, err)
	contractState := &state.Contract{
		ContractBase: state.ContractBase{
			NEF:      *ne,
			Hash:     hash.Hash160(script),
			Manifest: *m,
			ID:       123,
		},
	}

	chain := newTestChain(t)
	d := dao.NewSimple(storage.NewMemoryStore(), chain.config.StateRootInHeader)
	context := chain.newInteropContext(trigger.Application, d, nil, nil)
	v := context.SpawnVM()
	return v, contractState, context, chain
}

func createVMAndTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interop.Context, *Blockchain) {
	script := []byte{byte(opcode.PUSH1), byte(opcode.RET)}
	tx := transaction.New(script, 0)
	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3, 4}}}
	tx.Scripts = []transaction.Witness{{InvocationScript: []byte{}, VerificationScript: []byte{}}}
	chain := newTestChain(t)
	d := dao.NewSimple(storage.NewMemoryStore(), chain.config.StateRootInHeader)
	context := chain.newInteropContext(trigger.Application, d, nil, tx)
	v := context.SpawnVM()
	return v, tx, context, chain
}
