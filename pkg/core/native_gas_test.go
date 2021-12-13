package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
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

func TestGAS_RewardWithP2PSigExtensionsEnabled(t *testing.T) {
	chain := newTestChain(t)
	notaryHash := chain.contracts.Notary.Hash
	gasHash := chain.contracts.GAS.Hash
	signer := testchain.MultisigScriptHash()
	var err error

	const (
		nNotaries = 2
		nKeys     = 4
	)

	// set Notary nodes and check their balance
	notaryNodes := make([]*keys.PrivateKey, nNotaries)
	notaryNodesPublicKeys := make(keys.PublicKeys, nNotaries)
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
	transferTx := transferTokenFromMultisigAccount(t, chain, notaryHash, gasHash, depositAmount, signer, int64(chain.BlockHeight()+1))
	res, err := chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))

	// save initial validators balance
	balances := make(map[int]int64, testchain.ValidatorsCount)
	for i := 0; i < testchain.ValidatorsCount; i++ {
		balances[i] = chain.GetUtilityTokenBalance(testchain.PrivateKeyByID(i).GetScriptHash()).Int64()
	}
	ic := interop.NewContext(trigger.Application, chain, chain.dao, nil, nil, nil, nil, chain.log)
	tsInitial := chain.contracts.GAS.TotalSupply(ic, nil).Value().(*big.Int).Int64()

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
	singleReward := transaction.NotaryServiceFeePerKey * (nKeys + 1) / nNotaries
	for _, notaryNode := range notaryNodesPublicKeys {
		checkBalanceOf(t, chain, notaryNode.GetScriptHash(), singleReward)
	}
	for i := 0; i < testchain.ValidatorsCount; i++ {
		newBalance := chain.GetUtilityTokenBalance(testchain.PrivateKeyByID(i).GetScriptHash()).Int64()
		expectedBalance := balances[i]
		if i == int(b.Index)%testchain.CommitteeSize() {
			// committee reward
			expectedBalance += 5000_0000
		}
		if testchain.IDToOrder(i) == int(b.PrimaryIndex) {
			// primary reward
			expectedBalance += tx.NetworkFee - int64(singleReward*nNotaries)
		}
		assert.Equal(t, expectedBalance, newBalance, i)
	}
	tsUpdated := chain.contracts.GAS.TotalSupply(ic, nil).Value().(*big.Int).Int64()
	tsExpected := tsInitial + 5000_0000 - tx.SystemFee
	require.Equal(t, tsExpected, tsUpdated)
}
