package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestGAS_Roundtrip(t *testing.T) {
	bc := newTestChain(t)

	getUtilityTokenBalance := func(bc *Blockchain, acc util.Uint160) (*big.Int, uint32) {
		lub, err := bc.GetTokenLastUpdated(acc)
		require.NoError(t, err)
		return bc.GetUtilityTokenBalance(acc), lub[bc.contracts.GAS.ID]
	}

	initialBalance, _ := getUtilityTokenBalance(bc, neoOwner)
	require.NotNil(t, initialBalance)

	t.Run("bad: amount > initial balance", func(t *testing.T) {
		tx := transferTokenFromMultisigAccountWithAssert(t, bc, neoOwner, bc.contracts.GAS.Hash, initialBalance.Int64()+1, false)
		aer, err := bc.GetAppExecResults(tx.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, aer[0].VMState, aer[0].FaultException) // transfer without assert => HALT state
		checkResult(t, &aer[0], stackitem.NewBool(false))
		require.Len(t, aer[0].Events, 0) // failed transfer => no events
		// check balance and height were not changed
		updatedBalance, updatedHeight := getUtilityTokenBalance(bc, neoOwner)
		initialBalance.Sub(initialBalance, big.NewInt(tx.SystemFee+tx.NetworkFee))
		require.Equal(t, initialBalance, updatedBalance)
		require.Equal(t, bc.BlockHeight(), updatedHeight)
	})

	t.Run("good: amount < initial balance", func(t *testing.T) {
		amount := initialBalance.Int64() - 10_00000000
		tx := transferTokenFromMultisigAccountWithAssert(t, bc, neoOwner, bc.contracts.GAS.Hash, amount, false)
		aer, err := bc.GetAppExecResults(tx.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, aer[0].VMState, aer[0].FaultException)
		checkResult(t, &aer[0], stackitem.NewBool(true))
		require.Len(t, aer[0].Events, 1) // roundtrip
		// check balance wasn't changed and height was updated
		updatedBalance, updatedHeight := getUtilityTokenBalance(bc, neoOwner)
		initialBalance.Sub(initialBalance, big.NewInt(tx.SystemFee+tx.NetworkFee))
		require.Equal(t, initialBalance, updatedBalance)
		require.Equal(t, bc.BlockHeight(), updatedHeight)
	})
}
