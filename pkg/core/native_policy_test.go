package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
		checkResult(t, res, stackitem.NewBigInteger(big.NewInt(512)))
		require.NoError(t, chain.persist())
	})

	t.Run("set", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setMaxTransactionsPerBlock", bigint.ToBytes(big.NewInt(1024)))
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		require.NoError(t, chain.persist())
		n := chain.contracts.Policy.GetMaxTransactionsPerBlockInternal(chain.dao)
		require.Equal(t, 1024, int(n))
	})

	t.Run("set, too big value", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setMaxTransactionsPerBlock", bigint.ToBytes(big.NewInt(block.MaxContentsPerBlock)))
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
}

func TestMaxBlockSize(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetMaxBlockSizeInternal(chain.dao)
		require.Equal(t, 1024*256, int(n))
	})

	t.Run("get, contract method", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "getMaxBlockSize")
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBigInteger(big.NewInt(1024*256)))
		require.NoError(t, chain.persist())
	})

	t.Run("set", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setMaxBlockSize", bigint.ToBytes(big.NewInt(102400)))
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		require.NoError(t, chain.persist())
		res, err = invokeNativePolicyMethod(chain, "getMaxBlockSize")
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBigInteger(big.NewInt(102400)))
		require.NoError(t, chain.persist())
	})

	t.Run("set, too big value", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setMaxBlockSize", bigint.ToBytes(big.NewInt(payload.MaxSize+1)))
		require.NoError(t, err)
		checkFAULTState(t, res)
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
		checkResult(t, res, stackitem.NewBigInteger(big.NewInt(1000)))
		require.NoError(t, chain.persist())
	})

	t.Run("set", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setFeePerByte", bigint.ToBytes(big.NewInt(1024)))
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		require.NoError(t, chain.persist())
		n := chain.contracts.Policy.GetFeePerByteInternal(chain.dao)
		require.Equal(t, 1024, int(n))
	})

	t.Run("set, negative value", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setFeePerByte", bigint.ToBytes(big.NewInt(-1)))
		require.NoError(t, err)
		checkFAULTState(t, res)
	})

	t.Run("set, too big value", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setFeePerByte", bigint.ToBytes(big.NewInt(100_000_000+1)))
		require.NoError(t, err)
		checkFAULTState(t, res)
	})
}

func TestBlockSystemFee(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetMaxBlockSystemFeeInternal(chain.dao)
		require.Equal(t, 9000*native.GASFactor, int(n))
	})

	t.Run("get", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "getMaxBlockSystemFee")
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBigInteger(big.NewInt(9000*native.GASFactor)))
		require.NoError(t, chain.persist())
	})

	t.Run("set, too low fee", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setMaxBlockSystemFee", bigint.ToBytes(big.NewInt(4007600)))
		require.NoError(t, err)
		checkFAULTState(t, res)
	})

	t.Run("set, success", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "setMaxBlockSystemFee", bigint.ToBytes(big.NewInt(100000000)))
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		require.NoError(t, chain.persist())
		res, err = invokeNativePolicyMethod(chain, "getMaxBlockSystemFee")
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBigInteger(big.NewInt(100000000)))
		require.NoError(t, chain.persist())
	})
}

func TestBlockedAccounts(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()
	account := util.Uint160{1, 2, 3}

	t.Run("isBlocked, internal method", func(t *testing.T) {
		isBlocked := chain.contracts.Policy.IsBlockedInternal(chain.dao, random.Uint160())
		require.Equal(t, false, isBlocked)
	})

	t.Run("isBlocked, contract method", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "isBlocked", random.Uint160())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(false))
		require.NoError(t, chain.persist())
	})

	t.Run("block-unblock account", func(t *testing.T) {
		res, err := invokeNativePolicyMethod(chain, "blockAccount", account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))

		isBlocked := chain.contracts.Policy.IsBlockedInternal(chain.dao, account)
		require.Equal(t, isBlocked, true)
		require.NoError(t, chain.persist())

		res, err = invokeNativePolicyMethod(chain, "unblockAccount", account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))

		isBlocked = chain.contracts.Policy.IsBlockedInternal(chain.dao, account)
		require.Equal(t, false, isBlocked)
		require.NoError(t, chain.persist())
	})

	t.Run("double-block", func(t *testing.T) {
		// block
		res, err := invokeNativePolicyMethod(chain, "blockAccount", account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		require.NoError(t, chain.persist())

		// double-block should fail
		res, err = invokeNativePolicyMethod(chain, "blockAccount", account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(false))
		require.NoError(t, chain.persist())

		// unblock
		res, err = invokeNativePolicyMethod(chain, "unblockAccount", account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		require.NoError(t, chain.persist())

		// unblock the same account should fail as we don't have it blocked
		res, err = invokeNativePolicyMethod(chain, "unblockAccount", account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(false))
		require.NoError(t, chain.persist())
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
	addSigners(tx)
	err := testchain.SignTx(chain, tx)
	if err != nil {
		return nil, err
	}
	b := chain.newBlock(tx)
	err = chain.AddBlock(b)
	if err != nil {
		return nil, err
	}

	res, err := chain.GetAppExecResults(tx.Hash(), trigger.Application)
	if err != nil {
		return nil, err
	}
	return &res[0], nil
}

func checkResult(t *testing.T, result *state.AppExecResult, expected stackitem.Item) {
	require.Equal(t, vm.HaltState, result.VMState)
	require.Equal(t, 1, len(result.Stack))
	require.Equal(t, expected, result.Stack[0])
}

func checkFAULTState(t *testing.T, result *state.AppExecResult) {
	require.Equal(t, vm.FaultState, result.VMState)
}
