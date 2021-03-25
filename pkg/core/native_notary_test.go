package core

import (
	"math"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func TestNotaryContractPipeline(t *testing.T) {
	chain := newTestChain(t)

	notaryHash := chain.contracts.Notary.Hash
	gasHash := chain.contracts.GAS.Hash
	depositLock := 100

	transferFundsToCommittee(t, chain)

	// check Notary contract has no GAS on the account
	checkBalanceOf(t, chain, notaryHash, 0)

	// `balanceOf`: check multisig account has no GAS on deposit
	balance, err := invokeContractMethod(chain, 100000000, notaryHash, "balanceOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, balance, stackitem.Make(0))

	// `expirationOf`: should fail to get deposit which does not exist
	till, err := invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(0))

	// `lockDepositUntil`: should fail because there's no deposit
	lockDepositUntilRes, err := invokeContractMethod(chain, 100000000, notaryHash, "lockDepositUntil", testchain.MultisigScriptHash(), int64(depositLock+1))
	require.NoError(t, err)
	checkResult(t, lockDepositUntilRes, stackitem.NewBool(false))

	// `onPayment`: bad token
	transferTx := transferTokenFromMultisigAccount(t, chain, notaryHash, chain.contracts.NEO.Hash, 1, nil, int64(depositLock))
	res, err := chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkFAULTState(t, &res[0])

	// `onPayment`: insufficient first deposit
	transferTx = transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, 2*transaction.NotaryServiceFeePerKey-1, nil, int64(depositLock))
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkFAULTState(t, &res[0])

	// `onPayment`: invalid `data` (missing `till` parameter)
	transferTx = transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, 2*transaction.NotaryServiceFeePerKey-1, nil)
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkFAULTState(t, &res[0])

	// `onPayment`: good
	transferTx = transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, 2*transaction.NotaryServiceFeePerKey, nil, int64(depositLock))
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))
	checkBalanceOf(t, chain, notaryHash, 2*transaction.NotaryServiceFeePerKey)

	// `expirationOf`: check `till` was set
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(depositLock))

	// `balanceOf`: check deposited amount for the multisig account
	balance, err = invokeContractMethod(chain, 100000000, notaryHash, "balanceOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, balance, stackitem.Make(2*transaction.NotaryServiceFeePerKey))

	// `onPayment`: good second deposit and explicit `to` paramenter
	transferTx = transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, transaction.NotaryServiceFeePerKey, testchain.MultisigScriptHash(), int64(depositLock+1))
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))
	checkBalanceOf(t, chain, notaryHash, 3*transaction.NotaryServiceFeePerKey)

	// `balanceOf`: check deposited amount for the multisig account
	balance, err = invokeContractMethod(chain, 100000000, notaryHash, "balanceOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, balance, stackitem.Make(3*transaction.NotaryServiceFeePerKey))

	// `expirationOf`: check `till` is updated.
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(depositLock+1))

	// `onPayment`: empty payment, should fail because `till` less then the previous one
	transferTx = transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, 0, testchain.MultisigScriptHash(), int64(depositLock))
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkFAULTState(t, &res[0])
	checkBalanceOf(t, chain, notaryHash, 3*transaction.NotaryServiceFeePerKey)
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(depositLock+1))

	// `onPayment`: empty payment, should fail because `till` less then the chain height
	transferTx = transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, 0, testchain.MultisigScriptHash(), int64(1))
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkFAULTState(t, &res[0])
	checkBalanceOf(t, chain, notaryHash, 3*transaction.NotaryServiceFeePerKey)
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(depositLock+1))

	// `onPayment`: empty payment, should successfully update `till`
	transferTx = transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, 0, testchain.MultisigScriptHash(), int64(depositLock+2))
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))
	checkBalanceOf(t, chain, notaryHash, 3*transaction.NotaryServiceFeePerKey)
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(depositLock+2))

	// `lockDepositUntil`: bad witness
	lockDepositUntilRes, err = invokeContractMethod(chain, 100000000, notaryHash, "lockDepositUntil", util.Uint160{1, 2, 3}, int64(depositLock+5))
	require.NoError(t, err)
	checkResult(t, lockDepositUntilRes, stackitem.NewBool(false))
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(depositLock+2))

	// `lockDepositUntil`: bad `till` (less then the previous one)
	lockDepositUntilRes, err = invokeContractMethod(chain, 100000000, notaryHash, "lockDepositUntil", testchain.MultisigScriptHash(), int64(depositLock+1))
	require.NoError(t, err)
	checkResult(t, lockDepositUntilRes, stackitem.NewBool(false))
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(depositLock+2))

	// `lockDepositUntil`: bad `till` (less then the chain's height)
	lockDepositUntilRes, err = invokeContractMethod(chain, 100000000, notaryHash, "lockDepositUntil", testchain.MultisigScriptHash(), int64(1))
	require.NoError(t, err)
	checkResult(t, lockDepositUntilRes, stackitem.NewBool(false))
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(depositLock+2))

	// `lockDepositUntil`: good `till`
	lockDepositUntilRes, err = invokeContractMethod(chain, 100000000, notaryHash, "lockDepositUntil", testchain.MultisigScriptHash(), int64(depositLock+3))
	require.NoError(t, err)
	checkResult(t, lockDepositUntilRes, stackitem.NewBool(true))
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(depositLock+3))

	// transfer 1 GAS to the new account for the next test
	acc, _ := wallet.NewAccount()
	transferTx = transferTokenFromMultisigAccount(t, chain, acc.PrivateKey().PublicKey().GetScriptHash(), gasHash, 100000000)
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))

	// `withdraw`: bad witness
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, notaryHash, "withdraw", callflag.All,
		testchain.MultisigScriptHash(), acc.PrivateKey().PublicKey().GetScriptHash())
	require.NoError(t, w.Err)
	script := w.Bytes()
	withdrawTx := transaction.New(script, 10000000)
	withdrawTx.ValidUntilBlock = chain.blockHeight + 1
	withdrawTx.NetworkFee = 10000000
	withdrawTx.Signers = []transaction.Signer{
		{
			Account: acc.PrivateKey().PublicKey().GetScriptHash(),
			Scopes:  transaction.None,
		},
	}
	err = acc.SignTx(chain.GetConfig().Magic, withdrawTx)
	require.NoError(t, err)
	b := chain.newBlock(withdrawTx)
	err = chain.AddBlock(b)
	require.NoError(t, err)
	appExecRes, err := chain.GetAppExecResults(withdrawTx.Hash(), trigger.Application)
	require.NoError(t, err)
	checkResult(t, &appExecRes[0], stackitem.NewBool(false))
	balance, err = invokeContractMethod(chain, 100000000, notaryHash, "balanceOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, balance, stackitem.Make(3*transaction.NotaryServiceFeePerKey))

	// `withdraw`: locked deposit
	withdrawRes, err := invokeContractMethod(chain, 100000000, notaryHash, "withdraw", testchain.MultisigScriptHash(), acc.PrivateKey().PublicKey().GetScriptHash())
	require.NoError(t, err)
	checkResult(t, withdrawRes, stackitem.NewBool(false))
	balance, err = invokeContractMethod(chain, 100000000, notaryHash, "balanceOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, balance, stackitem.Make(3*transaction.NotaryServiceFeePerKey))

	// `withdraw`: unlock deposit and transfer GAS back to owner
	chain.genBlocks(depositLock)
	withdrawRes, err = invokeContractMethod(chain, 100000000, notaryHash, "withdraw", testchain.MultisigScriptHash(), testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, withdrawRes, stackitem.NewBool(true))
	balance, err = invokeContractMethod(chain, 100000000, notaryHash, "balanceOf", testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, balance, stackitem.Make(0))
	checkBalanceOf(t, chain, notaryHash, 0)

	// `withdraw`:  the second time it should fail, because there's no deposit left
	withdrawRes, err = invokeContractMethod(chain, 100000000, notaryHash, "withdraw", testchain.MultisigScriptHash(), testchain.MultisigScriptHash())
	require.NoError(t, err)
	checkResult(t, withdrawRes, stackitem.NewBool(false))

	// `onPayment`: good first deposit to other account, should set default `till` even if other `till` value is provided
	transferTx = transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, 2*transaction.NotaryServiceFeePerKey, acc.PrivateKey().PublicKey().GetScriptHash(), int64(math.MaxUint32-1))
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))
	checkBalanceOf(t, chain, notaryHash, 2*transaction.NotaryServiceFeePerKey)
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", acc.PrivateKey().PublicKey().GetScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(5760+chain.BlockHeight()-2))

	// `onPayment`: good second deposit to other account, shouldn't update `till` even if other `till` value is provided
	transferTx = transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, 2*transaction.NotaryServiceFeePerKey, acc.PrivateKey().PublicKey().GetScriptHash(), int64(math.MaxUint32-1))
	res, err = chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))
	checkBalanceOf(t, chain, notaryHash, 4*transaction.NotaryServiceFeePerKey)
	till, err = invokeContractMethod(chain, 100000000, notaryHash, "expirationOf", acc.PrivateKey().PublicKey().GetScriptHash())
	require.NoError(t, err)
	checkResult(t, till, stackitem.Make(5760+chain.BlockHeight()-4))
}

func TestNotaryNodesReward(t *testing.T) {
	checkReward := func(nKeys int, nNotaryNodes int, spendFullDeposit bool) {
		chain := newTestChain(t)
		notaryHash := chain.contracts.Notary.Hash
		gasHash := chain.contracts.GAS.Hash
		signer := testchain.MultisigScriptHash()
		var err error

		// set Notary nodes and check their balance
		notaryNodes := make([]*keys.PrivateKey, nNotaryNodes)
		notaryNodesPublicKeys := make(keys.PublicKeys, nNotaryNodes)
		for i := range notaryNodes {
			notaryNodes[i], err = keys.NewPrivateKey()
			require.NoError(t, err)
			notaryNodesPublicKeys[i] = notaryNodes[i].PublicKey()
		}
		chain.setNodesByRole(t, true, noderoles.P2PNotary, notaryNodesPublicKeys)
		for _, notaryNode := range notaryNodesPublicKeys {
			checkBalanceOf(t, chain, notaryNode.GetScriptHash(), 0)
		}

		// deposit GAS for `signer` with lock until the next block
		depositAmount := 100_0000 + (2+int64(nKeys))*transaction.NotaryServiceFeePerKey // sysfee + netfee of the next transaction
		if !spendFullDeposit {
			depositAmount += 1_0000
		}
		transferTx := transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, depositAmount, signer, int64(chain.BlockHeight()+1))
		res, err := chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
		require.NoError(t, err)
		require.Equal(t, vm.HaltState, res[0].VMState)
		require.Equal(t, 0, len(res[0].Stack))

		// send transaction with Notary contract as a sender
		tx := chain.newTestTx(util.Uint160{}, []byte{byte(opcode.PUSH1)})
		tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: uint8(nKeys)}})
		tx.NetworkFee = (2 + int64(nKeys)) * transaction.NotaryServiceFeePerKey
		tx.Signers = []transaction.Signer{
			{
				Account: notaryHash,
				Scopes:  transaction.None,
			},
			{
				Account: signer,
				Scopes:  transaction.None,
			},
		}
		tx.Scripts = []transaction.Witness{
			{
				InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, notaryNodes[0].SignHashable(uint32(testchain.Network()), tx)...),
			},
			{
				InvocationScript:   testchain.Sign(tx),
				VerificationScript: testchain.MultisigVerificationScript(),
			},
		}
		b := chain.newBlock(tx)
		require.NoError(t, chain.AddBlock(b))
		checkBalanceOf(t, chain, notaryHash, int(depositAmount-tx.SystemFee-tx.NetworkFee))
		for _, notaryNode := range notaryNodesPublicKeys {
			checkBalanceOf(t, chain, notaryNode.GetScriptHash(), transaction.NotaryServiceFeePerKey*(nKeys+1)/nNotaryNodes)
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

func TestMaxNotValidBeforeDelta(t *testing.T) {
	chain := newTestChain(t)

	testGetSet(t, chain, chain.contracts.Notary.Hash, "MaxNotValidBeforeDelta",
		140, int64(chain.GetConfig().ValidatorsCount), transaction.MaxValidUntilBlockIncrement/2)
}
