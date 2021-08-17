package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func transferFundsToCommittee(t *testing.T, chain *Blockchain) {
	transferTokenFromMultisigAccount(t, chain, testchain.CommitteeScriptHash(),
		chain.contracts.GAS.Hash, 1000_00000000)
}

func testGetSet(t *testing.T, chain *Blockchain, hash util.Uint160, name string, defaultValue, minValue, maxValue int64) {
	getName := "get" + name
	setName := "set" + name

	transferFundsToCommittee(t, chain)
	t.Run("set, not signed by committee", func(t *testing.T) {
		signer, err := wallet.NewAccount()
		require.NoError(t, err)
		invokeRes, err := invokeContractMethodBy(t, chain, signer, hash, setName, minValue+1)
		require.NoError(t, err)
		checkFAULTState(t, invokeRes)
	})

	t.Run("get, defult value", func(t *testing.T) {
		res, err := invokeContractMethod(chain, 100000000, hash, getName)
		require.NoError(t, err)
		checkResult(t, res, stackitem.Make(defaultValue))
		_, err = chain.persist()
		require.NoError(t, err)
	})

	t.Run("set, too small value", func(t *testing.T) {
		res, err := invokeContractMethodGeneric(chain, 100000000, hash, setName, true, minValue-1)
		require.NoError(t, err)
		checkFAULTState(t, res)
	})

	if maxValue != 0 {
		t.Run("set, too large value", func(t *testing.T) {
			// use big.Int because max can be `math.MaxInt64`
			max := big.NewInt(maxValue)
			max.Add(max, big.NewInt(1))
			res, err := invokeContractMethodGeneric(chain, 100000000, hash, setName, true, max)
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
		checkResult(t, aers[0], stackitem.Null{})
		if name != "GasPerBlock" { // GasPerBlock is set on the next block
			checkResult(t, aers[1], stackitem.Make(defaultValue+1))
		}
		_, err = chain.persist()
		require.NoError(t, err)

		// Get in the next block.
		res, err := invokeContractMethod(chain, 100000000, hash, getName)
		require.NoError(t, err)
		checkResult(t, res, stackitem.Make(defaultValue+1))
		_, err = chain.persist()
		require.NoError(t, err)
	})
}

func TestFeePerByte(t *testing.T) {
	chain := newTestChain(t)

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetFeePerByteInternal(chain.dao)
		require.Equal(t, 1000, int(n))
	})

	testGetSet(t, chain, chain.contracts.Policy.Hash, "FeePerByte", 1000, 0, 100_000_000)
}

func TestExecFeeFactor(t *testing.T) {
	chain := newTestChain(t)

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetExecFeeFactorInternal(chain.dao)
		require.EqualValues(t, interop.DefaultBaseExecFee, n)
	})

	testGetSet(t, chain, chain.contracts.Policy.Hash, "ExecFeeFactor", interop.DefaultBaseExecFee, 1, 1000)
}

func TestStoragePrice(t *testing.T) {
	chain := newTestChain(t)

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetStoragePriceInternal(chain.dao)
		require.Equal(t, int64(native.DefaultStoragePrice), n)
	})

	testGetSet(t, chain, chain.contracts.Policy.Hash, "StoragePrice", native.DefaultStoragePrice, 1, 10000000)
}

func TestBlockedAccounts(t *testing.T) {
	chain := newTestChain(t)
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
		_, err = chain.persist()
		require.NoError(t, err)
	})

	t.Run("block-unblock account", func(t *testing.T) {
		res, err := invokeContractMethodGeneric(chain, 100000000, policyHash, "blockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))

		isBlocked := chain.contracts.Policy.IsBlockedInternal(chain.dao, account)
		require.Equal(t, isBlocked, true)
		_, err = chain.persist()
		require.NoError(t, err)

		res, err = invokeContractMethodGeneric(chain, 100000000, policyHash, "unblockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))

		isBlocked = chain.contracts.Policy.IsBlockedInternal(chain.dao, account)
		require.Equal(t, false, isBlocked)
		_, err = chain.persist()
		require.NoError(t, err)
	})

	t.Run("double-block", func(t *testing.T) {
		// block
		res, err := invokeContractMethodGeneric(chain, 100000000, policyHash, "blockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		_, err = chain.persist()
		require.NoError(t, err)

		// double-block should fail
		res, err = invokeContractMethodGeneric(chain, 100000000, policyHash, "blockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(false))
		_, err = chain.persist()
		require.NoError(t, err)

		// unblock
		res, err = invokeContractMethodGeneric(chain, 100000000, policyHash, "unblockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))
		_, err = chain.persist()
		require.NoError(t, err)

		// unblock the same account should fail as we don't have it blocked
		res, err = invokeContractMethodGeneric(chain, 100000000, policyHash, "unblockAccount", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(false))
		_, err = chain.persist()
		require.NoError(t, err)
	})

	t.Run("not signed by committee", func(t *testing.T) {
		signer, err := wallet.NewAccount()
		require.NoError(t, err)
		invokeRes, err := invokeContractMethodBy(t, chain, signer, policyHash, "blockAccount", account.BytesBE())
		require.NoError(t, err)
		checkFAULTState(t, invokeRes)

		invokeRes, err = invokeContractMethodBy(t, chain, signer, policyHash, "unblockAccount", account.BytesBE())
		require.NoError(t, err)
		checkFAULTState(t, invokeRes)
	})

	t.Run("block-unblock contract", func(t *testing.T) {
		neoHash := chain.contracts.NEO.Metadata().Hash
		res, err := invokeContractMethodGeneric(chain, 100000000, policyHash, "blockAccount", true, neoHash.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))

		res, err = invokeContractMethodGeneric(chain, 100000000, neoHash, "balanceOf", true, account.BytesBE())
		require.NoError(t, err)
		checkFAULTState(t, res)

		res, err = invokeContractMethodGeneric(chain, 100000000, policyHash, "unblockAccount", true, neoHash.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.NewBool(true))

		res, err = invokeContractMethodGeneric(chain, 100000000, neoHash, "balanceOf", true, account.BytesBE())
		require.NoError(t, err)
		checkResult(t, res, stackitem.Make(0))
	})
}
