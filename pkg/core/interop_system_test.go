package core

import (
	"errors"
	"math/big"
	"testing"

	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestBCGetTransactionHeight(t *testing.T) {
	v, tx, ic, chain := createVMAndTX(t)
	defer chain.Close()

	for i := 0; i < 13; i++ {
		require.NoError(t, chain.AddBlock(chain.newBlock()))
	}
	require.NoError(t, ic.DAO.StoreAsTransaction(tx, 13, nil))
	t.Run("good", func(t *testing.T) {
		v.Estack().PushVal(tx.Hash().BytesBE())
		require.NoError(t, bcGetTransactionHeight(ic))
		require.Equal(t, big.NewInt(13), v.Estack().Pop().BigInt())
	})
	t.Run("bad", func(t *testing.T) {
		h := tx.Hash()
		h[0] ^= 0xFF
		v.Estack().PushVal(h.BytesBE())
		require.NoError(t, bcGetTransactionHeight(ic))
		require.Equal(t, big.NewInt(-1), v.Estack().Pop().BigInt())
	})
}

func TestBCGetTransaction(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()

	t.Run("success", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 0, nil))
		v.Estack().PushVal(tx.Hash().BytesBE())
		err := bcGetTransaction(context)
		require.NoError(t, err)

		value := v.Estack().Pop().Value()
		actual, ok := value.([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, 8, len(actual))
		require.Equal(t, tx.Hash().BytesBE(), actual[0].Value().([]byte))
		require.Equal(t, int64(tx.Version), actual[1].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.Nonce), actual[2].Value().(*big.Int).Int64())
		require.Equal(t, tx.Sender().BytesBE(), actual[3].Value().([]byte))
		require.Equal(t, int64(tx.SystemFee), actual[4].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.NetworkFee), actual[5].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.ValidUntilBlock), actual[6].Value().(*big.Int).Int64())
		require.Equal(t, tx.Script, actual[7].Value().([]byte))
	})

	t.Run("isn't traceable", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 1, nil))
		v.Estack().PushVal(tx.Hash().BytesBE())
		err := bcGetTransaction(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad hash", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 1, nil))
		v.Estack().PushVal(tx.Hash().BytesLE())
		err := bcGetTransaction(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})
}

func TestBCGetTransactionFromBlock(t *testing.T) {
	v, block, context, chain := createVMAndBlock(t)
	defer chain.Close()
	require.NoError(t, chain.AddBlock(chain.newBlock()))
	require.NoError(t, context.DAO.StoreAsBlock(block, nil))

	t.Run("success", func(t *testing.T) {
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetTransactionFromBlock(context)
		require.NoError(t, err)

		value := v.Estack().Pop().Value()
		actual, ok := value.([]byte)
		require.True(t, ok)
		require.Equal(t, block.Transactions[0].Hash().BytesBE(), actual)
	})

	t.Run("invalid block hash", func(t *testing.T) {
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesBE()[:10])
		err := bcGetTransactionFromBlock(context)
		require.Error(t, err)
	})

	t.Run("isn't traceable", func(t *testing.T) {
		block.Index = 2
		require.NoError(t, context.DAO.StoreAsBlock(block, nil))
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetTransactionFromBlock(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad block hash", func(t *testing.T) {
		block.Index = 1
		require.NoError(t, context.DAO.StoreAsBlock(block, nil))
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesLE())
		err := bcGetTransactionFromBlock(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad transaction index", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsBlock(block, nil))
		v.Estack().PushVal(1)
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetTransactionFromBlock(context)
		require.Error(t, err)
	})
}

func TestBCGetBlock(t *testing.T) {
	v, context, chain := createVM(t)
	defer chain.Close()
	block := chain.newBlock()
	require.NoError(t, chain.AddBlock(block))

	t.Run("success", func(t *testing.T) {
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetBlock(context)
		require.NoError(t, err)

		value := v.Estack().Pop().Value()
		actual, ok := value.([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, 8, len(actual))
		require.Equal(t, block.Hash().BytesBE(), actual[0].Value().([]byte))
		require.Equal(t, int64(block.Version), actual[1].Value().(*big.Int).Int64())
		require.Equal(t, block.PrevHash.BytesBE(), actual[2].Value().([]byte))
		require.Equal(t, block.MerkleRoot.BytesBE(), actual[3].Value().([]byte))
		require.Equal(t, int64(block.Timestamp), actual[4].Value().(*big.Int).Int64())
		require.Equal(t, int64(block.Index), actual[5].Value().(*big.Int).Int64())
		require.Equal(t, block.NextConsensus.BytesBE(), actual[6].Value().([]byte))
		require.Equal(t, int64(len(block.Transactions)), actual[7].Value().(*big.Int).Int64())
	})

	t.Run("bad hash", func(t *testing.T) {
		v.Estack().PushVal(block.Hash().BytesLE())
		err := bcGetTransaction(context)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})
}

func TestContractIsStandard(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()

	t.Run("contract not stored", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)

		pub := priv.PublicKey()
		tx := transaction.New(netmode.TestNet, []byte{1, 2, 3}, 1)
		tx.Scripts = []transaction.Witness{
			{
				InvocationScript:   []byte{1, 2, 3},
				VerificationScript: pub.GetVerificationScript(),
			},
		}
		ic.Container = tx

		t.Run("true", func(t *testing.T) {
			v.Estack().PushVal(pub.GetScriptHash().BytesBE())
			require.NoError(t, contractIsStandard(ic))
			require.True(t, v.Estack().Pop().Bool())
		})

		t.Run("false", func(t *testing.T) {
			tx.Scripts[0].VerificationScript = []byte{9, 8, 7}
			v.Estack().PushVal(pub.GetScriptHash().BytesBE())
			require.NoError(t, contractIsStandard(ic))
			require.False(t, v.Estack().Pop().Bool())
		})
	})

	t.Run("contract stored, true", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)

		pub := priv.PublicKey()
		ne, err := nef.NewFile(pub.GetVerificationScript())
		require.NoError(t, err)
		err = chain.contracts.Management.PutContractState(ic.DAO, &state.Contract{ID: 42, Hash: pub.GetScriptHash(), NEF: *ne})
		require.NoError(t, err)

		v.Estack().PushVal(pub.GetScriptHash().BytesBE())
		require.NoError(t, contractIsStandard(ic))
		require.True(t, v.Estack().Pop().Bool())
	})
	t.Run("contract stored, false", func(t *testing.T) {
		script := []byte{byte(opcode.PUSHT)}
		ne, err := nef.NewFile(script)
		require.NoError(t, err)
		require.NoError(t, chain.contracts.Management.PutContractState(ic.DAO, &state.Contract{ID: 24, Hash: hash.Hash160(script), NEF: *ne}))

		v.Estack().PushVal(crypto.Hash160(script).BytesBE())
		require.NoError(t, contractIsStandard(ic))
		require.False(t, v.Estack().Pop().Bool())
	})
}

func TestContractCreateAccount(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()
	t.Run("Good", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		pub := priv.PublicKey()
		v.Estack().PushVal(pub.Bytes())
		require.NoError(t, contractCreateStandardAccount(ic))

		value := v.Estack().Pop().Bytes()
		u, err := util.Uint160DecodeBytesBE(value)
		require.NoError(t, err)
		require.Equal(t, pub.GetScriptHash(), u)
	})
	t.Run("InvalidKey", func(t *testing.T) {
		v.Estack().PushVal([]byte{1, 2, 3})
		require.Error(t, contractCreateStandardAccount(ic))
	})
}

func TestRuntimeGasLeft(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()

	v.GasLimit = 100
	v.AddGas(58)
	require.NoError(t, runtime.GasLeft(ic))
	require.EqualValues(t, 42, v.Estack().Pop().BigInt().Int64())
}

func TestRuntimeGetNotifications(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()

	ic.Notifications = []state.NotificationEvent{
		{ScriptHash: util.Uint160{1}, Name: "Event1", Item: stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{11})})},
		{ScriptHash: util.Uint160{2}, Name: "Event2", Item: stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{22})})},
		{ScriptHash: util.Uint160{1}, Name: "Event1", Item: stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{33})})},
	}

	t.Run("NoFilter", func(t *testing.T) {
		v.Estack().PushVal(stackitem.Null{})
		require.NoError(t, runtime.GetNotifications(ic))

		arr := v.Estack().Pop().Array()
		require.Equal(t, len(ic.Notifications), len(arr))
		for i := range arr {
			elem := arr[i].Value().([]stackitem.Item)
			require.Equal(t, ic.Notifications[i].ScriptHash.BytesBE(), elem[0].Value())
			name, err := stackitem.ToString(elem[1])
			require.NoError(t, err)
			require.Equal(t, ic.Notifications[i].Name, name)
			require.Equal(t, ic.Notifications[i].Item, elem[2])
		}
	})

	t.Run("WithFilter", func(t *testing.T) {
		h := util.Uint160{2}.BytesBE()
		v.Estack().PushVal(h)
		require.NoError(t, runtime.GetNotifications(ic))

		arr := v.Estack().Pop().Array()
		require.Equal(t, 1, len(arr))
		elem := arr[0].Value().([]stackitem.Item)
		require.Equal(t, h, elem[0].Value())
		name, err := stackitem.ToString(elem[1])
		require.NoError(t, err)
		require.Equal(t, ic.Notifications[1].Name, name)
		require.Equal(t, ic.Notifications[1].Item, elem[2])
	})
}

func TestRuntimeGetInvocationCounter(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()

	ic.VM.Invocations[hash.Hash160([]byte{2})] = 42

	t.Run("No invocations", func(t *testing.T) {
		v.LoadScript([]byte{1})
		// do not return an error in this case.
		require.NoError(t, runtime.GetInvocationCounter(ic))
		require.EqualValues(t, 1, v.Estack().Pop().BigInt().Int64())
	})
	t.Run("NonZero", func(t *testing.T) {
		v.LoadScript([]byte{2})
		require.NoError(t, runtime.GetInvocationCounter(ic))
		require.EqualValues(t, 42, v.Estack().Pop().BigInt().Int64())
	})
}

func TestStoragePut(t *testing.T) {
	_, cs, ic, bc := createVMAndContractState(t)
	defer bc.Close()

	require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, cs))

	initVM := func(t *testing.T, key, value []byte, gas int64) {
		v := ic.SpawnVM()
		v.LoadScript(cs.NEF.Script)
		v.GasLimit = gas
		v.Estack().PushVal(value)
		v.Estack().PushVal(key)
		require.NoError(t, storageGetContext(ic))
	}

	t.Run("create, not enough gas", func(t *testing.T) {
		initVM(t, []byte{1}, []byte{2, 3}, 2*native.StoragePrice)
		err := storagePut(ic)
		require.True(t, errors.Is(err, errGasLimitExceeded), "got: %v", err)
	})

	initVM(t, []byte{4}, []byte{5, 6}, 3*native.StoragePrice)
	require.NoError(t, storagePut(ic))

	t.Run("update", func(t *testing.T) {
		t.Run("not enough gas", func(t *testing.T) {
			initVM(t, []byte{4}, []byte{5, 6, 7, 8}, native.StoragePrice)
			err := storagePut(ic)
			require.True(t, errors.Is(err, errGasLimitExceeded), "got: %v", err)
		})
		initVM(t, []byte{4}, []byte{5, 6, 7, 8}, 3*native.StoragePrice)
		require.NoError(t, storagePut(ic))
		initVM(t, []byte{4}, []byte{5, 6}, native.StoragePrice)
		require.NoError(t, storagePut(ic))
	})

	t.Run("check limits", func(t *testing.T) {
		initVM(t, make([]byte, MaxStorageKeyLen), make([]byte, MaxStorageValueLen), -1)
		require.NoError(t, storagePut(ic))
	})

	t.Run("bad", func(t *testing.T) {
		t.Run("readonly context", func(t *testing.T) {
			initVM(t, []byte{1}, []byte{1}, -1)
			require.NoError(t, storageContextAsReadOnly(ic))
			require.Error(t, storagePut(ic))
		})
		t.Run("big key", func(t *testing.T) {
			initVM(t, make([]byte, MaxStorageKeyLen+1), []byte{1}, -1)
			require.Error(t, storagePut(ic))
		})
		t.Run("big value", func(t *testing.T) {
			initVM(t, []byte{1}, make([]byte, MaxStorageValueLen+1), -1)
			require.Error(t, storagePut(ic))
		})
		t.Run("item exists and is const", func(t *testing.T) {
			v := ic.SpawnVM()
			v.LoadScript(cs.NEF.Script)
			v.GasLimit = -1
			v.Estack().PushVal(1)
			v.Estack().PushVal("value")
			v.Estack().PushVal("key")
			require.NoError(t, storageGetContext(ic))
			require.NoError(t, storagePutEx(ic))

			v.Estack().PushVal("new")
			v.Estack().PushVal("key")
			require.NoError(t, storageGetContext(ic))
			require.Error(t, storagePut(ic))
		})
	})
}

func TestStorageDelete(t *testing.T) {
	v, cs, ic, bc := createVMAndContractState(t)
	defer bc.Close()

	require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, cs))
	v.LoadScriptWithHash(cs.NEF.Script, cs.Hash, callflag.All)
	put := func(key, value string, flag int) {
		v.Estack().PushVal(flag)
		v.Estack().PushVal(value)
		v.Estack().PushVal(key)
		require.NoError(t, storageGetContext(ic))
		require.NoError(t, storagePutEx(ic))
	}
	put("key1", "value1", 0)
	put("key2", "value2", 0)
	put("key3", "value3", 0)
	put("key4", "value4", 1)

	t.Run("good", func(t *testing.T) {
		v.Estack().PushVal("key1")
		require.NoError(t, storageGetContext(ic))
		require.NoError(t, storageDelete(ic))
	})
	t.Run("readonly context", func(t *testing.T) {
		v.Estack().PushVal("key2")
		require.NoError(t, storageGetReadOnlyContext(ic))
		require.Error(t, storageDelete(ic))
	})
	t.Run("readonly context (from normal)", func(t *testing.T) {
		v.Estack().PushVal("key3")
		require.NoError(t, storageGetContext(ic))
		require.NoError(t, storageContextAsReadOnly(ic))
		require.Error(t, storageDelete(ic))
	})
	t.Run("constant item", func(t *testing.T) {
		v.Estack().PushVal("key4")
		require.NoError(t, storageGetContext(ic))
		require.Error(t, storageDelete(ic))
	})
}

// getTestContractState returns 2 contracts second of which is allowed to call the first.
func getTestContractState(bc *Blockchain) (*state.Contract, *state.Contract) {
	mgmtHash := bc.ManagementContractHash()

	w := io.NewBufBinWriter()
	emit.Opcodes(w.BinWriter, opcode.ABORT)
	addOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.ADD, opcode.RET)
	addMultiOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.ADD, opcode.ADD, opcode.RET)
	ret7Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.PUSH7, opcode.RET)
	dropOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.DROP, opcode.RET)
	initOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.INITSSLOT, 1, opcode.PUSH3, opcode.STSFLD0, opcode.RET)
	add3Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.LDSFLD0, opcode.ADD, opcode.RET)
	invalidRetOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.PUSH1, opcode.PUSH2, opcode.RET)
	justRetOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.RET)
	verifyOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.LDSFLD0, opcode.SUB,
		opcode.CONVERT, opcode.Opcode(stackitem.BooleanT), opcode.RET)
	deployOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.JMPIF, 2+8+3)
	emit.String(w.BinWriter, "create")
	emit.Opcodes(w.BinWriter, opcode.CALL, 3+8+3, opcode.RET)
	emit.String(w.BinWriter, "update")
	emit.Opcodes(w.BinWriter, opcode.CALL, 3, opcode.RET)
	putValOff := w.Len()
	emit.String(w.BinWriter, "initial")
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetContext)
	emit.Syscall(w.BinWriter, interopnames.SystemStoragePut)
	emit.Opcodes(w.BinWriter, opcode.RET)
	getValOff := w.Len()
	emit.String(w.BinWriter, "initial")
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGetContext)
	emit.Syscall(w.BinWriter, interopnames.SystemStorageGet)
	emit.Opcodes(w.BinWriter, opcode.RET)
	onPaymentOff := w.Len()
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeGetCallingScriptHash)
	emit.Int(w.BinWriter, 4)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.String(w.BinWriter, "LastPayment")
	emit.Syscall(w.BinWriter, interopnames.SystemRuntimeNotify)
	emit.Opcodes(w.BinWriter, opcode.RET)
	updateOff := w.Len()
	emit.Int(w.BinWriter, 2)
	emit.Opcodes(w.BinWriter, opcode.PACK)
	emit.AppCallNoArgs(w.BinWriter, mgmtHash, "update", callflag.All)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	emit.Opcodes(w.BinWriter, opcode.RET)
	destroyOff := w.Len()
	emit.AppCall(w.BinWriter, mgmtHash, "destroy", callflag.All)
	emit.Opcodes(w.BinWriter, opcode.DROP)
	emit.Opcodes(w.BinWriter, opcode.RET)
	invalidStackOff := w.Len()
	emit.Opcodes(w.BinWriter, opcode.NEWARRAY0, opcode.DUP, opcode.DUP, opcode.APPEND, opcode.NEWMAP)
	emit.Syscall(w.BinWriter, interopnames.SystemIteratorCreate)
	emit.Opcodes(w.BinWriter, opcode.RET)
	callT0Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.CALLT, 0, 0, opcode.PUSH1, opcode.ADD, opcode.RET)
	callT1Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.CALLT, 1, 0, opcode.RET)
	callT2Off := w.Len()
	emit.Opcodes(w.BinWriter, opcode.CALLT, 0, 0, opcode.RET)

	script := w.Bytes()
	h := hash.Hash160(script)
	m := manifest.NewManifest("TestMain")
	m.ABI.Methods = []manifest.Method{
		{
			Name:   "add",
			Offset: addOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("addend1", smartcontract.IntegerType),
				manifest.NewParameter("addend2", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:   "add",
			Offset: addMultiOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("addend1", smartcontract.IntegerType),
				manifest.NewParameter("addend2", smartcontract.IntegerType),
				manifest.NewParameter("addend3", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "ret7",
			Offset:     ret7Off,
			Parameters: []manifest.Parameter{},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "drop",
			Offset:     dropOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       manifest.MethodInit,
			Offset:     initOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "add3",
			Offset: add3Off,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("addend", smartcontract.IntegerType),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "invalidReturn",
			Offset:     invalidRetOff,
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "justReturn",
			Offset:     justRetOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       manifest.MethodVerify,
			Offset:     verifyOff,
			ReturnType: smartcontract.BoolType,
		},
		{
			Name:   manifest.MethodDeploy,
			Offset: deployOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("isUpdate", smartcontract.BoolType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       "getValue",
			Offset:     getValOff,
			ReturnType: smartcontract.StringType,
		},
		{
			Name:   "putValue",
			Offset: putValOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("value", smartcontract.StringType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "onPayment",
			Offset: onPaymentOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("from", smartcontract.Hash160Type),
				manifest.NewParameter("amount", smartcontract.IntegerType),
				manifest.NewParameter("data", smartcontract.AnyType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "update",
			Offset: updateOff,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("nef", smartcontract.ByteArrayType),
				manifest.NewParameter("manifest", smartcontract.ByteArrayType),
			},
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       "destroy",
			Offset:     destroyOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:       "invalidStack",
			Offset:     invalidStackOff,
			ReturnType: smartcontract.VoidType,
		},
		{
			Name:   "callT0",
			Offset: callT0Off,
			Parameters: []manifest.Parameter{
				manifest.NewParameter("address", smartcontract.Hash160Type),
			},
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "callT1",
			Offset:     callT1Off,
			ReturnType: smartcontract.IntegerType,
		},
		{
			Name:       "callT2",
			Offset:     callT2Off,
			ReturnType: smartcontract.IntegerType,
		},
	}
	m.Permissions = make([]manifest.Permission, 2)
	m.Permissions[0].Contract.Type = manifest.PermissionHash
	m.Permissions[0].Contract.Value = bc.contracts.NEO.Hash
	m.Permissions[0].Methods.Add("balanceOf")

	m.Permissions[1].Contract.Type = manifest.PermissionHash
	m.Permissions[1].Contract.Value = util.Uint160{}
	m.Permissions[1].Methods.Add("method")

	cs := &state.Contract{
		Hash:     h,
		Manifest: *m,
		ID:       42,
	}
	ne, err := nef.NewFile(script)
	if err != nil {
		panic(err)
	}
	ne.Tokens = []nef.MethodToken{
		{
			Hash:       bc.contracts.NEO.Hash,
			Method:     "balanceOf",
			ParamCount: 1,
			HasReturn:  true,
			CallFlag:   callflag.ReadStates,
		},
		{
			Hash:      util.Uint160{},
			Method:    "method",
			HasReturn: true,
			CallFlag:  callflag.ReadStates,
		},
	}
	ne.Checksum = ne.CalculateChecksum()
	cs.NEF = *ne

	currScript := []byte{byte(opcode.RET)}
	m = manifest.NewManifest("TestAux")
	perm := manifest.NewPermission(manifest.PermissionHash, h)
	perm.Methods.Add("add")
	perm.Methods.Add("drop")
	perm.Methods.Add("add3")
	perm.Methods.Add("invalidReturn")
	perm.Methods.Add("justReturn")
	perm.Methods.Add("getValue")
	m.Permissions = append(m.Permissions, *perm)
	ne, err = nef.NewFile(currScript)
	if err != nil {
		panic(err)
	}

	return cs, &state.Contract{
		NEF:      *ne,
		Hash:     hash.Hash160(currScript),
		Manifest: *m,
		ID:       123,
	}
}

func loadScript(ic *interop.Context, script []byte, args ...interface{}) {
	ic.SpawnVM()
	ic.VM.LoadScriptWithFlags(script, callflag.AllowCall)
	for i := range args {
		ic.VM.Estack().PushVal(args[i])
	}
	ic.VM.GasLimit = -1
}

func loadScriptWithHashAndFlags(ic *interop.Context, script []byte, hash util.Uint160, f callflag.CallFlag, args ...interface{}) {
	ic.SpawnVM()
	ic.VM.LoadScriptWithHash(script, hash, f)
	for i := range args {
		ic.VM.Estack().PushVal(args[i])
	}
	ic.VM.GasLimit = -1
}

func TestContractCall(t *testing.T) {
	_, ic, bc := createVM(t)
	defer bc.Close()

	cs, currCs := getTestContractState(bc)
	require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, cs))
	require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, currCs))

	currScript := currCs.NEF.Script
	h := hash.Hash160(cs.NEF.Script)

	addArgs := stackitem.NewArray([]stackitem.Item{stackitem.Make(1), stackitem.Make(2)})
	t.Run("Good", func(t *testing.T) {
		t.Run("2 arguments", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(addArgs)
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("add")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, big.NewInt(3), ic.VM.Estack().Pop().Value())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
		t.Run("3 arguments", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(
				append(addArgs.Value().([]stackitem.Item), stackitem.Make(3))))
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("add")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, big.NewInt(6), ic.VM.Estack().Pop().Value())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
	})

	t.Run("CallExInvalidFlag", func(t *testing.T) {
		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(addArgs)
		ic.VM.Estack().PushVal(byte(0xFF))
		ic.VM.Estack().PushVal("add")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.Error(t, contract.Call(ic))
	})

	runInvalid := func(args ...interface{}) func(t *testing.T) {
		return func(t *testing.T) {
			loadScriptWithHashAndFlags(ic, currScript, h, callflag.All, 42)
			for i := range args {
				ic.VM.Estack().PushVal(args[i])
			}
			// interops can both return error and panic,
			// we don't care which kind of error has occurred
			require.Panics(t, func() {
				err := contract.Call(ic)
				if err != nil {
					panic(err)
				}
			})
		}
	}

	t.Run("Invalid", func(t *testing.T) {
		t.Run("Hash", runInvalid(addArgs, "add", h.BytesBE()[1:]))
		t.Run("MissingHash", runInvalid(addArgs, "add", util.Uint160{}.BytesBE()))
		t.Run("Method", runInvalid(addArgs, stackitem.NewInterop("add"), h.BytesBE()))
		t.Run("MissingMethod", runInvalid(addArgs, "sub", h.BytesBE()))
		t.Run("DisallowedMethod", runInvalid(stackitem.NewArray(nil), "ret7", h.BytesBE()))
		t.Run("Arguments", runInvalid(1, "add", h.BytesBE()))
		t.Run("NotEnoughArguments", runInvalid(
			stackitem.NewArray([]stackitem.Item{stackitem.Make(1)}), "add", h.BytesBE()))
		t.Run("TooMuchArguments", runInvalid(
			stackitem.NewArray([]stackitem.Item{
				stackitem.Make(1), stackitem.Make(2), stackitem.Make(3), stackitem.Make(4)}),
			"add", h.BytesBE()))
	})

	t.Run("ReturnValues", func(t *testing.T) {
		t.Run("Many", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(nil))
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("invalidReturn")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.Error(t, ic.VM.Run())
		})
		t.Run("Void", func(t *testing.T) {
			loadScript(ic, currScript, 42)
			ic.VM.Estack().PushVal(stackitem.NewArray(nil))
			ic.VM.Estack().PushVal(callflag.All)
			ic.VM.Estack().PushVal("justReturn")
			ic.VM.Estack().PushVal(h.BytesBE())
			require.NoError(t, contract.Call(ic))
			require.NoError(t, ic.VM.Run())
			require.Equal(t, 2, ic.VM.Estack().Len())
			require.Equal(t, stackitem.Null{}, ic.VM.Estack().Pop().Item())
			require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
		})
	})

	t.Run("IsolatedStack", func(t *testing.T) {
		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(stackitem.NewArray(nil))
		ic.VM.Estack().PushVal(callflag.All)
		ic.VM.Estack().PushVal("drop")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.NoError(t, contract.Call(ic))
		require.Error(t, ic.VM.Run())
	})

	t.Run("CallInitialize", func(t *testing.T) {
		t.Run("Directly", runInvalid(stackitem.NewArray([]stackitem.Item{}), "_initialize", h.BytesBE()))

		loadScript(ic, currScript, 42)
		ic.VM.Estack().PushVal(stackitem.NewArray([]stackitem.Item{stackitem.Make(5)}))
		ic.VM.Estack().PushVal(callflag.All)
		ic.VM.Estack().PushVal("add3")
		ic.VM.Estack().PushVal(h.BytesBE())
		require.NoError(t, contract.Call(ic))
		require.NoError(t, ic.VM.Run())
		require.Equal(t, 2, ic.VM.Estack().Len())
		require.Equal(t, big.NewInt(8), ic.VM.Estack().Pop().Value())
		require.Equal(t, big.NewInt(42), ic.VM.Estack().Pop().Value())
	})
}

func TestContractGetCallFlags(t *testing.T) {
	v, ic, bc := createVM(t)
	defer bc.Close()

	v.LoadScriptWithHash([]byte{byte(opcode.RET)}, util.Uint160{1, 2, 3}, callflag.All)
	require.NoError(t, contractGetCallFlags(ic))
	require.Equal(t, int64(callflag.All), v.Estack().Pop().Value().(*big.Int).Int64())
}

func TestRuntimeCheckWitness(t *testing.T) {
	_, ic, bc := createVM(t)
	defer bc.Close()

	script := []byte{byte(opcode.RET)}
	scriptHash := hash.Hash160(script)
	check := func(t *testing.T, ic *interop.Context, arg interface{}, shouldFail bool, expected ...bool) {
		ic.VM.Estack().PushVal(arg)
		err := runtime.CheckWitness(ic)
		if shouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, expected)
			actual, ok := ic.VM.Estack().Pop().Value().(bool)
			require.True(t, ok)
			require.Equal(t, expected[0], actual)
		}
	}
	t.Run("error", func(t *testing.T) {
		t.Run("not a hash or key", func(t *testing.T) {
			check(t, ic, []byte{1, 2, 3}, true)
		})
		t.Run("script container is not a transaction", func(t *testing.T) {
			loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
			check(t, ic, random.Uint160().BytesBE(), true)
		})
		t.Run("check scope", func(t *testing.T) {
			t.Run("CustomGroups, missing ReadStates flag", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account:       hash,
							Scopes:        transaction.CustomGroups,
							AllowedGroups: []*keys.PublicKey{},
						},
					},
				}
				ic.Container = tx
				callingScriptHash := scriptHash
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, callflag.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), callflag.AllowCall)
				check(t, ic, hash.BytesBE(), true)
			})
			t.Run("CustomGroups, unknown contract", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account:       hash,
							Scopes:        transaction.CustomGroups,
							AllowedGroups: []*keys.PublicKey{},
						},
					},
				}
				ic.Container = tx
				callingScriptHash := scriptHash
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, callflag.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), callflag.ReadStates)
				check(t, ic, hash.BytesBE(), true)
			})
		})
	})
	t.Run("positive", func(t *testing.T) {
		t.Run("calling scripthash", func(t *testing.T) {
			t.Run("hashed witness", func(t *testing.T) {
				callingScriptHash := scriptHash
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, callflag.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), callflag.All)
				check(t, ic, callingScriptHash.BytesBE(), false, true)
			})
			t.Run("keyed witness", func(t *testing.T) {
				pk, err := keys.NewPrivateKey()
				require.NoError(t, err)
				callingScriptHash := pk.PublicKey().GetScriptHash()
				loadScriptWithHashAndFlags(ic, script, callingScriptHash, callflag.All)
				ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), callflag.All)
				check(t, ic, pk.PublicKey().Bytes(), false, true)
			})
		})
		t.Run("check scope", func(t *testing.T) {
			t.Run("Global", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.Global,
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
				ic.Container = tx
				check(t, ic, hash.BytesBE(), false, true)
			})
			t.Run("CalledByEntry", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.CalledByEntry,
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
				ic.Container = tx
				check(t, ic, hash.BytesBE(), false, true)
			})
			t.Run("CustomContracts", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account:          hash,
							Scopes:           transaction.CustomContracts,
							AllowedContracts: []util.Uint160{scriptHash},
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
				ic.Container = tx
				check(t, ic, hash.BytesBE(), false, true)
			})
			t.Run("CustomGroups", func(t *testing.T) {
				t.Run("empty calling scripthash", func(t *testing.T) {
					hash := random.Uint160()
					tx := &transaction.Transaction{
						Signers: []transaction.Signer{
							{
								Account:       hash,
								Scopes:        transaction.CustomGroups,
								AllowedGroups: []*keys.PublicKey{},
							},
						},
					}
					loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
					ic.Container = tx
					check(t, ic, hash.BytesBE(), false, false)
				})
				t.Run("positive", func(t *testing.T) {
					targetHash := random.Uint160()
					pk, err := keys.NewPrivateKey()
					require.NoError(t, err)
					tx := &transaction.Transaction{
						Signers: []transaction.Signer{
							{
								Account:       targetHash,
								Scopes:        transaction.CustomGroups,
								AllowedGroups: []*keys.PublicKey{pk.PublicKey()},
							},
						},
					}
					contractScript := []byte{byte(opcode.PUSH1), byte(opcode.RET)}
					contractScriptHash := hash.Hash160(contractScript)
					ne, err := nef.NewFile(contractScript)
					require.NoError(t, err)
					contractState := &state.Contract{
						ID:   15,
						Hash: contractScriptHash,
						NEF:  *ne,
						Manifest: manifest.Manifest{
							Groups: []manifest.Group{{PublicKey: pk.PublicKey()}},
						},
					}
					require.NoError(t, bc.contracts.Management.PutContractState(ic.DAO, contractState))
					loadScriptWithHashAndFlags(ic, contractScript, contractScriptHash, callflag.All)
					ic.VM.LoadScriptWithHash([]byte{0x1}, random.Uint160(), callflag.ReadStates)
					ic.Container = tx
					check(t, ic, targetHash.BytesBE(), false, true)
				})
			})
			t.Run("bad scope", func(t *testing.T) {
				hash := random.Uint160()
				tx := &transaction.Transaction{
					Signers: []transaction.Signer{
						{
							Account: hash,
							Scopes:  transaction.None,
						},
					},
				}
				loadScriptWithHashAndFlags(ic, script, scriptHash, callflag.ReadStates)
				ic.Container = tx
				check(t, ic, hash.BytesBE(), false, false)
			})
		})
	})
}

func TestLoadToken(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	cs, _ := getTestContractState(bc)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))

	t.Run("good", func(t *testing.T) {
		aer, err := invokeContractMethod(bc, 1_00000000, cs.Hash, "callT0", neoOwner.BytesBE())
		require.NoError(t, err)
		realBalance, _ := bc.GetGoverningTokenBalance(neoOwner)
		checkResult(t, aer, stackitem.Make(realBalance.Int64()+1))
	})
	t.Run("invalid param count", func(t *testing.T) {
		aer, err := invokeContractMethod(bc, 1_00000000, cs.Hash, "callT2")
		require.NoError(t, err)
		checkFAULTState(t, aer)
	})
	t.Run("invalid contract", func(t *testing.T) {
		aer, err := invokeContractMethod(bc, 1_00000000, cs.Hash, "callT1")
		require.NoError(t, err)
		checkFAULTState(t, aer)
	})
}
