package native_test

import (
	"encoding/json"
	"math"
	"math/big"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newNeoCommitteeClient(t *testing.T, expectedGASBalance int) *neotest.ContractInvoker {
	bc, validators, committee := chain.NewMulti(t)
	e := neotest.NewExecutor(t, bc, validators, committee)

	if expectedGASBalance > 0 {
		e.ValidatorInvoker(e.NativeHash(t, nativenames.Gas)).Invoke(t, true, "transfer", e.Validator.ScriptHash(), e.CommitteeHash, 100_0000_0000, nil)
	}

	return e.CommitteeInvoker(e.NativeHash(t, nativenames.Neo))
}

func newNeoValidatorsClient(t *testing.T) *neotest.ContractInvoker {
	c := newNeoCommitteeClient(t, 100_0000_0000)
	return c.ValidatorInvoker(c.NativeHash(t, nativenames.Neo))
}

func TestNEO_GasPerBlock(t *testing.T) {
	testGetSet(t, newNeoCommitteeClient(t, 100_0000_0000), "GasPerBlock", 5*native.GASFactor, 0, 10*native.GASFactor)
}

func TestNEO_RegisterPrice(t *testing.T) {
	testGetSet(t, newNeoCommitteeClient(t, 100_0000_0000), "RegisterPrice", native.DefaultRegisterPrice, 1, math.MaxInt64)
}

func TestNEO_Vote(t *testing.T) {
	neoCommitteeInvoker := newNeoCommitteeClient(t, 100_0000_0000)
	neoValidatorsInvoker := neoCommitteeInvoker.WithSigners(neoCommitteeInvoker.Validator)
	e := neoCommitteeInvoker.Executor

	cfg := e.Chain.GetConfig()
	committeeSize := cfg.GetCommitteeSize(0)
	validatorsCount := cfg.GetNumOfCNs(0)
	freq := validatorsCount + committeeSize
	advanceChain := func(t *testing.T) {
		for i := 0; i < freq; i++ {
			neoCommitteeInvoker.AddNewBlock(t)
		}
	}

	standBySorted, err := keys.NewPublicKeysFromStrings(e.Chain.GetConfig().StandbyCommittee)
	require.NoError(t, err)
	standBySorted = standBySorted[:validatorsCount]
	sort.Sort(standBySorted)
	pubs, err := e.Chain.GetValidators()
	require.NoError(t, err)
	require.Equal(t, standBySorted, keys.PublicKeys(pubs))

	// voters vote for candidates. The aim of this test is to check that voting
	// reward is proportional to the NEO balance.
	voters := make([]neotest.Signer, committeeSize)
	// referenceAccounts perform the same actions as voters except voting, i.e. we
	// will transfer the same amount of NEO to referenceAccounts and see how much
	// GAS they receive for NEO ownership. We need these values to be able to define
	// how much GAS voters receive for NEO ownership.
	referenceAccounts := make([]neotest.Signer, committeeSize)
	candidates := make([]neotest.Signer, committeeSize)
	for i := 0; i < committeeSize; i++ {
		voters[i] = e.NewAccount(t, 10_0000_0000)
		referenceAccounts[i] = e.NewAccount(t, 10_0000_0000)
		candidates[i] = e.NewAccount(t, 2000_0000_0000) // enough for one registration
	}
	txes := make([]*transaction.Transaction, 0, committeeSize*4-2)
	for i := 0; i < committeeSize; i++ {
		transferTx := neoValidatorsInvoker.PrepareInvoke(t, "transfer", e.Validator.ScriptHash(), voters[i].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), int64(committeeSize-i)*1000000, nil)
		txes = append(txes, transferTx)
		transferTx = neoValidatorsInvoker.PrepareInvoke(t, "transfer", e.Validator.ScriptHash(), referenceAccounts[i].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), int64(committeeSize-i)*1000000, nil)
		txes = append(txes, transferTx)
		if i > 0 {
			registerTx := neoValidatorsInvoker.WithSigners(candidates[i]).PrepareInvoke(t, "registerCandidate", candidates[i].(neotest.SingleSigner).Account().PrivateKey().PublicKey().Bytes())
			txes = append(txes, registerTx)
			voteTx := neoValidatorsInvoker.WithSigners(voters[i]).PrepareInvoke(t, "vote", voters[i].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), candidates[i].(neotest.SingleSigner).Account().PrivateKey().PublicKey().Bytes())
			txes = append(txes, voteTx)
		}
	}
	neoValidatorsInvoker.AddNewBlock(t, txes...)
	for _, tx := range txes {
		e.CheckHalt(t, tx.Hash(), stackitem.Make(true)) // luckily, both `transfer`, `registerCandidate` and `vote` return boolean values
	}

	// We still haven't voted enough validators in.
	pubs, err = e.Chain.GetValidators()
	require.NoError(t, err)
	require.Equal(t, standBySorted, keys.PublicKeys(pubs))

	advanceChain(t)
	pubs, err = e.Chain.GetNextBlockValidators()
	require.NoError(t, err)
	require.EqualValues(t, standBySorted, keys.PublicKeys(pubs))

	// Register and give some value to the last validator.
	txes = txes[:0]
	registerTx := neoValidatorsInvoker.WithSigners(candidates[0]).PrepareInvoke(t, "registerCandidate", candidates[0].(neotest.SingleSigner).Account().PrivateKey().PublicKey().Bytes())
	txes = append(txes, registerTx)
	voteTx := neoValidatorsInvoker.WithSigners(voters[0]).PrepareInvoke(t, "vote", voters[0].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), candidates[0].(neotest.SingleSigner).Account().PrivateKey().PublicKey().Bytes())
	txes = append(txes, voteTx)
	neoValidatorsInvoker.AddNewBlock(t, txes...)
	for _, tx := range txes {
		e.CheckHalt(t, tx.Hash(), stackitem.Make(true)) // luckily, both `transfer`, `registerCandidate` and `vote` return boolean values
	}

	advanceChain(t)
	pubs, err = neoCommitteeInvoker.Chain.GetNextBlockValidators()
	require.NoError(t, err)
	sortedCandidates := make(keys.PublicKeys, validatorsCount)
	for i := range candidates[:validatorsCount] {
		sortedCandidates[i] = candidates[i].(neotest.SingleSigner).Account().PrivateKey().PublicKey()
	}
	sort.Sort(sortedCandidates)
	require.EqualValues(t, sortedCandidates, keys.PublicKeys(pubs))

	pubs, err = neoCommitteeInvoker.Chain.GetNextBlockValidators()
	require.NoError(t, err)
	require.EqualValues(t, sortedCandidates, pubs)

	t.Run("check voter rewards", func(t *testing.T) {
		gasBalance := make([]*big.Int, len(voters))
		referenceGASBalance := make([]*big.Int, len(referenceAccounts))
		neoBalance := make([]*big.Int, len(voters))
		txes = make([]*transaction.Transaction, 0, len(voters))
		var refTxFee int64
		for i := range voters {
			h := voters[i].ScriptHash()
			refH := referenceAccounts[i].ScriptHash()
			gasBalance[i] = e.Chain.GetUtilityTokenBalance(h)
			neoBalance[i], _ = e.Chain.GetGoverningTokenBalance(h)
			referenceGASBalance[i] = e.Chain.GetUtilityTokenBalance(refH)

			tx := neoCommitteeInvoker.WithSigners(voters[i]).PrepareInvoke(t, "transfer", h.BytesBE(), h.BytesBE(), int64(1), nil)
			txes = append(txes, tx)
			tx = neoCommitteeInvoker.WithSigners(referenceAccounts[i]).PrepareInvoke(t, "transfer", refH.BytesBE(), refH.BytesBE(), int64(1), nil)
			txes = append(txes, tx)
			refTxFee = tx.SystemFee + tx.NetworkFee
		}
		neoCommitteeInvoker.AddNewBlock(t, txes...)
		for _, tx := range txes {
			e.CheckHalt(t, tx.Hash(), stackitem.Make(true))
		}

		// Define reference reward for NEO holding for each voter account.
		for i := range referenceGASBalance {
			newBalance := e.Chain.GetUtilityTokenBalance(referenceAccounts[i].ScriptHash())
			referenceGASBalance[i].Sub(newBalance, referenceGASBalance[i])
			referenceGASBalance[i].Add(referenceGASBalance[i], big.NewInt(refTxFee))
		}

		// GAS increase consists of 2 parts: NEO holding + voting for committee nodes.
		// Here we check that 2-nd part exists and is proportional to the amount of NEO given.
		for i := range voters {
			newGAS := e.Chain.GetUtilityTokenBalance(voters[i].ScriptHash())
			newGAS.Sub(newGAS, gasBalance[i])
			gasForHold := referenceGASBalance[i]
			newGAS.Sub(newGAS, gasForHold)
			require.True(t, newGAS.Sign() > 0)
			gasBalance[i] = newGAS
		}
		// First account voted later than the others.
		require.Equal(t, -1, gasBalance[0].Cmp(gasBalance[1]))
		for i := 2; i < validatorsCount; i++ {
			require.Equal(t, 0, gasBalance[i].Cmp(gasBalance[1]))
		}
		require.Equal(t, 1, gasBalance[1].Cmp(gasBalance[validatorsCount]))
		for i := validatorsCount; i < committeeSize; i++ {
			require.Equal(t, 0, gasBalance[i].Cmp(gasBalance[validatorsCount]))
		}
	})

	neoCommitteeInvoker.WithSigners(candidates[0]).Invoke(t, true, "unregisterCandidate", candidates[0].(neotest.SingleSigner).Account().PrivateKey().PublicKey().Bytes())
	neoCommitteeInvoker.WithSigners(voters[0]).Invoke(t, false, "vote", voters[0].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), candidates[0].(neotest.SingleSigner).Account().PrivateKey().PublicKey().Bytes())

	advanceChain(t)

	pubs, err = e.Chain.GetValidators()
	require.NoError(t, err)
	for i := range pubs {
		require.NotEqual(t, candidates[0], pubs[i])
	}
}

// TestNEO_RecursiveDistribution is a test for https://github.com/nspcc-dev/neo-go/pull/2181.
func TestNEO_RecursiveGASMint(t *testing.T) {
	neoCommitteeInvoker := newNeoCommitteeClient(t, 100_0000_0000)
	neoValidatorInvoker := neoCommitteeInvoker.WithSigners(neoCommitteeInvoker.Validator)
	e := neoCommitteeInvoker.Executor
	gasValidatorInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Gas))

	c := neotest.CompileFile(t, e.Validator.ScriptHash(), "../../../rpc/server/testdata/test_contract.go", "../../../rpc/server/testdata/test_contract.yml")
	e.DeployContract(t, c, nil)

	gasValidatorInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), c.Hash, int64(2_0000_0000), nil)

	// Transfer 10 NEO to test contract, the contract should earn some GAS by owning this NEO.
	neoValidatorInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), c.Hash, int64(10), nil)

	// Add blocks to be able to trigger NEO transfer from contract address to owner
	// address inside onNEP17Payment (the contract starts NEO transfers from chain height = 100).
	for i := e.Chain.BlockHeight(); i < 100; i++ {
		e.AddNewBlock(t)
	}

	// Transfer 1 more NEO to the contract. Transfer will trigger onNEP17Payment. OnNEP17Payment will
	// trigger transfer of 11 NEO to the contract owner (based on the contract code). 11 NEO Transfer will
	// trigger GAS distribution. GAS transfer will trigger OnNEP17Payment one more time. The recursion
	// shouldn't occur here, because contract's balance LastUpdated height has already been updated in
	// this block.
	neoValidatorInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), c.Hash, int64(1), nil)
}

func TestNEO_GetAccountState(t *testing.T) {
	neoValidatorInvoker := newNeoValidatorsClient(t)
	e := neoValidatorInvoker.Executor

	t.Run("empty", func(t *testing.T) {
		neoValidatorInvoker.Invoke(t, stackitem.Null{}, "getAccountState", util.Uint160{})
	})

	t.Run("with funds", func(t *testing.T) {
		amount := int64(1)
		acc := e.NewAccount(t)
		neoValidatorInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), acc.ScriptHash(), amount, nil)
		lub := e.Chain.BlockHeight()
		neoValidatorInvoker.Invoke(t, stackitem.NewStruct([]stackitem.Item{
			stackitem.Make(amount),
			stackitem.Make(lub),
			stackitem.Null{},
		}), "getAccountState", acc.ScriptHash())
	})
}

func TestNEO_CommitteeBountyOnPersist(t *testing.T) {
	neoCommitteeInvoker := newNeoCommitteeClient(t, 0)
	e := neoCommitteeInvoker.Executor

	hs, err := keys.NewPublicKeysFromStrings(e.Chain.GetConfig().StandbyCommittee)
	require.NoError(t, err)
	committeeSize := len(hs)

	const singleBounty = 50000000
	bs := map[int]int64{0: singleBounty}
	checkBalances := func() {
		for i := 0; i < committeeSize; i++ {
			require.EqualValues(t, bs[i], e.Chain.GetUtilityTokenBalance(hs[i].GetScriptHash()).Int64(), i)
		}
	}
	for i := 0; i < committeeSize*2; i++ {
		e.AddNewBlock(t)
		bs[(i+1)%committeeSize] += singleBounty
		checkBalances()
	}
}

func TestNEO_TransferOnPayment(t *testing.T) {
	neoValidatorsInvoker := newNeoValidatorsClient(t)
	e := neoValidatorsInvoker.Executor
	managementValidatorsInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Management))

	cs, _ := getTestContractState(t, 1, 2, e.CommitteeHash)
	cs.Hash = state.CreateContractHash(e.Validator.ScriptHash(), cs.NEF.Checksum, cs.Manifest.Name) // set proper hash
	manifB, err := json.Marshal(cs.Manifest)
	require.NoError(t, err)
	nefB, err := cs.NEF.Bytes()
	require.NoError(t, err)
	si, err := cs.ToStackItem()
	require.NoError(t, err)
	managementValidatorsInvoker.Invoke(t, si, "deploy", nefB, manifB)

	const amount int64 = 2

	h := neoValidatorsInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), cs.Hash, amount, nil)
	aer := e.GetTxExecResult(t, h)
	require.Equal(t, 3, len(aer.Events)) // transfer + GAS claim for sender + onPayment
	e.CheckTxNotificationEvent(t, h, 1, state.NotificationEvent{
		ScriptHash: cs.Hash,
		Name:       "LastPayment",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(neoValidatorsInvoker.Hash.BytesBE()),
			stackitem.NewByteArray(e.Validator.ScriptHash().BytesBE()),
			stackitem.NewBigInteger(big.NewInt(amount)),
			stackitem.Null{},
		}),
	})

	h = neoValidatorsInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), cs.Hash, amount, nil)
	aer = e.GetTxExecResult(t, h)
	require.Equal(t, 5, len(aer.Events))                         // Now we must also have GAS claim for contract and corresponding `onPayment`.
	e.CheckTxNotificationEvent(t, h, 1, state.NotificationEvent{ // onPayment for NEO transfer
		ScriptHash: cs.Hash,
		Name:       "LastPayment",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(e.NativeHash(t, nativenames.Neo).BytesBE()),
			stackitem.NewByteArray(e.Validator.ScriptHash().BytesBE()),
			stackitem.NewBigInteger(big.NewInt(amount)),
			stackitem.Null{},
		}),
	})
	e.CheckTxNotificationEvent(t, h, 4, state.NotificationEvent{ // onPayment for GAS claim
		ScriptHash: cs.Hash,
		Name:       "LastPayment",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(e.NativeHash(t, nativenames.Gas).BytesBE()),
			stackitem.Null{},
			stackitem.NewBigInteger(big.NewInt(1)),
			stackitem.Null{},
		}),
	})
}

func TestNEO_Roundtrip(t *testing.T) {
	neoValidatorsInvoker := newNeoValidatorsClient(t)
	e := neoValidatorsInvoker.Executor
	validatorH := neoValidatorsInvoker.Validator.ScriptHash()

	initialBalance, initialHeight := e.Chain.GetGoverningTokenBalance(validatorH)
	require.NotNil(t, initialBalance)

	t.Run("bad: amount > initial balance", func(t *testing.T) {
		h := neoValidatorsInvoker.Invoke(t, false, "transfer", validatorH, validatorH, initialBalance.Int64()+1, nil)
		aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 0, len(aer[0].Events)) // failed transfer => no events
		// check balance and height were not changed
		updatedBalance, updatedHeight := e.Chain.GetGoverningTokenBalance(validatorH)
		require.Equal(t, initialBalance, updatedBalance)
		require.Equal(t, initialHeight, updatedHeight)
	})

	t.Run("good: amount == initial balance", func(t *testing.T) {
		h := neoValidatorsInvoker.Invoke(t, true, "transfer", validatorH, validatorH, initialBalance.Int64(), nil)
		aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 2, len(aer[0].Events)) // roundtrip + GAS claim
		// check balance wasn't changed and height was updated
		updatedBalance, updatedHeight := e.Chain.GetGoverningTokenBalance(validatorH)
		require.Equal(t, initialBalance, updatedBalance)
		require.Equal(t, e.Chain.BlockHeight(), updatedHeight)
	})
}

func TestNEO_TransferZeroWithZeroBalance(t *testing.T) {
	neoValidatorsInvoker := newNeoValidatorsClient(t)
	e := neoValidatorsInvoker.Executor

	check := func(t *testing.T, roundtrip bool) {
		acc := neoValidatorsInvoker.WithSigners(e.NewAccount(t))
		accH := acc.Signers[0].ScriptHash()
		to := accH
		if !roundtrip {
			to = random.Uint160()
		}
		h := acc.Invoke(t, true, "transfer", accH, to, int64(0), nil)
		aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 1, len(aer[0].Events))                                                                       // roundtrip/transfer only, no GAS claim
		require.Equal(t, stackitem.NewBigInteger(big.NewInt(0)), aer[0].Events[0].Item.Value().([]stackitem.Item)[2]) // amount is 0
		// check balance wasn't changed and height was updated
		updatedBalance, updatedHeight := e.Chain.GetGoverningTokenBalance(accH)
		require.Equal(t, int64(0), updatedBalance.Int64())
		require.Equal(t, uint32(0), updatedHeight)
	}
	t.Run("roundtrip: amount == initial balance == 0", func(t *testing.T) {
		check(t, true)
	})
	t.Run("non-roundtrip: amount == initial balance == 0", func(t *testing.T) {
		check(t, false)
	})
}

func TestNEO_TransferZeroWithNonZeroBalance(t *testing.T) {
	neoValidatorsInvoker := newNeoValidatorsClient(t)
	e := neoValidatorsInvoker.Executor

	check := func(t *testing.T, roundtrip bool) {
		acc := e.NewAccount(t)
		neoValidatorsInvoker.Invoke(t, true, "transfer", neoValidatorsInvoker.Validator.ScriptHash(), acc.ScriptHash(), int64(100), nil)
		neoAccInvoker := neoValidatorsInvoker.WithSigners(acc)
		initialBalance, _ := e.Chain.GetGoverningTokenBalance(acc.ScriptHash())
		require.True(t, initialBalance.Sign() > 0)
		to := acc.ScriptHash()
		if !roundtrip {
			to = random.Uint160()
		}
		h := neoAccInvoker.Invoke(t, true, "transfer", acc.ScriptHash(), to, int64(0), nil)

		aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
		require.NoError(t, err)
		require.Equal(t, 2, len(aer[0].Events))                                                                       // roundtrip + GAS claim
		require.Equal(t, stackitem.NewBigInteger(big.NewInt(0)), aer[0].Events[0].Item.Value().([]stackitem.Item)[2]) // amount is 0
		// check balance wasn't changed and height was updated
		updatedBalance, updatedHeight := e.Chain.GetGoverningTokenBalance(acc.ScriptHash())
		require.Equal(t, initialBalance, updatedBalance)
		require.Equal(t, e.Chain.BlockHeight(), updatedHeight)
	}
	t.Run("roundtrip", func(t *testing.T) {
		check(t, true)
	})
	t.Run("non-roundtrip", func(t *testing.T) {
		check(t, false)
	})
}

func TestNEO_CalculateBonus(t *testing.T) {
	neoCommitteeInvoker := newNeoCommitteeClient(t, 10_0000_0000)
	e := neoCommitteeInvoker.Executor
	neoValidatorsInvoker := neoCommitteeInvoker.WithSigners(e.Validator)

	acc := neoValidatorsInvoker.WithSigners(e.NewAccount(t))
	accH := acc.Signers[0].ScriptHash()
	rewardDistance := 10

	t.Run("Zero", func(t *testing.T) {
		initialGASBalance := e.Chain.GetUtilityTokenBalance(accH)
		for i := 0; i < rewardDistance; i++ {
			e.AddNewBlock(t)
		}
		// Claim GAS, but there's no NEO on the account, so no GAS should be earned.
		h := acc.Invoke(t, true, "transfer", accH, accH, 0, nil)
		claimTx, _ := e.GetTransaction(t, h)

		e.CheckGASBalance(t, accH, big.NewInt(initialGASBalance.Int64()-claimTx.SystemFee-claimTx.NetworkFee))
	})

	t.Run("Many blocks", func(t *testing.T) {
		amount := 100
		defaultGASParBlock := 5
		newGASPerBlock := 1

		initialGASBalance := e.Chain.GetUtilityTokenBalance(accH)

		// Five blocks of NEO owning with default GasPerBlockValue.
		neoValidatorsInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), accH, amount, nil)
		for i := 0; i < rewardDistance/2-2; i++ {
			e.AddNewBlock(t)
		}
		neoCommitteeInvoker.Invoke(t, stackitem.Null{}, "setGasPerBlock", newGASPerBlock*native.GASFactor)

		// Five blocks more with modified GasPerBlock value.
		for i := 0; i < rewardDistance/2; i++ {
			e.AddNewBlock(t)
		}

		// GAS claim for the last 10 blocks of NEO owning.
		h := acc.Invoke(t, true, "transfer", accH, accH, amount, nil)
		claimTx, _ := e.GetTransaction(t, h)

		firstPart := int64(amount*rewardDistance/2*defaultGASParBlock) / int64(rewardDistance)
		secondPart := int64(amount*rewardDistance/2*newGASPerBlock) / int64(rewardDistance)
		e.CheckGASBalance(t, accH, big.NewInt(initialGASBalance.Int64()-
			claimTx.SystemFee-claimTx.NetworkFee + +firstPart + secondPart))
	})
}
