package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestLedgerGetTransactionHeight(t *testing.T) {
	_, tx, _, chain := createVMAndTX(t)

	ledger := chain.contracts.ByName(nativenames.Ledger).Metadata().Hash

	for i := 0; i < 13; i++ {
		require.NoError(t, chain.AddBlock(chain.newBlock()))
	}
	require.NoError(t, chain.dao.StoreAsTransaction(tx, 13, nil, nil))
	t.Run("good", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransactionHeight", tx.Hash().BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.Make(13))
	})
	t.Run("bad", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransactionHeight", tx.Hash().BytesLE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.Make(-1))
	})
	t.Run("not a hash", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransactionHeight", []byte{1})
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
}

func TestLedgerGetTransaction(t *testing.T) {
	_, tx, _, chain := createVMAndTX(t)
	ledger := chain.contracts.ByName(nativenames.Ledger).Metadata().Hash

	t.Run("success", func(t *testing.T) {
		require.NoError(t, chain.dao.StoreAsTransaction(tx, 0, nil, nil))

		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransaction", tx.Hash().BytesBE())
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, res.VMState, res.FaultException)
		require.Equal(t, 1, len(res.Stack))
		value := res.Stack[0].Value()

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
		require.NoError(t, chain.dao.StoreAsTransaction(tx, 2, nil, nil)) // block 1 is added above
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransaction", tx.Hash().BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.Null{})
	})
	t.Run("bad hash", func(t *testing.T) {
		require.NoError(t, chain.dao.StoreAsTransaction(tx, 0, nil, nil))
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransaction", tx.Hash().BytesLE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.Null{})
	})
}

func TestLedgerGetTransactionFromBlock(t *testing.T) {
	chain := newTestChain(t)
	ledger := chain.contracts.ByName(nativenames.Ledger).Metadata().Hash

	res, err := invokeContractMethod(chain, 100000000, ledger, "currentIndex") // adds a block
	require.NoError(t, err)
	checkResult(t, res, stackitem.Make(0))
	bhash := chain.GetHeaderHash(1)
	b, err := chain.GetBlock(bhash)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransactionFromBlock", bhash.BytesBE(), int64(0))
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, res.VMState, res.FaultException)
		require.Equal(t, 1, len(res.Stack))
		value := res.Stack[0].Value()

		actual, ok := value.([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, b.Transactions[0].Hash().BytesBE(), actual[0].Value().([]byte))
	})
	t.Run("bad transaction index", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransactionFromBlock", bhash.BytesBE(), int64(1))
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("invalid block hash (>int64)", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransactionFromBlock", bhash.BytesBE()[:10], int64(0))
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("invalid block hash (int64)", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransactionFromBlock", bhash.BytesBE()[:6], int64(0))
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
	t.Run("bad block hash", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransactionFromBlock", bhash.BytesLE(), int64(0))
		require.NoError(t, err)
		checkResult(t, res, stackitem.Null{})
	})
	t.Run("isn't traceable", func(t *testing.T) {
		b.Index = chain.BlockHeight() + 1
		require.NoError(t, chain.dao.StoreAsBlock(b, nil, nil, nil))
		res, err := invokeContractMethod(chain, 100000000, ledger, "getTransactionFromBlock", bhash.BytesBE(), int64(0))
		require.NoError(t, err)
		checkResult(t, res, stackitem.Null{})
	})
}

func TestLedgerGetBlock(t *testing.T) {
	chain := newTestChain(t)
	ledger := chain.contracts.ByName(nativenames.Ledger).Metadata().Hash

	bhash := chain.GetHeaderHash(0)
	res, err := invokeContractMethod(chain, 100000000, ledger, "currentHash") // adds a block
	require.NoError(t, err)
	checkResult(t, res, stackitem.Make(bhash.BytesBE()))
	bhash = chain.GetHeaderHash(1)
	b, err := chain.GetBlock(bhash)
	require.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getBlock", bhash.BytesBE())
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, res.VMState, res.FaultException)
		require.Equal(t, 1, len(res.Stack))
		value := res.Stack[0].Value()

		actual, ok := value.([]stackitem.Item)
		require.True(t, ok)
		require.Equal(t, 9, len(actual))
		require.Equal(t, b.Hash().BytesBE(), actual[0].Value().([]byte))
		require.Equal(t, int64(b.Version), actual[1].Value().(*big.Int).Int64())
		require.Equal(t, b.PrevHash.BytesBE(), actual[2].Value().([]byte))
		require.Equal(t, b.MerkleRoot.BytesBE(), actual[3].Value().([]byte))
		require.Equal(t, int64(b.Timestamp), actual[4].Value().(*big.Int).Int64())
		require.Equal(t, int64(b.Nonce), actual[5].Value().(*big.Int).Int64())
		require.Equal(t, int64(b.Index), actual[6].Value().(*big.Int).Int64())
		require.Equal(t, b.NextConsensus.BytesBE(), actual[7].Value().([]byte))
		require.Equal(t, int64(len(b.Transactions)), actual[8].Value().(*big.Int).Int64())
	})
	t.Run("bad hash", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, ledger, "getBlock", bhash.BytesLE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.Null{})
	})
	t.Run("isn't traceable", func(t *testing.T) {
		b.Index = chain.BlockHeight() + 1
		require.NoError(t, chain.dao.StoreAsBlock(b, nil, nil, nil))
		res, err := invokeContractMethod(chain, 100000000, ledger, "getBlock", bhash.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.Null{})
	})
}
