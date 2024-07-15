package native_test

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/stretchr/testify/require"
)

func TestPolicy_FeePerByte(t *testing.T) {
	bc, _, _ := chain.NewMulti(t)

	t.Run("get, internal method", func(t *testing.T) {
		n := bc.FeePerByte()
		require.Equal(t, 1000, int(n))
	})
}

func TestPolicy_ExecFeeFactor(t *testing.T) {
	bc, _, _ := chain.NewMulti(t)

	t.Run("get, internal method", func(t *testing.T) {
		n := bc.GetBaseExecFee()
		require.EqualValues(t, interop.DefaultBaseExecFee, n)
	})
}

func TestPolicy_StoragePrice(t *testing.T) {
	bc, validators, committee := chain.NewMulti(t)
	e := neotest.NewExecutor(t, bc, validators, committee)

	t.Run("get, internal method", func(t *testing.T) {
		e.AddNewBlock(t) // avoid default value got from Blockchain.

		n := bc.GetStoragePrice()
		require.Equal(t, int64(native.DefaultStoragePrice), n)
	})
}

func TestPolicy_BlockedAccounts(t *testing.T) {
	bc, validators, committee := chain.NewMulti(t)
	e := neotest.NewExecutor(t, bc, validators, committee)
	policyHash := e.NativeHash(t, nativenames.Policy)

	policySuperInvoker := e.NewInvoker(policyHash, validators, committee)
	unlucky := e.NewAccount(t, 5_0000_0000)
	policyUnluckyInvoker := e.NewInvoker(policyHash, unlucky)

	// Block unlucky account.
	policySuperInvoker.Invoke(t, true, "blockAccount", unlucky.ScriptHash())

	// Transaction from blocked account shouldn't be accepted.
	t.Run("isBlocked, internal method", func(t *testing.T) {
		tx := policyUnluckyInvoker.PrepareInvoke(t, "getStoragePrice")
		b := e.NewUnsignedBlock(t, tx)
		e.SignBlock(b)
		expectedErr := fmt.Sprintf("transaction %s failed to verify: not allowed by policy: account %s is blocked", tx.Hash().StringLE(), unlucky.ScriptHash().StringLE())
		err := e.Chain.AddBlock(b)
		require.Error(t, err)
		require.Equal(t, expectedErr, err.Error())
	})
}

func TestPolicy_GetNotaryFeePerKey(t *testing.T) {
	const echidnaHeight = 4
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.Hardforks = map[string]uint32{
			config.HFEchidna.String(): echidnaHeight,
		}
	})
	e := neotest.NewExecutor(t, bc, acc, acc)
	p := e.CommitteeInvoker(nativehashes.PolicyContract)

	// Invoke before Echidna should fail.
	p.InvokeFail(t, "invalid attribute type: 34", "getAttributeFee", int64(transaction.NotaryAssistedT))

	for e.Chain.BlockHeight() < echidnaHeight-1 {
		e.AddNewBlock(t)
	}

	// Invoke at Echidna should return the default value.
	p.Invoke(t, 1000_0000, "getAttributeFee", int64(transaction.NotaryAssistedT))

	// Invoke after Echidna should return the default value.
	p.Invoke(t, 1000_0000, "getAttributeFee", int64(transaction.NotaryAssistedT))
}

func TestPolicy_SetNotaryFeePerKey(t *testing.T) {
	const echidnaHeight = 4
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.Hardforks = map[string]uint32{
			config.HFEchidna.String(): echidnaHeight,
		}
	})
	e := neotest.NewExecutor(t, bc, acc, acc)
	p := e.CommitteeInvoker(nativehashes.PolicyContract)

	// Invoke before Echidna should fail.
	p.InvokeFail(t, "invalid attribute type: 34", "setAttributeFee", int64(transaction.NotaryAssistedT), 500_0000)

	for e.Chain.BlockHeight() < echidnaHeight-1 {
		e.AddNewBlock(t)
	}

	// Invoke at Echidna should return the default value.
	p.Invoke(t, nil, "setAttributeFee", int64(transaction.NotaryAssistedT), 500_0000)

	// Invoke after Echidna should return the default value.
	p.Invoke(t, nil, "setAttributeFee", int64(transaction.NotaryAssistedT), 510_0000)
}
