package native_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newGasClient(t *testing.T) *neotest.ContractInvoker {
	return newNativeClient(t, nativenames.Gas)
}

func TestGAS_Roundtrip(t *testing.T) {
	c := newGasClient(t)
	e := c.Executor
	gasInvoker := c.WithSigners(c.NewAccount(t))
	owner := gasInvoker.Signers[0].ScriptHash()

	getUtilityTokenBalance := func(acc util.Uint160) (*big.Int, uint32) {
		lub, err := e.Chain.GetTokenLastUpdated(acc)
		require.NoError(t, err)
		return e.Chain.GetUtilityTokenBalance(acc), lub[e.NativeID(t, nativenames.Gas)]
	}

	initialBalance, _ := getUtilityTokenBalance(owner)
	require.NotNil(t, initialBalance)

	t.Run("bad: amount > initial balance", func(t *testing.T) {
		h := gasInvoker.Invoke(t, false, "transfer", owner, owner, initialBalance.Int64()+1, nil)
		tx, height := e.GetTransaction(t, h)
		require.Equal(t, 0, len(e.GetTxExecResult(t, h).Events)) // no events (failed transfer)
		// check balance and height were not changed
		updatedBalance, updatedHeight := getUtilityTokenBalance(owner)
		initialBalance.Sub(initialBalance, big.NewInt(tx.SystemFee+tx.NetworkFee))
		require.Equal(t, initialBalance, updatedBalance)
		require.Equal(t, height, updatedHeight)
	})

	t.Run("good: amount < initial balance", func(t *testing.T) {
		h := gasInvoker.Invoke(t, true, "transfer", owner, owner, initialBalance.Int64()-1_0000_0000, nil)
		tx, height := e.GetTransaction(t, h)
		require.Equal(t, 1, len(e.GetTxExecResult(t, h).Events)) // roundtrip
		// check balance wasn't changed and height was updated
		updatedBalance, updatedHeight := getUtilityTokenBalance(owner)
		initialBalance.Sub(initialBalance, big.NewInt(tx.SystemFee+tx.NetworkFee))
		require.Equal(t, initialBalance, updatedBalance)
		require.Equal(t, height, updatedHeight)
	})
}

func TestGAS_RewardWithP2PSigExtensionsEnabled(t *testing.T) {
	const (
		nNotaries = 2
		nKeys     = 4
	)

	bc, validator, committee := chain.NewMultiWithCustomConfig(t, func(cfg *config.Blockchain) {
		cfg.P2PSigExtensions = true
	})
	e := neotest.NewExecutor(t, bc, validator, committee)
	gasCommitteeInvoker := e.CommitteeInvoker(e.NativeHash(t, nativenames.Gas))
	notaryHash := e.NativeHash(t, nativenames.Notary)
	notaryServiceFeePerKey := e.Chain.GetNotaryServiceFeePerKey()

	// transfer funds to committee
	e.ValidatorInvoker(e.NativeHash(t, nativenames.Gas)).Invoke(t, true, "transfer", e.Validator.ScriptHash(), e.CommitteeHash, 1000_0000_0000, nil)

	// set Notary nodes and check their balance
	notaryNodes := make([]*keys.PrivateKey, nNotaries)
	notaryNodesPublicKeys := make([]any, nNotaries)
	var err error
	for i := range notaryNodes {
		notaryNodes[i], err = keys.NewPrivateKey()
		require.NoError(t, err)
		notaryNodesPublicKeys[i] = notaryNodes[i].PublicKey().Bytes()
	}
	e.CommitteeInvoker(e.NativeHash(t, nativenames.Designation)).Invoke(t, stackitem.Null{}, "designateAsRole", int(noderoles.P2PNotary), notaryNodesPublicKeys)
	for _, notaryNode := range notaryNodes {
		e.CheckGASBalance(t, notaryNode.GetScriptHash(), big.NewInt(0))
	}

	// deposit GAS for `signer` with lock until the next block
	depositAmount := 100_0000 + (2+int64(nKeys))*notaryServiceFeePerKey // sysfee + netfee of the next transaction
	gasCommitteeInvoker.Invoke(t, true, "transfer", e.CommitteeHash, notaryHash, depositAmount, []any{e.CommitteeHash, e.Chain.BlockHeight() + 1})

	// save initial GAS total supply
	getGASTS := func(t *testing.T) int64 {
		stack, err := gasCommitteeInvoker.TestInvoke(t, "totalSupply")
		require.NoError(t, err)
		return stack.Pop().Value().(*big.Int).Int64()
	}
	tsInitial := getGASTS(t)

	// send transaction with Notary contract as a sender
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 1_000_000)
	tx.Nonce = neotest.Nonce()
	tx.ValidUntilBlock = e.Chain.BlockHeight() + 1
	tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: uint8(nKeys)}})
	tx.NetworkFee = (2 + int64(nKeys)) * notaryServiceFeePerKey
	tx.Signers = []transaction.Signer{
		{
			Account: notaryHash,
			Scopes:  transaction.None,
		},
		{
			Account: e.CommitteeHash,
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

	// check balance of notaries
	e.CheckGASBalance(t, notaryHash, big.NewInt(int64(depositAmount-tx.SystemFee-tx.NetworkFee)))
	for _, notaryNode := range notaryNodes {
		e.CheckGASBalance(t, notaryNode.GetScriptHash(), big.NewInt(notaryServiceFeePerKey*(nKeys+1)/nNotaries))
	}
	tsUpdated := getGASTS(t)
	tsExpected := tsInitial + 5000_0000 - tx.SystemFee
	require.Equal(t, tsExpected, tsUpdated)
}
