package core

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/enumerator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
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

func TestGetTrigger(t *testing.T) {
	_, _, context, chain := createVMAndPushBlock(t)
	defer chain.Close()
	require.NoError(t, runtimeGetTrigger(context))
}

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

	require.NoError(t, context.DAO.PutContractState(contractState))

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

func TestECDSAVerify(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	chain := newTestChain(t)
	defer chain.Close()

	ic := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet), nil, nil)
	runCase := func(t *testing.T, isErr bool, result interface{}, args ...interface{}) {
		v := vm.New()
		ic.VM = v
		for i := range args {
			v.Estack().PushVal(args[i])
		}

		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			err = crypto.ECDSASecp256r1Verify(ic)
		}()

		if isErr {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)
		require.Equal(t, 1, v.Estack().Len())
		require.Equal(t, result, v.Estack().Pop().Value().(bool))
	}

	msg := []byte("test message")

	t.Run("success", func(t *testing.T) {
		sign := priv.Sign(msg)
		runCase(t, false, true, sign, priv.PublicKey().Bytes(), msg)
	})

	t.Run("signed interop item", func(t *testing.T) {
		tx := transaction.New(netmode.UnitTestNet, []byte{0, 1, 2}, 1)
		msg := tx.GetSignedPart()
		sign := priv.Sign(msg)
		runCase(t, false, true, sign, priv.PublicKey().Bytes(), stackitem.NewInterop(tx))
	})

	t.Run("signed script container", func(t *testing.T) {
		tx := transaction.New(netmode.UnitTestNet, []byte{0, 1, 2}, 1)
		msg := tx.GetSignedPart()
		sign := priv.Sign(msg)
		ic.Container = tx
		runCase(t, false, true, sign, priv.PublicKey().Bytes(), stackitem.Null{})
	})

	t.Run("missing arguments", func(t *testing.T) {
		runCase(t, true, false)
		sign := priv.Sign(msg)
		runCase(t, true, false, sign)
		runCase(t, true, false, sign, priv.PublicKey().Bytes())
	})

	t.Run("invalid signature", func(t *testing.T) {
		sign := priv.Sign(msg)
		sign[0] = ^sign[0]
		runCase(t, false, false, sign, priv.PublicKey().Bytes(), msg)
	})

	t.Run("invalid public key", func(t *testing.T) {
		sign := priv.Sign(msg)
		pub := priv.PublicKey().Bytes()
		pub[0] = 0xFF // invalid prefix
		runCase(t, true, false, sign, pub, msg)
	})

	t.Run("invalid message", func(t *testing.T) {
		sign := priv.Sign(msg)
		runCase(t, false, false, sign, priv.PublicKey().Bytes(),
			stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray(msg)}))
	})
}

func TestRuntimeEncode(t *testing.T) {
	str := []byte("my pretty string")
	v, ic, bc := createVM(t)
	defer bc.Close()

	v.Estack().PushVal(str)
	require.NoError(t, runtimeEncode(ic))

	expected := []byte(base64.StdEncoding.EncodeToString(str))
	actual := v.Estack().Pop().Bytes()
	require.Equal(t, expected, actual)
}

func TestRuntimeDecode(t *testing.T) {
	expected := []byte("my pretty string")
	str := base64.StdEncoding.EncodeToString(expected)
	v, ic, bc := createVM(t)
	defer bc.Close()

	t.Run("positive", func(t *testing.T) {
		v.Estack().PushVal(str)
		require.NoError(t, runtimeDecode(ic))

		actual := v.Estack().Pop().Bytes()
		require.Equal(t, expected, actual)
	})

	t.Run("error", func(t *testing.T) {
		v.Estack().PushVal(str + "%")
		require.Error(t, runtimeDecode(ic))
	})
}

// Helper functions to create VM, InteropContext, TX, Account, Contract.

func createVM(t *testing.T) (*vm.VM, *interop.Context, *Blockchain) {
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application,
		dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet), nil, nil)
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
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet), block, nil)
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
	m := manifest.NewManifest(hash.Hash160(script))
	m.Features = smartcontract.HasStorage
	contractState := &state.Contract{
		Script:   script,
		Manifest: *m,
		ID:       123,
	}

	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet), nil, nil)
	v := context.SpawnVM()
	return v, contractState, context, chain
}

func createVMAndTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interop.Context, *Blockchain) {
	script := []byte{byte(opcode.PUSH1), byte(opcode.RET)}
	tx := transaction.New(netmode.UnitTestNet, script, 0)

	tx.Signers = []transaction.Signer{{Account: util.Uint160{1, 2, 3, 4}}}
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore(), netmode.UnitTestNet), nil, tx)
	v := context.SpawnVM()
	return v, tx, context, chain
}
