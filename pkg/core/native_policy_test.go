package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/stretchr/testify/require"
)

func TestFeePerByte(t *testing.T) {
	chain := newTestChain(t)

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetFeePerByteInternal(chain.dao)
		require.Equal(t, 1000, int(n))
	})
}

func TestExecFeeFactor(t *testing.T) {
	chain := newTestChain(t)

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetExecFeeFactorInternal(chain.dao)
		require.EqualValues(t, interop.DefaultBaseExecFee, n)
	})
}

func TestStoragePrice(t *testing.T) {
	chain := newTestChain(t)

	t.Run("get, internal method", func(t *testing.T) {
		n := chain.contracts.Policy.GetStoragePriceInternal(chain.dao)
		require.Equal(t, int64(native.DefaultStoragePrice), n)
	})
}

func TestBlockedAccounts(t *testing.T) {
	chain := newTestChain(t)
	transferTokenFromMultisigAccount(t, chain, testchain.CommitteeScriptHash(),
		chain.contracts.GAS.Hash, 100_00000000)

	t.Run("isBlocked, internal method", func(t *testing.T) {
		isBlocked := chain.contracts.Policy.IsBlockedInternal(chain.dao, random.Uint160())
		require.Equal(t, false, isBlocked)
	})
}
