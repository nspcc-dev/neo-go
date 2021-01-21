package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func transferFundsToCommittee(t *testing.T, chain *Blockchain) {
	transferTokenFromMultisigAccount(t, chain, testchain.CommitteeScriptHash(),
		chain.contracts.GAS.Hash, 100_00000000)
}

func testGetSet(t *testing.T, chain *Blockchain, hash util.Uint160, name string, defaultValue, minValue, maxValue int64) {
	getName := "get" + name
	setName := "set" + name

	transferFundsToCommittee(t, chain)
	t.Run("set, not signed by committee", func(t *testing.T) {
		signer, err := wallet.NewAccount()
		require.NoError(t, err)
		invokeRes, err := invokeContractMethodBy(t, chain, signer, hash, setName, minValue+1)
		checkResult(t, invokeRes, stackitem.NewBool(false))
	})

	t.Run("get, defult value", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, hash, getName)
		require.NoError(t, err)
		checkResult(t, res, stackitem.Make(defaultValue))
		require.NoError(t, chain.persist())
	})

	t.Run("set, too small value", func(t *testing.T) {
		res, err := invokeContractMethodGeneric(chain, 100000000, hash, setName, true, minValue-1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})

	if maxValue != 0 {
		t.Run("set, too large value", func(t *testing.T) {
			res, err := invokeContractMethodGeneric(chain, 100000000, hash, setName, true, maxValue+1)
			require.NoError(t, err)
			checkFAULTState(t, res)
		})
	}

	t.Run("set, success", func(t *testing.T) {
		// Set and get in the same block.
		txSet, err := prepareContractMethodInvokeGeneric(chain, 100000000, hash, setName, true, defaultValue+1)
		require.NoError(t, err)
		txGet1, err := prepareContractMethodInvoke(chain, 100000000, hash, getName)
		require.NoError(t, err)
		aers, err := persistBlock(chain, txSet, txGet1)
		require.NoError(t, err)
		checkResult(t, aers[0], stackitem.NewBool(true))
		checkResult(t, aers[1], stackitem.Make(defaultValue+1))
		require.NoError(t, chain.persist())

		// Get in the next block.
		res, err := invokeContractMethod(chain, 100000000, hash, getName)
		require.NoError(t, err)
		checkResult(t, res, stackitem.Make(defaultValue+1))
		require.NoError(t, chain.persist())
	})
}

func TestMaxTransactionsPerBlock(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetMaxTransactionsPerBlockInternal(chain.dao)
		require.Equal(t, 512, int(n))
	})

	testGetSet(t, chain, chain.contracts.Policy.Hash, "MaxTransactionsPerBlock", 512, 0, block.MaxTransactionsPerBlock)
}

func TestMaxBlockSize(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetMaxBlockSizeInternal(chain.dao)
		require.Equal(t, 1024*256, int(n))
	})

	testGetSet(t, chain, chain.contracts.Policy.Hash, "MaxBlockSize", 1024*256, 0, payload.MaxSize)
}

func TestFeePerByte(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetFeePerByteInternal(chain.dao)
		require.Equal(t, 1000, int(n))
	})

	testGetSet(t, chain, chain.contracts.Policy.Hash, "FeePerByte", 1000, 0, 100_000_000)
}

func TestExecFeeFactor(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetExecFeeFactorInternal(chain.dao)
		require.EqualValues(t, interop.DefaultBaseExecFee, n)
	})

	testGetSet(t, chain, chain.contracts.Policy.Hash, "ExecFeeFactor", interop.DefaultBaseExecFee, 1, 1000)
}

func TestBlockSystemFee(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetMaxBlockSystemFeeInternal(chain.dao)
		require.Equal(t, 9000*native.GASFactor, int(n))
	})

	testGetSet(t, chain, chain.contracts.Policy.Hash, "MaxBlockSystemFee", 9000*native.GASFactor, 4007600, 0)
}

func TestStoragePrice(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetStoragePriceInternal(chain.dao)
		require.Equal(t, int64(native.StoragePrice), n)
	})

	testGetSet(t, chain, chain.contracts.Policy.Hash, "StoragePrice", native.StoragePrice, 1, 10000000)
}

func TestBlockedAccounts(t *testing.T) {
	chain := newTestChain(t)
	defer chain.Close()
	account := util.Uint160{1, 2, 3}
	policyHash := chain.contracts.Policy.Metadata().Hash

	transferTokenFromMultisigAccount(t, chain, testchain.CommitteeScriptHash(),
		chain.contracts.GAS.Hash, 100_00000000)

	t.Run("isBlocked, internal method", func(t *testing.T) {
		isBlocked := chain.contracts.Policy.IsBlockedInternal(chain.dao, random.Uint160())
		require.Equal(t, false, isBlocked)
	})

	t.Run("isBlocked, contract method", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, policyHash, "isBlocked", random.Uint160())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(false))
		require.NoError(t, chain.persist())
	})

	t.Run("block-unblock account", func(t *testing.T) {
		res, err := invokeContractMethodGeneric(chain, 100000000, policyHash, "blockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))

		isBlocked := chain.contracts.Policy.IsBlockedInternal(chain.dao, account)
		require.Equal(t, isBlocked, true)
		require.NoError(t, chain.persist())

		res, err = invokeContractMethodGeneric(chain, 100000000, policyHash, "unblockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))

		isBlocked = chain.contracts.Policy.IsBlockedInternal(chain.dao, account)
		require.Equal(t, false, isBlocked)
		require.NoError(t, chain.persist())
	})

	t.Run("double-block", func(t *testing.T) {
		// block
		res, err := invokeContractMethodGeneric(chain, 100000000, policyHash, "blockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		require.NoError(t, chain.persist())

		// double-block should fail
		res, err = invokeContractMethodGeneric(chain, 100000000, policyHash, "blockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(false))
		require.NoError(t, chain.persist())

		// unblock
		res, err = invokeContractMethodGeneric(chain, 100000000, policyHash, "unblockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		require.NoError(t, chain.persist())

		// unblock the same account should fail as we don't have it blocked
		res, err = invokeContractMethodGeneric(chain, 100000000, policyHash, "unblockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(false))
		require.NoError(t, chain.persist())
	})

	t.Run("not signed by committee", func(t *testing.T) {
		signer, err := wallet.NewAccount()
		require.NoError(t, err)
		invokeRes, err := invokeContractMethodBy(t, chain, signer, policyHash, "blockAccount", account.BytesBE())
		checkResult(t, invokeRes, stackitem.NewBool(false))
	})
}
