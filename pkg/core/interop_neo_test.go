package core

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/enumerator"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

/*  Missing tests:
 *  TestTxGetWitnesses
 *  TestBcGetAccount
 *  TestAccountGetBalance
 *  TestAccountIsStandard
 *  TestCreateContractStateFromVM
 *  TestContractCreate
 *  TestContractMigrate
 *  TestRuntimeSerialize
 *  TestRuntimeDeserialize
 */

func TestGetTrigger(t *testing.T) {
	v, _, context, chain := createVMAndPushBlock(t)
	defer chain.Close()
	require.NoError(t, runtimeGetTrigger(context, v))
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

	scriptHash := contractState.ScriptHash()

	for i := range skeys {
		err := context.DAO.PutStorageItem(scriptHash, skeys[i], items[i])
		require.NoError(t, err)
	}

	t.Run("normal invocation", func(t *testing.T) {
		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ScriptHash: scriptHash}))

		err := storageFind(context, v)
		require.NoError(t, err)

		var iter *stackitem.Interop
		require.NotPanics(t, func() { iter = v.Estack().Top().Interop() })

		require.NoError(t, enumerator.Next(context, v))
		require.True(t, v.Estack().Pop().Bool())

		v.Estack().PushVal(iter)
		require.NoError(t, iterator.Key(context, v))
		require.Equal(t, []byte{0x01, 0x01}, v.Estack().Pop().Bytes())

		v.Estack().PushVal(iter)
		require.NoError(t, enumerator.Value(context, v))
		require.Equal(t, []byte{0x03, 0x04, 0x05, 0x06}, v.Estack().Pop().Bytes())

		v.Estack().PushVal(iter)
		require.NoError(t, enumerator.Next(context, v))
		require.True(t, v.Estack().Pop().Bool())

		v.Estack().PushVal(iter)
		require.NoError(t, iterator.Key(context, v))
		require.Equal(t, []byte{0x01, 0x02}, v.Estack().Pop().Bytes())

		v.Estack().PushVal(iter)
		require.NoError(t, enumerator.Value(context, v))
		require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, v.Estack().Pop().Bytes())

		v.Estack().PushVal(iter)
		require.NoError(t, enumerator.Next(context, v))
		require.False(t, v.Estack().Pop().Bool())
	})

	t.Run("normal invocation, empty result", func(t *testing.T) {
		v.Estack().PushVal([]byte{0x03})
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ScriptHash: scriptHash}))

		err := storageFind(context, v)
		require.NoError(t, err)

		require.NoError(t, enumerator.Next(context, v))
		require.False(t, v.Estack().Pop().Bool())
	})

	t.Run("invalid type for StorageContext", func(t *testing.T) {
		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(nil))

		require.Error(t, storageFind(context, v))
	})

	t.Run("invalid script hash", func(t *testing.T) {
		invalidHash := scriptHash
		invalidHash[0] = ^invalidHash[0]

		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(stackitem.NewInterop(&StorageContext{ScriptHash: invalidHash}))

		require.Error(t, storageFind(context, v))
	})
}

func TestHeaderGetVersion(t *testing.T) {
	v, block, context, chain := createVMAndPushBlock(t)
	defer chain.Close()

	err := headerGetVersion(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().(*big.Int)
	require.Equal(t, uint64(block.Version), value.Uint64())
}

func TestHeaderGetVersion_Negative(t *testing.T) {
	v := vm.New()
	block := newDumbBlock()
	chain := newTestChain(t)
	defer chain.Close()
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), block, nil)
	v.Estack().PushVal(stackitem.NewBool(false))

	err := headerGetVersion(context, v)
	require.Errorf(t, err, "value is not a header or block")
}

func TestHeaderGetMerkleRoot(t *testing.T) {
	v, block, context, chain := createVMAndPushBlock(t)
	defer chain.Close()

	err := headerGetMerkleRoot(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value()
	require.Equal(t, block.MerkleRoot.BytesBE(), value)
}

func TestHeaderGetNextConsensus(t *testing.T) {
	v, block, context, chain := createVMAndPushBlock(t)
	defer chain.Close()

	err := headerGetNextConsensus(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value()
	require.Equal(t, block.NextConsensus.BytesBE(), value)
}

func TestWitnessGetVerificationScript(t *testing.T) {
	v := vm.New()
	script := []byte{byte(opcode.PUSHM1), byte(opcode.RET)}
	witness := transaction.Witness{InvocationScript: nil, VerificationScript: script}

	chain := newTestChain(t)
	defer chain.Close()

	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), nil, nil)
	v.Estack().PushVal(stackitem.NewInterop(&witness))
	err := witnessGetVerificationScript(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().([]byte)
	require.Equal(t, witness.VerificationScript, value)
}

func TestECDSAVerify(t *testing.T) {
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	chain := newTestChain(t)
	defer chain.Close()

	ic := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), nil, nil)
	runCase := func(t *testing.T, isErr bool, result interface{}, args ...interface{}) {
		v := vm.New()
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
			err = crypto.ECDSAVerify(ic, v)
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
		tx := transaction.New([]byte{0, 1, 2}, 1)
		msg := tx.GetSignedPart()
		sign := priv.Sign(msg)
		runCase(t, false, true, sign, priv.PublicKey().Bytes(), stackitem.NewInterop(tx))
	})

	t.Run("signed script container", func(t *testing.T) {
		tx := transaction.New([]byte{0, 1, 2}, 1)
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
		pub = pub[10:]
		runCase(t, true, false, sign, pub, msg)
	})
}

func TestAccountGetScriptHash(t *testing.T) {
	v, accState, context, chain := createVMAndAccState(t)
	defer chain.Close()
	v.Estack().PushVal(stackitem.NewInterop(accState))

	err := accountGetScriptHash(context, v)
	require.NoError(t, err)
	hash := v.Estack().Pop().Value()
	require.Equal(t, accState.ScriptHash.BytesBE(), hash)
}

func TestContractGetScript(t *testing.T) {
	v, contractState, context, chain := createVMAndContractState(t)
	defer chain.Close()
	v.Estack().PushVal(stackitem.NewInterop(contractState))

	err := contractGetScript(context, v)
	require.NoError(t, err)
	script := v.Estack().Pop().Value()
	require.Equal(t, contractState.Script, script)
}

func TestContractIsPayable(t *testing.T) {
	v, contractState, context, chain := createVMAndContractState(t)
	defer chain.Close()
	v.Estack().PushVal(stackitem.NewInterop(contractState))

	err := contractIsPayable(context, v)
	require.NoError(t, err)
	isPayable := v.Estack().Pop().Value()
	require.Equal(t, contractState.IsPayable(), isPayable)
}

// Helper functions to create VM, InteropContext, TX, Account, Contract.

func createVMAndPushBlock(t *testing.T) (*vm.VM, *block.Block, *interop.Context, *Blockchain) {
	v := vm.New()
	block := newDumbBlock()
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), block, nil)
	v.Estack().PushVal(stackitem.NewInterop(block))
	return v, block, context, chain
}

func createVMAndPushTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interop.Context, *Blockchain) {
	v, tx, context, chain := createVMAndTX(t)
	v.Estack().PushVal(stackitem.NewInterop(tx))
	return v, tx, context, chain
}

func createVMAndContractState(t *testing.T) (*vm.VM, *state.Contract, *interop.Context, *Blockchain) {
	v := vm.New()
	contractState := &state.Contract{
		Script:      []byte("testscript"),
		ParamList:   []smartcontract.ParamType{smartcontract.StringType, smartcontract.IntegerType, smartcontract.Hash160Type},
		ReturnType:  smartcontract.ArrayType,
		Properties:  smartcontract.HasStorage,
		Name:        random.String(10),
		CodeVersion: random.String(10),
		Author:      random.String(10),
		Email:       random.String(10),
		Description: random.String(10),
	}

	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), nil, nil)
	return v, contractState, context, chain
}

func createVMAndAccState(t *testing.T) (*vm.VM, *state.Account, *interop.Context, *Blockchain) {
	v := vm.New()
	rawHash := "4d3b96ae1bcc5a585e075e3b81920210dec16302"
	hash, err := util.Uint160DecodeStringBE(rawHash)
	accountState := state.NewAccount(hash)

	require.NoError(t, err)
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), nil, nil)
	return v, accountState, context, chain
}

func createVMAndTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interop.Context, *Blockchain) {
	v := vm.New()
	script := []byte{byte(opcode.PUSH1), byte(opcode.RET)}
	tx := transaction.New(script, 0)

	bytes := make([]byte, 1)
	attributes := append(tx.Attributes, transaction.Attribute{
		Usage: transaction.Description,
		Data:  bytes,
	})

	tx.Attributes = attributes
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), nil, tx)
	return v, tx, context, chain
}
