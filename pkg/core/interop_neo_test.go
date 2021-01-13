package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/enumerator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
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
	defer chain.Close()

	skeys := [][]byte{{0x01, 0x02}, {0x02, 0x01}, {0x01, 0x01}}
	items := []*state.StorageItem{
		{
			Value: []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			Value: []byte{0x04, 0x03, 0x02, 0x01},
		},
		{
			Value: []byte{0x03, 0x04, 0x05, 0x06},
		},
	}

	require.NoError(t, chain.contracts.Management.PutContractState(chain.dao, contractState))

	id := contractState.ID

	for i := range skeys {
		err := context.DAO.PutStorageItem(id, skeys[i], items[i])
		require.NoError(t, err)
	}

	t.Run("normal invocation", func(t *testing.T) {
		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: id}))

		err := storageFind(context)
		require.NoError(t, err)

		var iter *stackitem.Interop
		require.NotPanics(t, func() { iter = v.Estack().Top().Interop() })

		require.NoError(t, enumerator.Next(context))
		require.True(t, v.Estack().Pop().Bool())

		v.Estack().PushVal(iter)
		require.NoError(t, iterator.Key(context))
		require.Equal(t, []byte{0x01, 0x01}, v.Estack().Pop().Bytes())

		v.Estack().PushVal(iter)
		require.NoError(t, enumerator.Value(context))
		require.Equal(t, []byte{0x03, 0x04, 0x05, 0x06}, v.Estack().Pop().Bytes())

		v.Estack().PushVal(iter)
		require.NoError(t, enumerator.Next(context))
		require.True(t, v.Estack().Pop().Bool())

		v.Estack().PushVal(iter)
		require.NoError(t, iterator.Key(context))
		require.Equal(t, []byte{0x01, 0x02}, v.Estack().Pop().Bytes())

		v.Estack().PushVal(iter)
		require.NoError(t, enumerator.Value(context))
		require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, v.Estack().Pop().Bytes())

		v.Estack().PushVal(iter)
		require.NoError(t, enumerator.Next(context))
		require.False(t, v.Estack().Pop().Bool())
	})

	t.Run("normal invocation, empty result", func(t *testing.T) {
		v.Estack().PushVal([]byte{0x03})
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: id}))

		err := storageFind(context)
		require.NoError(t, err)

		require.NoError(t, enumerator.Next(context))
		require.False(t, v.Estack().Pop().Bool())
	})

	t.Run("invalid type for StorageContext", func(t *testing.T) {
		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(nil))

		require.Error(t, storageFind(context))
	})

	t.Run("invalid id", func(t *testing.T) {
		invalidID := id + 1

		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ID: invalidID}))

		require.NoError(t, storageFind(context))
		require.NoError(t, enumerator.Next(context))
		require.False(t, v.Estack().Pop().Bool())
	})
}

// Helper functions to create VM, InteropContext, TX, Account, Contract.

func createVM(t *testing.T) (*vm.VM, *interop.Context, *Blockchain) {
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application,
		dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, chain.config.StateRootInHeader), nil, nil)
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
	d := dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, chain.GetConfig().StateRootInHeader)
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
		NEF:      *ne,
		Hash:     hash.Hash160(script),
		Manifest: *m,
		ID:       123,
	}

	chain := newTestChain(t)
	d := dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, chain.config.StateRootInHeader)
	context := chain.newInteropContext(trigger.Application, d, nil, nil)
	v := context.SpawnVM()
	return v, contractState, context, chain
}

func createVMAndTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interop.Context, *Blockchain) {
	script := []byte{byte(opcode.PUSH1), byte(opcode.RET)}
	tx := transaction.New(netmode.UnitTestNet, script, 0)

	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3, 4}}}
	chain := newTestChain(t)
	d := dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet, chain.config.StateRootInHeader)
	context := chain.newInteropContext(trigger.Application, d, nil, tx)
	v := context.SpawnVM()
	return v, tx, context, chain
}
