package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
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
	"github.com/stretchr/testify/require"
)

/*  Missing tests:
 *  TestTxGetReferences
 *  TestTxGetUnspentCoins
 *  TestTxGetWitnesses
 *  TestBcGetAccount
 *  TestBcGetAsset
 *  TestAccountGetBalance
 *  TestAccountIsStandard
 *  TestCreateContractStateFromVM
 *  TestContractCreate
 *  TestContractMigrate
 *  TestAssetCreate
 *  TestAssetRenew
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

	skeys := [][]byte{{0x01, 0x02}, {0x02, 0x01}}
	items := []*state.StorageItem{
		{
			Value: []byte{0x01, 0x02, 0x03, 0x04},
		},
		{
			Value: []byte{0x04, 0x03, 0x02, 0x01},
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
		v.Estack().PushVal(vm.NewInteropItem(&StorageContext{ScriptHash: scriptHash}))

		err := storageFind(context, v)
		require.NoError(t, err)

		var iter *vm.InteropItem
		require.NotPanics(t, func() { iter = v.Estack().Top().Interop() })

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

	t.Run("invalid type for StorageContext", func(t *testing.T) {
		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(vm.NewInteropItem(nil))

		require.Error(t, storageFind(context, v))
	})

	t.Run("invalid script hash", func(t *testing.T) {
		invalidHash := scriptHash
		invalidHash[0] = ^invalidHash[0]

		v.Estack().PushVal([]byte{0x01})
		v.Estack().PushVal(vm.NewInteropItem(&StorageContext{ScriptHash: invalidHash}))

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
	v.Estack().PushVal(vm.NewBoolItem(false))

	err := headerGetVersion(context, v)
	require.Errorf(t, err, "value is not a header or block")
}

func TestHeaderGetConsensusData(t *testing.T) {
	v, block, context, chain := createVMAndPushBlock(t)
	defer chain.Close()

	err := headerGetConsensusData(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().(*big.Int)
	require.Equal(t, block.ConsensusData, value.Uint64())
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

func TestTxGetAttributes(t *testing.T) {
	v, tx, context, chain := createVMAndPushTX(t)
	defer chain.Close()

	err := txGetAttributes(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().([]vm.StackItem)
	require.Equal(t, tx.Attributes[0].Usage, value[0].Value().(*transaction.Attribute).Usage)
}

func TestTxGetInputs(t *testing.T) {
	v, tx, context, chain := createVMAndPushTX(t)
	defer chain.Close()

	err := txGetInputs(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().([]vm.StackItem)
	require.Equal(t, tx.Inputs[0], *value[0].Value().(*transaction.Input))
}

func TestTxGetOutputs(t *testing.T) {
	v, tx, context, chain := createVMAndPushTX(t)
	defer chain.Close()

	err := txGetOutputs(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().([]vm.StackItem)
	require.Equal(t, tx.Outputs[0], *value[0].Value().(*transaction.Output))
}

func TestTxGetType(t *testing.T) {
	v, tx, context, chain := createVMAndPushTX(t)
	defer chain.Close()

	err := txGetType(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().(*big.Int)
	require.Equal(t, big.NewInt(int64(tx.Type)), value)
}

func TestInvocationTxGetScript(t *testing.T) {
	v, tx, context, chain := createVMAndPushTX(t)
	defer chain.Close()

	err := invocationTxGetScript(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().([]byte)
	inv := tx.Data.(*transaction.InvocationTX)
	require.Equal(t, inv.Script, value)
}

func TestWitnessGetVerificationScript(t *testing.T) {
	v := vm.New()
	script := []byte{byte(opcode.PUSHM1), byte(opcode.RET)}
	witness := transaction.Witness{InvocationScript: nil, VerificationScript: script}

	chain := newTestChain(t)
	defer chain.Close()

	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), nil, nil)
	v.Estack().PushVal(vm.NewInteropItem(&witness))
	err := witnessGetVerificationScript(context, v)
	require.NoError(t, err)
	value := v.Estack().Pop().Value().([]byte)
	require.Equal(t, witness.VerificationScript, value)
}

func TestPopInputFromVM(t *testing.T) {
	v, tx, _, chain := createVMAndTX(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(&tx.Inputs[0]))

	input, err := popInputFromVM(v)
	require.NoError(t, err)
	require.Equal(t, tx.Inputs[0], *input)
}

func TestInputGetHash(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(&tx.Inputs[0]))

	err := inputGetHash(context, v)
	require.NoError(t, err)
	hash := v.Estack().Pop().Value()
	require.Equal(t, tx.Inputs[0].PrevHash.BytesBE(), hash)
}

func TestInputGetIndex(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(&tx.Inputs[0]))

	err := inputGetIndex(context, v)
	require.NoError(t, err)
	index := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(tx.Inputs[0].PrevIndex)), index)
}

func TestPopOutputFromVM(t *testing.T) {
	v, tx, _, chain := createVMAndTX(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(&tx.Outputs[0]))

	output, err := popOutputFromVM(v)
	require.NoError(t, err)
	require.Equal(t, tx.Outputs[0], *output)
}

func TestOutputGetAssetID(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(&tx.Outputs[0]))

	err := outputGetAssetID(context, v)
	require.NoError(t, err)
	assetID := v.Estack().Pop().Value()
	require.Equal(t, tx.Outputs[0].AssetID.BytesBE(), assetID)
}

func TestOutputGetScriptHash(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(&tx.Outputs[0]))

	err := outputGetScriptHash(context, v)
	require.NoError(t, err)
	scriptHash := v.Estack().Pop().Value()
	require.Equal(t, tx.Outputs[0].ScriptHash.BytesBE(), scriptHash)
}

func TestOutputGetValue(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(&tx.Outputs[0]))

	err := outputGetValue(context, v)
	require.NoError(t, err)
	amount := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(tx.Outputs[0].Amount)), amount)
}

func TestAttrGetData(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(&tx.Attributes[0]))

	err := attrGetData(context, v)
	require.NoError(t, err)
	data := v.Estack().Pop().Value()
	require.Equal(t, tx.Attributes[0].Data, data)
}

func TestAttrGetUsage(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(&tx.Attributes[0]))

	err := attrGetUsage(context, v)
	require.NoError(t, err)
	usage := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(tx.Attributes[0].Usage)), usage)
}

func TestAccountGetScriptHash(t *testing.T) {
	v, accState, context, chain := createVMAndAccState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(accState))

	err := accountGetScriptHash(context, v)
	require.NoError(t, err)
	hash := v.Estack().Pop().Value()
	require.Equal(t, accState.ScriptHash.BytesBE(), hash)
}

func TestAccountGetVotes(t *testing.T) {
	v, accState, context, chain := createVMAndAccState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(accState))

	err := accountGetVotes(context, v)
	require.NoError(t, err)
	votes := v.Estack().Pop().Value().([]vm.StackItem)
	require.Equal(t, vm.NewByteArrayItem(accState.Votes[0].Bytes()), votes[0])
}

func TestContractGetScript(t *testing.T) {
	v, contractState, context, chain := createVMAndContractState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(contractState))

	err := contractGetScript(context, v)
	require.NoError(t, err)
	script := v.Estack().Pop().Value()
	require.Equal(t, contractState.Script, script)
}

func TestContractIsPayable(t *testing.T) {
	v, contractState, context, chain := createVMAndContractState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(contractState))

	err := contractIsPayable(context, v)
	require.NoError(t, err)
	isPayable := v.Estack().Pop().Value()
	require.Equal(t, contractState.IsPayable(), isPayable)
}

func TestAssetGetAdmin(t *testing.T) {
	v, assetState, context, chain := createVMAndAssetState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := assetGetAdmin(context, v)
	require.NoError(t, err)
	admin := v.Estack().Pop().Value()
	require.Equal(t, assetState.Admin.BytesBE(), admin)
}

func TestAssetGetAmount(t *testing.T) {
	v, assetState, context, chain := createVMAndAssetState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := assetGetAmount(context, v)
	require.NoError(t, err)
	amount := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(assetState.Amount)), amount)
}

func TestAssetGetAssetID(t *testing.T) {
	v, assetState, context, chain := createVMAndAssetState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := assetGetAssetID(context, v)
	require.NoError(t, err)
	assetID := v.Estack().Pop().Value()
	require.Equal(t, assetState.ID.BytesBE(), assetID)
}

func TestAssetGetAssetType(t *testing.T) {
	v, assetState, context, chain := createVMAndAssetState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := assetGetAssetType(context, v)
	require.NoError(t, err)
	assetType := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(assetState.AssetType)), assetType)
}

func TestAssetGetAvailable(t *testing.T) {
	v, assetState, context, chain := createVMAndAssetState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := assetGetAvailable(context, v)
	require.NoError(t, err)
	available := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(assetState.Available)), available)
}

func TestAssetGetIssuer(t *testing.T) {
	v, assetState, context, chain := createVMAndAssetState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := assetGetIssuer(context, v)
	require.NoError(t, err)
	issuer := v.Estack().Pop().Value()
	require.Equal(t, assetState.Issuer.BytesBE(), issuer)
}

func TestAssetGetOwner(t *testing.T) {
	v, assetState, context, chain := createVMAndAssetState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := assetGetOwner(context, v)
	require.NoError(t, err)
	owner := v.Estack().Pop().Value()
	require.Equal(t, assetState.Owner.Bytes(), owner)
}

func TestAssetGetPrecision(t *testing.T) {
	v, assetState, context, chain := createVMAndAssetState(t)
	defer chain.Close()
	v.Estack().PushVal(vm.NewInteropItem(assetState))

	err := assetGetPrecision(context, v)
	require.NoError(t, err)
	precision := v.Estack().Pop().Value()
	require.Equal(t, big.NewInt(int64(assetState.Precision)), precision)
}

// Helper functions to create VM, InteropContext, TX, Account, Contract, Asset.

func createVMAndPushBlock(t *testing.T) (*vm.VM, *block.Block, *interop.Context, *Blockchain) {
	v := vm.New()
	block := newDumbBlock()
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), block, nil)
	v.Estack().PushVal(vm.NewInteropItem(block))
	return v, block, context, chain
}

func createVMAndPushTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interop.Context, *Blockchain) {
	v, tx, context, chain := createVMAndTX(t)
	v.Estack().PushVal(vm.NewInteropItem(tx))
	return v, tx, context, chain
}

func createVMAndAssetState(t *testing.T) (*vm.VM, *state.Asset, *interop.Context, *Blockchain) {
	v := vm.New()
	assetState := &state.Asset{
		ID:         util.Uint256{},
		AssetType:  transaction.GoverningToken,
		Name:       "TestAsset",
		Amount:     1,
		Available:  2,
		Precision:  1,
		FeeMode:    1,
		FeeAddress: random.Uint160(),
		Owner:      keys.PublicKey{X: big.NewInt(1), Y: big.NewInt(1)},
		Admin:      random.Uint160(),
		Issuer:     random.Uint160(),
		Expiration: 10,
		IsFrozen:   false,
	}

	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), nil, nil)
	return v, assetState, context, chain
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

	key := &keys.PublicKey{X: big.NewInt(1), Y: big.NewInt(1)}
	accountState.Votes = []*keys.PublicKey{key}

	require.NoError(t, err)
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), nil, nil)
	return v, accountState, context, chain
}

func createVMAndTX(t *testing.T) (*vm.VM, *transaction.Transaction, *interop.Context, *Blockchain) {
	v := vm.New()
	script := []byte{byte(opcode.PUSH1), byte(opcode.RET)}
	tx := transaction.NewInvocationTX(script, 0)

	bytes := make([]byte, 1)
	attributes := append(tx.Attributes, transaction.Attribute{
		Usage: transaction.Description,
		Data:  bytes,
	})

	inputs := append(tx.Inputs, transaction.Input{
		PrevHash:  random.Uint256(),
		PrevIndex: 1,
	})

	outputs := append(tx.Outputs, transaction.Output{
		AssetID:    random.Uint256(),
		Amount:     10,
		ScriptHash: random.Uint160(),
		Position:   1,
	})

	tx.Attributes = attributes
	tx.Inputs = inputs
	tx.Outputs = outputs
	chain := newTestChain(t)
	context := chain.newInteropContext(trigger.Application, dao.NewSimple(storage.NewMemoryStore()), nil, tx)
	return v, tx, context, chain
}
