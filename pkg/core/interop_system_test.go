package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestBCGetTransaction(t *testing.T) {
	v, tx, context, chain := createVMAndTX(t)
	defer chain.Close()

	t.Run("success", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 0))
		v.Estack().PushVal(tx.Hash().BytesBE())
		err := bcGetTransaction(context, v)
		require.NoError(t, err)

		value := v.Estack().Pop().Value()
		actual, ok := value.([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, 8, len(actual))
		require.Equal(t, tx.Hash().BytesBE(), actual[0].Value().([]byte))
		require.Equal(t, int64(tx.Version), actual[1].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.Nonce), actual[2].Value().(*big.Int).Int64())
		require.Equal(t, tx.Sender.BytesBE(), actual[3].Value().([]byte))
		require.Equal(t, int64(tx.SystemFee), actual[4].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.NetworkFee), actual[5].Value().(*big.Int).Int64())
		require.Equal(t, int64(tx.ValidUntilBlock), actual[6].Value().(*big.Int).Int64())
		require.Equal(t, tx.Script, actual[7].Value().([]byte))
	})

	t.Run("isn't traceable", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 1))
		v.Estack().PushVal(tx.Hash().BytesBE())
		err := bcGetTransaction(context, v)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad hash", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsTransaction(tx, 1))
		v.Estack().PushVal(tx.Hash().BytesLE())
		err := bcGetTransaction(context, v)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})
}

func TestBCGetTransactionFromBlock(t *testing.T) {
	v, block, context, chain := createVMAndBlock(t)
	defer chain.Close()
	require.NoError(t, chain.AddBlock(chain.newBlock()))
	require.NoError(t, context.DAO.StoreAsBlock(block))

	t.Run("success", func(t *testing.T) {
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetTransactionFromBlock(context, v)
		require.NoError(t, err)

		value := v.Estack().Pop().Value()
		actual, ok := value.([]byte)
		require.True(t, ok)
		require.Equal(t, block.Transactions[0].Hash().BytesBE(), actual)
	})

	t.Run("invalid block hash", func(t *testing.T) {
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesBE()[:10])
		err := bcGetTransactionFromBlock(context, v)
		require.Error(t, err)
	})

	t.Run("isn't traceable", func(t *testing.T) {
		block.Index = 2
		require.NoError(t, context.DAO.StoreAsBlock(block))
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetTransactionFromBlock(context, v)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad block hash", func(t *testing.T) {
		block.Index = 1
		require.NoError(t, context.DAO.StoreAsBlock(block))
		v.Estack().PushVal(0)
		v.Estack().PushVal(block.Hash().BytesLE())
		err := bcGetTransactionFromBlock(context, v)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})

	t.Run("bad transaction index", func(t *testing.T) {
		require.NoError(t, context.DAO.StoreAsBlock(block))
		v.Estack().PushVal(1)
		v.Estack().PushVal(block.Hash().BytesBE())
		err := bcGetTransactionFromBlock(context, v)
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
		err := bcGetBlock(context, v)
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
		err := bcGetTransaction(context, v)
		require.NoError(t, err)

		_, ok := v.Estack().Pop().Item().(stackitem.Null)
		require.True(t, ok)
	})
}

func TestContractIsStandard(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()

	t.Run("True", func(t *testing.T) {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)

		pub := priv.PublicKey()
		err = ic.DAO.PutContractState(&state.Contract{ID: 42, Script: pub.GetVerificationScript()})
		require.NoError(t, err)

		v.Estack().PushVal(pub.GetScriptHash().BytesBE())
		require.NoError(t, contractIsStandard(ic, v))
		require.True(t, v.Estack().Pop().Bool())
	})
	t.Run("False", func(t *testing.T) {
		script := []byte{byte(opcode.PUSHT)}
		require.NoError(t, ic.DAO.PutContractState(&state.Contract{ID: 24, Script: script}))

		v.Estack().PushVal(crypto.Hash160(script).BytesBE())
		require.NoError(t, contractIsStandard(ic, v))
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
		require.NoError(t, contractCreateStandardAccount(ic, v))

		value := v.Estack().Pop().Bytes()
		u, err := util.Uint160DecodeBytesBE(value)
		require.NoError(t, err)
		require.Equal(t, pub.GetScriptHash(), u)
	})
	t.Run("InvalidKey", func(t *testing.T) {
		v.Estack().PushVal([]byte{1, 2, 3})
		require.Error(t, contractCreateStandardAccount(ic, v))
	})
}

func TestRuntimeGasLeft(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()

	v.GasLimit = 100
	v.AddGas(58)
	require.NoError(t, runtime.GasLeft(ic, v))
	require.EqualValues(t, 42, v.Estack().Pop().BigInt().Int64())
}

func TestRuntimeGetNotifications(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()

	ic.Notifications = []state.NotificationEvent{
		{ScriptHash: util.Uint160{1}, Item: stackitem.NewByteArray([]byte{11})},
		{ScriptHash: util.Uint160{2}, Item: stackitem.NewByteArray([]byte{22})},
		{ScriptHash: util.Uint160{1}, Item: stackitem.NewByteArray([]byte{33})},
	}

	t.Run("NoFilter", func(t *testing.T) {
		v.Estack().PushVal(stackitem.Null{})
		require.NoError(t, runtime.GetNotifications(ic, v))

		arr := v.Estack().Pop().Array()
		require.Equal(t, len(ic.Notifications), len(arr))
		for i := range arr {
			elem := arr[i].Value().([]stackitem.Item)
			require.Equal(t, ic.Notifications[i].ScriptHash.BytesBE(), elem[0].Value())
			require.Equal(t, ic.Notifications[i].Item, elem[1])
		}
	})

	t.Run("WithFilter", func(t *testing.T) {
		h := util.Uint160{2}.BytesBE()
		v.Estack().PushVal(h)
		require.NoError(t, runtime.GetNotifications(ic, v))

		arr := v.Estack().Pop().Array()
		require.Equal(t, 1, len(arr))
		elem := arr[0].Value().([]stackitem.Item)
		require.Equal(t, h, elem[0].Value())
		require.Equal(t, ic.Notifications[1].Item, elem[1])
	})
}

func TestRuntimeGetInvocationCounter(t *testing.T) {
	v, ic, chain := createVM(t)
	defer chain.Close()

	ic.Invocations[hash.Hash160([]byte{2})] = 42

	t.Run("Zero", func(t *testing.T) {
		v.LoadScript([]byte{1})
		require.Error(t, runtime.GetInvocationCounter(ic, v))
	})
	t.Run("NonZero", func(t *testing.T) {
		v.LoadScript([]byte{2})
		require.NoError(t, runtime.GetInvocationCounter(ic, v))
		require.EqualValues(t, 42, v.Estack().Pop().BigInt().Int64())
	})
}
