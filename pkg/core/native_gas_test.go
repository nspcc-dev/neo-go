package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/vm"
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
