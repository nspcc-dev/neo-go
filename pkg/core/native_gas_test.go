package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestGAS_Refuel(t *testing.T) {
	bc := newTestChain(t)

	cs, _ := getTestContractState(bc)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))

	const (
		sysFee  = 10_000000
		burnFee = sysFee + 12345678
	)

	accs := []*wallet.Account{
		newAccountWithGAS(t, bc),
		newAccountWithGAS(t, bc),
	}

	t.Run("good, refuel from self", func(t *testing.T) {
		before0 := bc.GetUtilityTokenBalance(accs[0].Contract.ScriptHash())
		aer, err := invokeContractMethodGeneric(bc, sysFee, bc.contracts.GAS.Hash, "refuel",
			accs[0], accs[0].Contract.ScriptHash(), int64(burnFee))
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, aer.VMState)

		after0 := bc.GetUtilityTokenBalance(accs[0].Contract.ScriptHash())
		tx, _, _ := bc.GetTransaction(aer.Container)
		require.Equal(t, before0, new(big.Int).Add(after0, big.NewInt(tx.SystemFee+tx.NetworkFee+burnFee)))
	})

	t.Run("good, refuel from other", func(t *testing.T) {
		before0 := bc.GetUtilityTokenBalance(accs[0].Contract.ScriptHash())
		before1 := bc.GetUtilityTokenBalance(accs[1].Contract.ScriptHash())
		aer, err := invokeContractMethodGeneric(bc, sysFee, cs.Hash, "refuelGas",
			accs, accs[1].Contract.ScriptHash(), int64(burnFee), int64(burnFee))
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, aer.VMState)

		after0 := bc.GetUtilityTokenBalance(accs[0].Contract.ScriptHash())
		after1 := bc.GetUtilityTokenBalance(accs[1].Contract.ScriptHash())

		tx, _, _ := bc.GetTransaction(aer.Container)
		require.Equal(t, before0, new(big.Int).Add(after0, big.NewInt(tx.SystemFee+tx.NetworkFee)))
		require.Equal(t, before1, new(big.Int).Add(after1, big.NewInt(burnFee)))
	})

	t.Run("bad, invalid witness", func(t *testing.T) {
		aer, err := invokeContractMethodGeneric(bc, sysFee, cs.Hash, "refuelGas",
			accs, random.Uint160(), int64(1), int64(1))
		require.NoError(t, err)
		require.Equal(t, vm.FaultState, aer.VMState)
	})

	t.Run("bad, invalid GAS amount", func(t *testing.T) {
		aer, err := invokeContractMethodGeneric(bc, sysFee, cs.Hash, "refuelGas",
			accs, accs[0].Contract.ScriptHash(), int64(0), int64(1))
		require.NoError(t, err)
		require.Equal(t, vm.FaultState, aer.VMState)
	})
}

func TestGAS_Roundtrip(t *testing.T) {
	bc := newTestChain(t)

	getUtilityTokenBalance := func(bc *Blockchain, acc util.Uint160) (*big.Int, uint32) {
		bs, err := bc.dao.GetNEP17TransferInfo(acc)
		if err != nil {
			return big.NewInt(0), 0
		}
		balance := bs.LastUpdated[bc.contracts.GAS.ID]
		return &balance.Balance, balance.LastUpdatedBlock
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
