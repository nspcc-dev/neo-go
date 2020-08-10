package core

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

func setSigner(tx *transaction.Transaction, h util.Uint160) {
	tx.Signers = []transaction.Signer{{
		Account: h,
		Scopes:  transaction.Global,
	}}
}

func TestNEO_Vote(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	neo := bc.contracts.NEO
	tx := transaction.New(netmode.UnitTestNet, []byte{}, 0)
	ic := bc.newInteropContext(trigger.System, bc.dao, nil, tx)
	ic.VM = vm.New()

	pubs, err := neo.GetValidatorsInternal(bc, ic.DAO)
	require.NoError(t, err)
	require.Equal(t, bc.GetStandByValidators(), pubs)

	sz := testchain.Size()

	candidates := make(keys.PublicKeys, sz)
	for i := 0; i < sz; i++ {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		candidates[i] = priv.PublicKey()
		if i > 0 {
			require.NoError(t, neo.RegisterCandidateInternal(ic, candidates[i]))
		}
	}

	for i := 0; i < sz; i++ {
		to := testchain.PrivateKeyByID(i).GetScriptHash()
		ic.VM.Load(testchain.MultisigVerificationScript())
		ic.VM.LoadScriptWithHash(testchain.MultisigVerificationScript(), testchain.MultisigScriptHash(), smartcontract.All)
		require.NoError(t, neo.TransferInternal(ic, testchain.MultisigScriptHash(), to, big.NewInt(int64(sz-i)*10000000)))
	}

	for i := 1; i < sz; i++ {
		priv := testchain.PrivateKeyByID(i)
		h := priv.GetScriptHash()
		setSigner(tx, h)
		ic.VM.Load(priv.PublicKey().GetVerificationScript())
		require.NoError(t, neo.VoteInternal(ic, h, candidates[i]))
	}

	// We still haven't voted enough validators in.
	pubs, err = neo.GetValidatorsInternal(bc, ic.DAO)
	require.NoError(t, err)
	require.Equal(t, bc.GetStandByValidators(), pubs)

	// Register and give some value to the last validator.
	require.NoError(t, neo.RegisterCandidateInternal(ic, candidates[0]))
	priv := testchain.PrivateKeyByID(0)
	h := priv.GetScriptHash()
	setSigner(tx, h)
	ic.VM.Load(priv.PublicKey().GetVerificationScript())
	require.NoError(t, neo.VoteInternal(ic, h, candidates[0]))

	pubs, err = neo.GetValidatorsInternal(bc, ic.DAO)
	require.NoError(t, err)
	require.Equal(t, candidates, pubs)

	require.NoError(t, neo.UnregisterCandidateInternal(ic, candidates[0]))
	require.Error(t, neo.VoteInternal(ic, h, candidates[0]))

	pubs, err = neo.GetValidatorsInternal(bc, ic.DAO)
	require.NoError(t, err)
	for i := range pubs {
		require.NotEqual(t, candidates[0], pubs[i])
	}
}
