package core

import (
	"math/big"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/stretchr/testify/require"
)

func TestMaxTransactionsPerBlock(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetMaxTransactionsPerBlockInternal(chain.dao)
		require.Equal(t, 512, int(n))
	})

	t.Run("get, contract method", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "getMaxTransactionsPerBlock")
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.IntegerType,
			Value: 512,
		})
		require.NoError(t, chain.persist())
	})

	t.Run("set", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setMaxTransactionsPerBlock", bigint.ToBytes(big.NewInt(1024)))
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.BoolType,
			Value: true,
		})
		require.NoError(t, chain.persist())
		n := chain.contracts.Policy.GetMaxTransactionsPerBlockInternal(chain.dao)
		require.Equal(t, 1024, int(n))
	})
}

func TestMaxBlockSize(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "getMaxBlockSize")
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.IntegerType,
			Value: 1024 * 256,
		})
		require.NoError(t, chain.persist())
	})

	t.Run("set", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setMaxBlockSize", bigint.ToBytes(big.NewInt(102400)))
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.BoolType,
			Value: true,
		})
		require.NoError(t, chain.persist())
		res, err = invokeNativePolicyMethod(chain, "getMaxBlockSize")
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.IntegerType,
			Value: 102400,
		})
		require.NoError(t, chain.persist())
	})
}

func TestFeePerByte(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetFeePerByteInternal(chain.dao)
		require.Equal(t, 1000, int(n))
	})

	t.Run("get, contract method", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "getFeePerByte")
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.IntegerType,
			Value: 1000,
		})
		require.NoError(t, chain.persist())
	})

	t.Run("set", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setFeePerByte", bigint.ToBytes(big.NewInt(1024)))
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.BoolType,
			Value: true,
		})
		require.NoError(t, chain.persist())
		n := chain.contracts.Policy.GetFeePerByteInternal(chain.dao)
		require.Equal(t, 1024, int(n))
	})
}

func TestBlockedAccounts(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()
	account := util.Uint160{1, 2, 3}

	t.Run("get, internal method", func(t *testing.T) {
		accounts, err := chain.contracts.Policy.GetBlockedAccountsInternal(chain.dao)
		require.NoError(t, err)
		require.Equal(t, native.BlockedAccounts{}, accounts)
	})

	t.Run("get, contract method", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "getBlockedAccounts")
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.ArrayType,
			Value: []smartcontract.Parameter{},
		})
		require.NoError(t, chain.persist())
	})

	t.Run("block-unblock account", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "blockAccount", account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.BoolType,
			Value: true,
		})

		accounts, err := chain.contracts.Policy.GetBlockedAccountsInternal(chain.dao)
		require.NoError(t, err)
		require.Equal(t, native.BlockedAccounts{account}, accounts)
		require.NoError(t, chain.persist())

		res, err = invokeNativePolicyMethod(chain, "unblockAccount", account.BytesBE())
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.BoolType,
			Value: true,
		})

		accounts, err = chain.contracts.Policy.GetBlockedAccountsInternal(chain.dao)
		require.NoError(t, err)
		require.Equal(t, native.BlockedAccounts{}, accounts)
		require.NoError(t, chain.persist())
	})

	t.Run("double-block", func(t *testing.T) {
		// block
		res, err := invokeNativePolicyMethod(chain, "blockAccount", account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.BoolType,
			Value: true,
		})
		require.NoError(t, chain.persist())

		// double-block should fail
		res, err = invokeNativePolicyMethod(chain, "blockAccount", account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.BoolType,
			Value: false,
		})
		require.NoError(t, chain.persist())

		// unblock
		res, err = invokeNativePolicyMethod(chain, "unblockAccount", account.BytesBE())
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.BoolType,
			Value: true,
		})
		require.NoError(t, chain.persist())

		// unblock the same account should fail as we don't have it blocked
		res, err = invokeNativePolicyMethod(chain, "unblockAccount", account.BytesBE())
		checkResult(t, res, smartcontract.Parameter{
			Type:  smartcontract.BoolType,
			Value: false,
		})
		require.NoError(t, chain.persist())
	})

	t.Run("sorted", func(t *testing.T) {
		accounts := []util.Uint160{
			{2, 3, 4},
			{4, 5, 6},
			{3, 4, 5},
			{1, 2, 3},
		}
		for _, acc := range accounts {
			res, err := invokeNativePolicyMethod(chain, "blockAccount", acc.BytesBE())
			require.NoError(t, err)
			checkResult(t, res, smartcontract.Parameter{
				Type:  smartcontract.BoolType,
				Value: true,
			})
			require.NoError(t, chain.persist())
		}

		sort.Slice(accounts, func(i, j int) bool {
			return accounts[i].Less(accounts[j])
		})
		actual, err := chain.contracts.Policy.GetBlockedAccountsInternal(chain.dao)
		require.NoError(t, err)
		require.Equal(t, native.BlockedAccounts(accounts), actual)
	})
}

func invokeNativePolicyMethod(chain *Blockchain, method string, args ...interface{}) (*state.AppExecResult, error) {
	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, chain.contracts.Policy.Metadata().Hash, method, args...)
	if w.Err != nil {
		return nil, w.Err
	}
	script := w.Bytes()
	tx := transaction.New(chain.GetConfig().Magic, script, 10000000)
	validUntil := chain.blockHeight + 1
	tx.ValidUntilBlock = validUntil
	err := addSender(tx)
	if err != nil {
		return nil, err
	}
	addCosigners(tx)
	err = signTx(chain, tx)
	if err != nil {
		return nil, err
	}
	b := chain.newBlock(tx)
	err = chain.AddBlock(b)
	if err != nil {
		return nil, err
	}

	res, err := chain.GetAppExecResult(tx.Hash())
	if err != nil {
		return nil, err
	}
	return res, nil
}

func checkResult(t *testing.T, result *state.AppExecResult, expected smartcontract.Parameter) {
	require.Equal(t, "HALT", result.VMState)
	require.Equal(t, 1, len(result.Stack))
	require.Equal(t, expected.Type, result.Stack[0].Type)
	require.EqualValues(t, expected.Value, result.Stack[0].Value)
}
