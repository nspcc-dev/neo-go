package native_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/contracts"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
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

func TestNEO_GasPerBlockCache(t *testing.T) {
	testGetSetCache(t, newNeoCommitteeClient(t, 100_0000_0000), "GasPerBlock", 5*native.GASFactor)
}

func TestNEO_RegisterPrice(t *testing.T) {
	testGetSet(t, newNeoCommitteeClient(t, 100_0000_0000), "RegisterPrice", native.DefaultRegisterPrice, 1, math.MaxInt64)
}

func TestNEO_RegisterPriceCache(t *testing.T) {
	testGetSetCache(t, newNeoCommitteeClient(t, 100_0000_0000), "RegisterPrice", native.DefaultRegisterPrice)
}

func TestNEO_CandidateEvents(t *testing.T) {
	c := newNativeClient(t, nativenames.Neo)
	singleSigner := c.Signers[0].(neotest.MultiSigner).Single(0)
	cc := c.WithSigners(c.Signers[0], singleSigner)
	e := c.Executor
	pkb := singleSigner.Account().PublicKey().Bytes()

	// Register 1 -> event
	tx := cc.Invoke(t, true, "registerCandidate", pkb)
	e.CheckTxNotificationEvent(t, tx, 0, state.NotificationEvent{
		ScriptHash: c.Hash,
		Name:       "CandidateStateChanged",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(pkb),
			stackitem.NewBool(true),
			stackitem.Make(0),
		}),
	})

	// Register 2 -> no event
	tx = cc.Invoke(t, true, "registerCandidate", pkb)
	aer := e.GetTxExecResult(t, tx)
	require.Equal(t, 0, len(aer.Events))

	// Vote -> event
	tx = c.Invoke(t, true, "vote", c.Signers[0].ScriptHash().BytesBE(), pkb)
	e.CheckTxNotificationEvent(t, tx, 0, state.NotificationEvent{
		ScriptHash: c.Hash,
		Name:       "Vote",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(c.Signers[0].ScriptHash().BytesBE()),
			stackitem.Null{},
			stackitem.NewByteArray(pkb),
			stackitem.Make(100000000),
		}),
	})

	// Unregister 1 -> event
	tx = cc.Invoke(t, true, "unregisterCandidate", pkb)
	e.CheckTxNotificationEvent(t, tx, 0, state.NotificationEvent{
		ScriptHash: c.Hash,
		Name:       "CandidateStateChanged",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(pkb),
			stackitem.NewBool(false),
			stackitem.Make(100000000),
		}),
	})

	// Unregister 2 -> no event
	tx = cc.Invoke(t, true, "unregisterCandidate", pkb)
	aer = e.GetTxExecResult(t, tx)
	require.Equal(t, 0, len(aer.Events))
}

func TestNEO_Vote(t *testing.T) {
	neoCommitteeInvoker := newNeoCommitteeClient(t, 100_0000_0000)
	neoValidatorsInvoker := neoCommitteeInvoker.WithSigners(neoCommitteeInvoker.Validator)
	policyInvoker := neoCommitteeInvoker.CommitteeInvoker(neoCommitteeInvoker.NativeHash(t, nativenames.Policy))
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
	pubs := e.Chain.ComputeNextBlockValidators()
	require.Equal(t, standBySorted, keys.PublicKeys(pubs))

	// voters vote for candidates. The aim of this test is to check if voting
	// reward is proportional to the NEO balance.
	voters := make([]neotest.Signer, committeeSize+1)
	// referenceAccounts perform the same actions as voters except voting, i.e. we
	// will transfer the same amount of NEO to referenceAccounts and see how much
	// GAS they receive for NEO ownership. We need these values to be able to define
	// how much GAS voters receive for NEO ownership.
	referenceAccounts := make([]neotest.Signer, committeeSize+1)
	candidates := make([]neotest.Signer, committeeSize+1)
	for i := 0; i < committeeSize+1; i++ {
		voters[i] = e.NewAccount(t, 10_0000_0000)
		referenceAccounts[i] = e.NewAccount(t, 10_0000_0000)
		candidates[i] = e.NewAccount(t, 2000_0000_0000) // enough for one registration
	}
	txes := make([]*transaction.Transaction, 0, committeeSize*4-2)
	for i := 0; i < committeeSize+1; i++ {
		transferTx := neoValidatorsInvoker.PrepareInvoke(t, "transfer", e.Validator.ScriptHash(), voters[i].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), int64(committeeSize+1-i)*1000000, nil)
		txes = append(txes, transferTx)
		transferTx = neoValidatorsInvoker.PrepareInvoke(t, "transfer", e.Validator.ScriptHash(), referenceAccounts[i].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), int64(committeeSize+1-i)*1000000, nil)
		txes = append(txes, transferTx)
		if i > 0 {
			registerTx := neoValidatorsInvoker.WithSigners(candidates[i]).PrepareInvoke(t, "registerCandidate", candidates[i].(neotest.SingleSigner).Account().PublicKey().Bytes())
			txes = append(txes, registerTx)
			voteTx := neoValidatorsInvoker.WithSigners(voters[i]).PrepareInvoke(t, "vote", voters[i].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), candidates[i].(neotest.SingleSigner).Account().PublicKey().Bytes())
			txes = append(txes, voteTx)
		}
	}
	txes = append(txes, policyInvoker.PrepareInvoke(t, "blockAccount", candidates[len(candidates)-1].(neotest.SingleSigner).Account().ScriptHash()))
	neoValidatorsInvoker.AddNewBlock(t, txes...)
	for _, tx := range txes {
		e.CheckHalt(t, tx.Hash(), stackitem.Make(true)) // luckily, both `transfer`, `registerCandidate` and `vote` return boolean values
	}

	// We still haven't voted enough validators in.
	pubs = e.Chain.ComputeNextBlockValidators()
	require.NoError(t, err)
	require.Equal(t, standBySorted, keys.PublicKeys(pubs))

	advanceChain(t)
	pubs, err = e.Chain.GetNextBlockValidators()
	require.NoError(t, err)
	require.EqualValues(t, standBySorted, keys.PublicKeys(pubs))

	// Register and give some value to the last validator.
	txes = txes[:0]
	registerTx := neoValidatorsInvoker.WithSigners(candidates[0]).PrepareInvoke(t, "registerCandidate", candidates[0].(neotest.SingleSigner).Account().PublicKey().Bytes())
	txes = append(txes, registerTx)
	voteTx := neoValidatorsInvoker.WithSigners(voters[0]).PrepareInvoke(t, "vote", voters[0].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), candidates[0].(neotest.SingleSigner).Account().PublicKey().Bytes())
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
		sortedCandidates[i] = candidates[i].(neotest.SingleSigner).Account().PublicKey()
	}
	sort.Sort(sortedCandidates)
	require.EqualValues(t, sortedCandidates, keys.PublicKeys(pubs))

	pubs, err = neoCommitteeInvoker.Chain.GetNextBlockValidators()
	require.NoError(t, err)
	require.EqualValues(t, sortedCandidates, pubs)

	t.Run("check voter rewards", func(t *testing.T) {
		gasBalance := make([]*big.Int, len(voters)-1)
		referenceGASBalance := make([]*big.Int, len(referenceAccounts)-1)
		neoBalance := make([]*big.Int, len(voters)-1)
		txes = make([]*transaction.Transaction, 0, len(voters)-1)
		var refTxFee int64
		for i := range voters[:len(voters)-1] {
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
		for i := range voters[:len(voters)-1] {
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

	neoCommitteeInvoker.WithSigners(candidates[0]).Invoke(t, true, "unregisterCandidate", candidates[0].(neotest.SingleSigner).Account().PublicKey().Bytes())
	neoCommitteeInvoker.WithSigners(voters[0]).Invoke(t, false, "vote", voters[0].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), candidates[0].(neotest.SingleSigner).Account().PublicKey().Bytes())

	advanceChain(t)

	pubs = e.Chain.ComputeNextBlockValidators()
	for i := range pubs {
		require.NotEqual(t, candidates[0], pubs[i])
		require.NotEqual(t, candidates[len(candidates)-1], pubs[i])
	}
}

// TestNEO_RecursiveDistribution is a test for https://github.com/nspcc-dev/neo-go/pull/2181.
func TestNEO_RecursiveGASMint(t *testing.T) {
	neoCommitteeInvoker := newNeoCommitteeClient(t, 100_0000_0000)
	neoValidatorInvoker := neoCommitteeInvoker.WithSigners(neoCommitteeInvoker.Validator)
	e := neoCommitteeInvoker.Executor
	gasValidatorInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Gas))

	c := neotest.CompileFile(t, e.Validator.ScriptHash(), "../../../../internal/basicchain/testdata/test_contract.go", "../../../../internal/basicchain/testdata/test_contract.yml")
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

	cfg := e.Chain.GetConfig()
	committeeSize := cfg.GetCommitteeSize(0)
	validatorSize := cfg.GetNumOfCNs(0)
	advanceChain := func(t *testing.T) {
		for i := 0; i < committeeSize; i++ {
			neoValidatorInvoker.AddNewBlock(t)
		}
	}

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
			stackitem.Make(0),
		}), "getAccountState", acc.ScriptHash())
	})

	t.Run("lastGasPerVote", func(t *testing.T) {
		const (
			GasPerBlock      = 5
			VoterRewardRatio = 80
		)
		getAccountState := func(t *testing.T, account util.Uint160) *state.NEOBalance {
			stack, err := neoValidatorInvoker.TestInvoke(t, "getAccountState", account)
			require.NoError(t, err)
			as := new(state.NEOBalance)
			err = as.FromStackItem(stack.Pop().Item())
			require.NoError(t, err)
			return as
		}

		amount := int64(1000)
		acc := e.NewAccount(t)
		neoValidatorInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), acc.ScriptHash(), amount, nil)
		as := getAccountState(t, acc.ScriptHash())
		require.Equal(t, uint64(amount), as.Balance.Uint64())
		require.Equal(t, e.Chain.BlockHeight(), as.BalanceHeight)
		require.Equal(t, uint64(0), as.LastGasPerVote.Uint64())
		committee, _ := e.Chain.GetCommittee()
		neoValidatorInvoker.WithSigners(e.Validator, e.Validator.(neotest.MultiSigner).Single(0)).Invoke(t, true, "registerCandidate", committee[0].Bytes())
		neoValidatorInvoker.WithSigners(acc).Invoke(t, true, "vote", acc.ScriptHash(), committee[0].Bytes())
		as = getAccountState(t, acc.ScriptHash())
		require.Equal(t, uint64(0), as.LastGasPerVote.Uint64())
		advanceChain(t)
		neoValidatorInvoker.WithSigners(acc).Invoke(t, true, "transfer", acc.ScriptHash(), acc.ScriptHash(), amount, nil)
		as = getAccountState(t, acc.ScriptHash())
		expect := GasPerBlock * native.GASFactor * VoterRewardRatio / 100 * (uint64(e.Chain.BlockHeight()) / uint64(committeeSize))
		expect = expect * uint64(committeeSize) / uint64(validatorSize+committeeSize) * native.NEOTotalSupply / as.Balance.Uint64()
		require.Equal(t, e.Chain.BlockHeight(), as.BalanceHeight)
		require.Equal(t, expect, as.LastGasPerVote.Uint64())
	})
}

func TestNEO_GetAccountStateInteropAPI(t *testing.T) {
	neoValidatorInvoker := newNeoValidatorsClient(t)
	e := neoValidatorInvoker.Executor

	cfg := e.Chain.GetConfig()
	committeeSize := cfg.GetCommitteeSize(0)
	validatorSize := cfg.GetNumOfCNs(0)
	advanceChain := func(t *testing.T) {
		for i := 0; i < committeeSize; i++ {
			neoValidatorInvoker.AddNewBlock(t)
		}
	}

	amount := int64(1000)
	acc := e.NewAccount(t)
	neoValidatorInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), acc.ScriptHash(), amount, nil)
	committee, _ := e.Chain.GetCommittee()
	neoValidatorInvoker.WithSigners(e.Validator, e.Validator.(neotest.MultiSigner).Single(0)).Invoke(t, true, "registerCandidate", committee[0].Bytes())
	neoValidatorInvoker.WithSigners(acc).Invoke(t, true, "vote", acc.ScriptHash(), committee[0].Bytes())
	advanceChain(t)
	neoValidatorInvoker.WithSigners(acc).Invoke(t, true, "transfer", acc.ScriptHash(), acc.ScriptHash(), amount, nil)

	var hashAStr string
	for i := 0; i < util.Uint160Size; i++ {
		hashAStr += fmt.Sprintf("%#x", acc.ScriptHash()[i])
		if i != util.Uint160Size-1 {
			hashAStr += ", "
		}
	}
	src := `package testaccountstate
	  import (
		  "github.com/nspcc-dev/neo-go/pkg/interop/native/neo"
		  "github.com/nspcc-dev/neo-go/pkg/interop"
	  )
	  func GetLastGasPerVote() int {
		  accState := neo.GetAccountState(interop.Hash160{` + hashAStr + `})
		  if accState == nil {
			  panic("nil state")
		  }
		  return accState.LastGasPerVote
	  }`
	ctr := neotest.CompileSource(t, e.Validator.ScriptHash(), strings.NewReader(src), &compiler.Options{
		Name: "testaccountstate_contract",
	})
	e.DeployContract(t, ctr, nil)

	const (
		GasPerBlock      = 5
		VoterRewardRatio = 80
	)
	expect := GasPerBlock * native.GASFactor * VoterRewardRatio / 100 * (uint64(e.Chain.BlockHeight()) / uint64(committeeSize))
	expect = expect * uint64(committeeSize) / uint64(validatorSize+committeeSize) * native.NEOTotalSupply / uint64(amount)
	ctrInvoker := e.NewInvoker(ctr.Hash, e.Committee)
	ctrInvoker.Invoke(t, stackitem.Make(expect), "getLastGasPerVote")
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

	cs, _ := contracts.GetTestContractState(t, pathToInternalContracts, 1, 2, e.CommitteeHash)
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
		Name:       "LastPaymentNEP17",
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
		Name:       "LastPaymentNEP17",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(e.NativeHash(t, nativenames.Neo).BytesBE()),
			stackitem.NewByteArray(e.Validator.ScriptHash().BytesBE()),
			stackitem.NewBigInteger(big.NewInt(amount)),
			stackitem.Null{},
		}),
	})
	e.CheckTxNotificationEvent(t, h, 4, state.NotificationEvent{ // onPayment for GAS claim
		ScriptHash: cs.Hash,
		Name:       "LastPaymentNEP17",
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

// https://github.com/nspcc-dev/neo-go/issues/3190
func TestNEO_TransferNonZeroWithZeroBalance(t *testing.T) {
	neoValidatorsInvoker := newNeoValidatorsClient(t)
	e := neoValidatorsInvoker.Executor

	acc := neoValidatorsInvoker.WithSigners(e.NewAccount(t))
	accH := acc.Signers[0].ScriptHash()
	h := acc.Invoke(t, false, "transfer", accH, accH, int64(5), nil)
	aer := e.CheckHalt(t, h, stackitem.Make(false))
	require.Equal(t, 0, len(aer.Events))
	// check balance wasn't changed and height was not updated
	updatedBalance, updatedHeight := e.Chain.GetGoverningTokenBalance(accH)
	require.Equal(t, int64(0), updatedBalance.Int64())
	require.Equal(t, uint32(0), updatedHeight)
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

func TestNEO_GetCandidates(t *testing.T) {
	neoCommitteeInvoker := newNeoCommitteeClient(t, 100_0000_0000)
	neoValidatorsInvoker := neoCommitteeInvoker.WithSigners(neoCommitteeInvoker.Validator)
	policyInvoker := neoCommitteeInvoker.CommitteeInvoker(neoCommitteeInvoker.NativeHash(t, nativenames.Policy))
	e := neoCommitteeInvoker.Executor

	cfg := e.Chain.GetConfig()
	candidatesCount := cfg.GetCommitteeSize(0) - 1

	// Register a set of candidates and vote for them.
	voters := make([]neotest.Signer, candidatesCount)
	candidates := make([]neotest.Signer, candidatesCount)
	for i := 0; i < candidatesCount; i++ {
		voters[i] = e.NewAccount(t, 10_0000_0000)
		candidates[i] = e.NewAccount(t, 2000_0000_0000) // enough for one registration
	}
	txes := make([]*transaction.Transaction, 0, candidatesCount*3)
	for i := 0; i < candidatesCount; i++ {
		transferTx := neoValidatorsInvoker.PrepareInvoke(t, "transfer", e.Validator.ScriptHash(), voters[i].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), int64(candidatesCount+1-i)*1000000, nil)
		txes = append(txes, transferTx)
		registerTx := neoValidatorsInvoker.WithSigners(candidates[i]).PrepareInvoke(t, "registerCandidate", candidates[i].(neotest.SingleSigner).Account().PublicKey().Bytes())
		txes = append(txes, registerTx)
		voteTx := neoValidatorsInvoker.WithSigners(voters[i]).PrepareInvoke(t, "vote", voters[i].(neotest.SingleSigner).Account().PrivateKey().GetScriptHash(), candidates[i].(neotest.SingleSigner).Account().PublicKey().Bytes())
		txes = append(txes, voteTx)
	}

	neoValidatorsInvoker.AddNewBlock(t, txes...)
	for _, tx := range txes {
		e.CheckHalt(t, tx.Hash(), stackitem.Make(true)) // luckily, both `transfer`, `registerCandidate` and `vote` return boolean values
	}
	expected := make([]stackitem.Item, candidatesCount)
	for i := range expected {
		pub := candidates[i].(neotest.SingleSigner).Account().PublicKey().Bytes()
		v := stackitem.NewBigInteger(big.NewInt(int64(candidatesCount-i+1) * 1000000))
		expected[i] = stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray(pub),
			v,
		})
		neoCommitteeInvoker.Invoke(t, v, "getCandidateVote", pub)
	}
	sort.Slice(expected, func(i, j int) bool {
		return bytes.Compare(expected[i].Value().([]stackitem.Item)[0].Value().([]byte), expected[j].Value().([]stackitem.Item)[0].Value().([]byte)) < 0
	})
	neoCommitteeInvoker.Invoke(t, stackitem.NewArray(expected), "getCandidates")

	// Check that GetAllCandidates works the same way as GetCandidates.
	checkGetAllCandidates := func(t *testing.T, expected []stackitem.Item) {
		for i := 0; i < len(expected)+1; i++ {
			w := io.NewBufBinWriter()
			emit.AppCall(w.BinWriter, neoCommitteeInvoker.Hash, "getAllCandidates", callflag.All)
			for j := 0; j < i+1; j++ {
				emit.Opcodes(w.BinWriter, opcode.DUP)
				emit.Syscall(w.BinWriter, interopnames.SystemIteratorNext)
				emit.Opcodes(w.BinWriter, opcode.DROP) // drop the value returned from Next.
			}
			emit.Syscall(w.BinWriter, interopnames.SystemIteratorValue)
			require.NoError(t, w.Err)
			h := neoCommitteeInvoker.InvokeScript(t, w.Bytes(), neoCommitteeInvoker.Signers)
			if i < len(expected) {
				e.CheckHalt(t, h, expected[i])
			} else {
				e.CheckFault(t, h, "iterator index out of range") // ensure there are no extra elements.
			}
			w.Reset()
		}
	}
	checkGetAllCandidates(t, expected)

	// Block candidate and check it won't be returned from getCandidates and getAllCandidates.
	unlucky := candidates[len(candidates)-1].(neotest.SingleSigner).Account().PublicKey()
	policyInvoker.Invoke(t, true, "blockAccount", unlucky.GetScriptHash())
	for i := range expected {
		if bytes.Equal(expected[i].Value().([]stackitem.Item)[0].Value().([]byte), unlucky.Bytes()) {
			if i != len(expected)-1 {
				expected = append(expected[:i], expected[i+1:]...)
			} else {
				expected = expected[:i]
			}
			break
		}
	}
	neoCommitteeInvoker.Invoke(t, expected, "getCandidates")
	checkGetAllCandidates(t, expected)
}
