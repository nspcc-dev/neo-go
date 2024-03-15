package native_test

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/notary"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newNotaryClient(t *testing.T) *neotest.ContractInvoker {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(cfg *config.Blockchain) {
		cfg.P2PSigExtensions = true
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	return e.CommitteeInvoker(e.NativeHash(t, nativenames.Notary))
}

func TestNotary_MaxNotValidBeforeDelta(t *testing.T) {
	c := newNotaryClient(t)
	testGetSet(t, c, "MaxNotValidBeforeDelta", 140, int64(c.Chain.GetConfig().ValidatorsCount), int64(c.Chain.GetConfig().MaxValidUntilBlockIncrement/2))
}

func TestNotary_MaxNotValidBeforeDeltaCache(t *testing.T) {
	c := newNotaryClient(t)
	testGetSetCache(t, c, "MaxNotValidBeforeDelta", 140)
}

func TestNotary_Pipeline(t *testing.T) {
	notaryCommitteeInvoker := newNotaryClient(t)
	e := notaryCommitteeInvoker.Executor
	neoCommitteeInvoker := e.CommitteeInvoker(e.NativeHash(t, nativenames.Neo))
	gasCommitteeInvoker := e.CommitteeInvoker(e.NativeHash(t, nativenames.Gas))

	notaryHash := notaryCommitteeInvoker.NativeHash(t, nativenames.Notary)
	feePerKey := e.Chain.GetNotaryServiceFeePerKey()
	multisigHash := notaryCommitteeInvoker.Validator.ScriptHash() // matches committee's one for single chain
	depositLock := 100

	checkBalanceOf := func(t *testing.T, acc util.Uint160, expected int64) { // we don't have big numbers in this test, thus may use int
		notaryCommitteeInvoker.CheckGASBalance(t, acc, big.NewInt(expected))
	}

	// check Notary contract has no GAS on the account
	checkBalanceOf(t, notaryHash, 0)

	// `balanceOf`: check multisig account has no GAS on deposit
	notaryCommitteeInvoker.Invoke(t, 0, "balanceOf", multisigHash)

	// `expirationOf`: should fail to get deposit which does not exist
	notaryCommitteeInvoker.Invoke(t, 0, "expirationOf", multisigHash)

	// `lockDepositUntil`: should fail because there's no deposit
	notaryCommitteeInvoker.Invoke(t, false, "lockDepositUntil", multisigHash, int64(depositLock+1))

	// `onPayment`: bad token
	neoCommitteeInvoker.InvokeFail(t, "only GAS can be accepted for deposit", "transfer", multisigHash, notaryHash, int64(1), &notary.OnNEP17PaymentData{Till: uint32(depositLock)})

	// `onPayment`: insufficient first deposit
	gasCommitteeInvoker.InvokeFail(t, "first deposit can not be less than", "transfer", multisigHash, notaryHash, int64(2*feePerKey-1), &notary.OnNEP17PaymentData{Till: uint32(depositLock)})

	// `onPayment`: invalid `data` (missing `till` parameter)
	gasCommitteeInvoker.InvokeFail(t, "`data` parameter should be an array of 2 elements", "transfer", multisigHash, notaryHash, 2*feePerKey, []any{nil})

	// `onPayment`: invalid `data` (outdated `till` parameter)
	gasCommitteeInvoker.InvokeFail(t, "`till` shouldn't be less than the chain's height", "transfer", multisigHash, notaryHash, 2*feePerKey, &notary.OnNEP17PaymentData{})

	// `onPayment`: good
	gasCommitteeInvoker.Invoke(t, true, "transfer", multisigHash, notaryHash, 2*feePerKey, &notary.OnNEP17PaymentData{Till: uint32(depositLock)})
	checkBalanceOf(t, notaryHash, 2*feePerKey)

	// `expirationOf`: check `till` was set
	notaryCommitteeInvoker.Invoke(t, depositLock, "expirationOf", multisigHash)

	// `balanceOf`: check deposited amount for the multisig account
	notaryCommitteeInvoker.Invoke(t, 2*feePerKey, "balanceOf", multisigHash)

	// `onPayment`: good second deposit and explicit `to` paramenter
	gasCommitteeInvoker.Invoke(t, true, "transfer", multisigHash, notaryHash, feePerKey, &notary.OnNEP17PaymentData{Account: &multisigHash, Till: uint32(depositLock + 1)})
	checkBalanceOf(t, notaryHash, 3*feePerKey)

	// `balanceOf`: check deposited amount for the multisig account
	notaryCommitteeInvoker.Invoke(t, 3*feePerKey, "balanceOf", multisigHash)

	// `expirationOf`: check `till` is updated.
	notaryCommitteeInvoker.Invoke(t, depositLock+1, "expirationOf", multisigHash)

	// `onPayment`: empty payment, should fail because `till` less than the previous one
	gasCommitteeInvoker.InvokeFail(t, "`till` shouldn't be less than the previous value", "transfer", multisigHash, notaryHash, int64(0), &notary.OnNEP17PaymentData{Account: &multisigHash, Till: uint32(depositLock)})
	checkBalanceOf(t, notaryHash, 3*feePerKey)
	notaryCommitteeInvoker.Invoke(t, depositLock+1, "expirationOf", multisigHash)

	// `onPayment`: empty payment, should fail because `till` less than the chain height
	gasCommitteeInvoker.InvokeFail(t, "`till` shouldn't be less than the chain's height", "transfer", multisigHash, notaryHash, int64(0), &notary.OnNEP17PaymentData{Account: &multisigHash, Till: uint32(1)})
	checkBalanceOf(t, notaryHash, 3*feePerKey)
	notaryCommitteeInvoker.Invoke(t, depositLock+1, "expirationOf", multisigHash)

	// `onPayment`: empty payment, should successfully update `till`
	gasCommitteeInvoker.Invoke(t, true, "transfer", multisigHash, notaryHash, int64(0), &notary.OnNEP17PaymentData{Account: &multisigHash, Till: uint32(depositLock + 2)})
	checkBalanceOf(t, notaryHash, 3*feePerKey)
	notaryCommitteeInvoker.Invoke(t, depositLock+2, "expirationOf", multisigHash)

	// `lockDepositUntil`: bad witness
	notaryCommitteeInvoker.Invoke(t, false, "lockDepositUntil", util.Uint160{1, 2, 3}, int64(depositLock+3))
	notaryCommitteeInvoker.Invoke(t, depositLock+2, "expirationOf", multisigHash)

	// `lockDepositUntil`: bad `till` (less than the previous one)
	notaryCommitteeInvoker.Invoke(t, false, "lockDepositUntil", multisigHash, int64(depositLock+1))
	notaryCommitteeInvoker.Invoke(t, depositLock+2, "expirationOf", multisigHash)

	// `lockDepositUntil`: bad `till` (less than the chain's height)
	notaryCommitteeInvoker.Invoke(t, false, "lockDepositUntil", multisigHash, int64(1))
	notaryCommitteeInvoker.Invoke(t, depositLock+2, "expirationOf", multisigHash)

	// `lockDepositUntil`: good `till`
	notaryCommitteeInvoker.Invoke(t, true, "lockDepositUntil", multisigHash, int64(depositLock+3))
	notaryCommitteeInvoker.Invoke(t, depositLock+3, "expirationOf", multisigHash)

	// Create new account for the next test
	notaryAccInvoker := notaryCommitteeInvoker.WithSigners(e.NewAccount(t))
	accHash := notaryAccInvoker.Signers[0].ScriptHash()

	// `withdraw`: bad witness
	notaryAccInvoker.Invoke(t, false, "withdraw", multisigHash, accHash)
	notaryCommitteeInvoker.Invoke(t, 3*feePerKey, "balanceOf", multisigHash)

	// `withdraw`: locked deposit
	notaryCommitteeInvoker.Invoke(t, false, "withdraw", multisigHash, multisigHash)
	notaryCommitteeInvoker.Invoke(t, 3*feePerKey, "balanceOf", multisigHash)

	// `withdraw`: unlock deposit and transfer GAS back to owner
	e.GenerateNewBlocks(t, depositLock)
	notaryCommitteeInvoker.Invoke(t, true, "withdraw", multisigHash, accHash)
	notaryCommitteeInvoker.Invoke(t, 0, "balanceOf", multisigHash)
	checkBalanceOf(t, notaryHash, 0)

	// `withdraw`:  the second time it should fail, because there's no deposit left
	notaryCommitteeInvoker.Invoke(t, false, "withdraw", multisigHash, accHash)

	// `onPayment`: good first deposit to other account, should set default `till` even if other `till` value is provided
	gasCommitteeInvoker.Invoke(t, true, "transfer", multisigHash, notaryHash, 2*feePerKey, &notary.OnNEP17PaymentData{Account: &accHash, Till: uint32(math.MaxUint32 - 1)})
	checkBalanceOf(t, notaryHash, 2*feePerKey)
	notaryCommitteeInvoker.Invoke(t, 5760+e.Chain.BlockHeight()-1, "expirationOf", accHash)

	// `onPayment`: good second deposit to other account, shouldn't update `till` even if other `till` value is provided
	gasCommitteeInvoker.Invoke(t, true, "transfer", multisigHash, notaryHash, feePerKey, &notary.OnNEP17PaymentData{Account: &accHash, Till: uint32(math.MaxUint32 - 1)})
	checkBalanceOf(t, notaryHash, 3*feePerKey)
	notaryCommitteeInvoker.Invoke(t, 5760+e.Chain.BlockHeight()-3, "expirationOf", accHash)
}

func TestNotary_MaliciousWithdrawal(t *testing.T) {
	const defaultDepositDeltaTill = 5760
	notaryCommitteeInvoker := newNotaryClient(t)
	e := notaryCommitteeInvoker.Executor
	gasCommitteeInvoker := e.CommitteeInvoker(e.NativeHash(t, nativenames.Gas))

	notaryHash := notaryCommitteeInvoker.NativeHash(t, nativenames.Notary)
	feePerKey := e.Chain.GetNotaryServiceFeePerKey()
	multisigHash := notaryCommitteeInvoker.Validator.ScriptHash() // matches committee's one for single chain

	checkBalanceOf := func(t *testing.T, acc util.Uint160, expected int64) { // we don't have big numbers in this test, thus may use int
		notaryCommitteeInvoker.CheckGASBalance(t, acc, big.NewInt(expected))
	}

	// Check Notary contract has no GAS on the account.
	checkBalanceOf(t, notaryHash, 0)

	// Perform several deposits to a set of different accounts.
	count := 3
	for i := 0; i < count; i++ {
		h := random.Uint160()
		gasCommitteeInvoker.Invoke(t, true, "transfer", multisigHash, notaryHash, 2*feePerKey, &notary.OnNEP17PaymentData{Account: &h, Till: e.Chain.BlockHeight() + 2})
	}
	checkBalanceOf(t, notaryHash, int64(count)*2*feePerKey)

	// Deploy malicious contract and make Notary deposit for it.
	src := `package foo
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/native/notary"
			"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
			"github.com/nspcc-dev/neo-go/pkg/interop"
			"github.com/nspcc-dev/neo-go/pkg/interop/storage"
		)
		const (
			prefixMaxCallDepth = 0x01
			defaultCallDepth = 3
		)
		func _deploy(_ any, isUpdate bool) {
			ctx := storage.GetContext()
			storage.Put(ctx, prefixMaxCallDepth, defaultCallDepth)
		}
		func OnNEP17Payment(from interop.Hash160, amount int, data any) {
			ctx := storage.GetContext()
			depth := storage.Get(ctx, prefixMaxCallDepth).(int)
			if depth > 0 {
				storage.Put(ctx, prefixMaxCallDepth, depth-1)
				notary.Withdraw(runtime.GetExecutingScriptHash(), runtime.GetExecutingScriptHash())
			}
		}
		func Verify() bool {
			return true
		}`
	ctr := neotest.CompileSource(t, e.CommitteeHash, strings.NewReader(src), &compiler.Options{Name: "Helper", Permissions: []manifest.Permission{{
		Methods: manifest.WildStrings{},
	}}})
	e.DeployContract(t, ctr, nil)
	gasCommitteeInvoker.Invoke(t, true, "transfer", multisigHash, notaryHash, 2*feePerKey, &notary.OnNEP17PaymentData{Account: &ctr.Hash, Till: e.Chain.BlockHeight() + 2})
	checkBalanceOf(t, notaryHash, int64(count+1)*2*feePerKey)
	depositLock := e.Chain.BlockHeight() + defaultDepositDeltaTill

	// Try to perform malicious withdrawal from Notary contract. In malicious scenario the withdrawal cycle
	// will be triggered by the contract itself (i.e. by malicious caller of the malicious contract), but
	// for the test simplicity we'll use committee account.
	for e.Chain.BlockHeight() <= depositLock { // side account made Notary deposit for contract, thus, default deposit delta till is used.
		e.AddNewBlock(t)
	}
	maliciousInvoker := e.NewInvoker(notaryHash, notaryCommitteeInvoker.Signers[0], neotest.NewContractSigner(ctr.Hash, func(tx *transaction.Transaction) []any {
		return nil
	}))
	maliciousInvoker.Invoke(t, true, "withdraw", ctr.Hash, ctr.Hash)
	checkBalanceOf(t, notaryHash, int64(count)*2*feePerKey)
	gasCommitteeInvoker.CheckGASBalance(t, ctr.Hash, big.NewInt(2*feePerKey))
}

func TestNotary_NotaryNodesReward(t *testing.T) {
	checkReward := func(nKeys int, nNotaryNodes int, spendFullDeposit bool) {
		notaryCommitteeInvoker := newNotaryClient(t)
		e := notaryCommitteeInvoker.Executor
		gasCommitteeInvoker := e.CommitteeInvoker(e.NativeHash(t, nativenames.Gas))
		designationCommitteeInvoker := e.CommitteeInvoker(e.NativeHash(t, nativenames.Designation))

		notaryHash := notaryCommitteeInvoker.NativeHash(t, nativenames.Notary)
		feePerKey := e.Chain.GetNotaryServiceFeePerKey()
		multisigHash := notaryCommitteeInvoker.Validator.ScriptHash() // matches committee's one for single chain

		var err error

		// set Notary nodes and check their balance
		notaryNodes := make([]*keys.PrivateKey, nNotaryNodes)
		notaryNodesPublicKeys := make([]any, nNotaryNodes)
		for i := range notaryNodes {
			notaryNodes[i], err = keys.NewPrivateKey()
			require.NoError(t, err)
			notaryNodesPublicKeys[i] = notaryNodes[i].PublicKey().Bytes()
		}

		designationCommitteeInvoker.Invoke(t, stackitem.Null{}, "designateAsRole", int(noderoles.P2PNotary), notaryNodesPublicKeys)
		for _, notaryNode := range notaryNodes {
			e.CheckGASBalance(t, notaryNode.GetScriptHash(), big.NewInt(0))
		}

		// deposit GAS for `signer` with lock until the next block inclusively
		depositAmount := 100_0000 + (2+int64(nKeys))*feePerKey // sysfee + netfee of the next transaction
		if !spendFullDeposit {
			depositAmount += 1_0000
		}
		gasCommitteeInvoker.Invoke(t, true, "transfer", multisigHash, notaryHash, depositAmount, &notary.OnNEP17PaymentData{Account: &multisigHash, Till: e.Chain.BlockHeight() + 2})

		// send transaction with Notary contract as a sender
		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 1_000_000)
		tx.Nonce = neotest.Nonce()
		tx.ValidUntilBlock = e.Chain.BlockHeight() + 1
		tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: uint8(nKeys)}})
		tx.NetworkFee = (2 + int64(nKeys)) * feePerKey
		tx.Signers = []transaction.Signer{
			{
				Account: notaryHash,
				Scopes:  transaction.None,
			},
			{
				Account: multisigHash,
				Scopes:  transaction.None,
			},
		}
		tx.Scripts = []transaction.Witness{
			{
				InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, notaryNodes[0].SignHashable(uint32(e.Chain.GetConfig().Magic), tx)...),
			},
			{
				InvocationScript:   e.Committee.SignHashable(uint32(e.Chain.GetConfig().Magic), tx),
				VerificationScript: e.Committee.Script(),
			},
		}
		e.AddNewBlock(t, tx)

		e.CheckGASBalance(t, notaryHash, big.NewInt(int64(depositAmount-tx.SystemFee-tx.NetworkFee)))
		for _, notaryNode := range notaryNodes {
			e.CheckGASBalance(t, notaryNode.GetScriptHash(), big.NewInt(feePerKey*int64((nKeys+1))/int64(nNotaryNodes)))
		}
	}

	for _, spendDeposit := range []bool{true, false} {
		checkReward(0, 1, spendDeposit)
		checkReward(0, 2, spendDeposit)
		checkReward(1, 1, spendDeposit)
		checkReward(1, 2, spendDeposit)
		checkReward(1, 3, spendDeposit)
		checkReward(5, 1, spendDeposit)
		checkReward(5, 2, spendDeposit)
		checkReward(5, 6, spendDeposit)
		checkReward(5, 7, spendDeposit)
	}
}
