package core

import (
	"math/big"
	"sort"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
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
	tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	ic := bc.newInteropContext(trigger.System, bc.dao, nil, tx)
	ic.VM = vm.New()
	ic.Block = bc.newBlock(tx)

	freq := testchain.ValidatorsCount + testchain.CommitteeSize()
	advanceChain := func(t *testing.T) {
		for i := 0; i < freq; i++ {
			require.NoError(t, neo.OnPersist(ic))
			ic.Block.Index++
		}
	}

	standBySorted := bc.GetStandByValidators()
	sort.Sort(standBySorted)
	pubs, err := neo.ComputeNextBlockValidators(bc, ic.DAO)
	require.NoError(t, err)
	require.Equal(t, standBySorted, pubs)

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
	pubs, err = neo.ComputeNextBlockValidators(bc, ic.DAO)
	require.NoError(t, err)
	require.Equal(t, standBySorted, pubs)

	advanceChain(t)
	pubs = neo.GetNextBlockValidatorsInternal()
	require.EqualValues(t, standBySorted, pubs)

	// Register and give some value to the last validator.
	require.NoError(t, neo.RegisterCandidateInternal(ic, candidates[0]))
	priv := testchain.PrivateKeyByID(0)
	h := priv.GetScriptHash()
	setSigner(tx, h)
	ic.VM.Load(priv.PublicKey().GetVerificationScript())
	require.NoError(t, neo.VoteInternal(ic, h, candidates[0]))

	for i := testchain.ValidatorsCount; i < testchain.CommitteeSize(); i++ {
		priv := testchain.PrivateKey(i)
		require.NoError(t, neo.RegisterCandidateInternal(ic, priv.PublicKey()))
	}

	advanceChain(t)
	pubs, err = neo.ComputeNextBlockValidators(bc, ic.DAO)
	require.NoError(t, err)
	sortedCandidates := candidates.Copy()
	sort.Sort(sortedCandidates)
	require.EqualValues(t, sortedCandidates, pubs)

	pubs = neo.GetNextBlockValidatorsInternal()
	require.EqualValues(t, sortedCandidates, pubs)

	require.NoError(t, neo.UnregisterCandidateInternal(ic, candidates[0]))
	require.Error(t, neo.VoteInternal(ic, h, candidates[0]))
	advanceChain(t)

	pubs, err = neo.ComputeNextBlockValidators(bc, ic.DAO)
	require.NoError(t, err)
	for i := range pubs {
		require.NotEqual(t, candidates[0], pubs[i])
	}
}

func TestNEO_SetGasPerBlock(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	neo := bc.contracts.NEO
	tx := transaction.New(netmode.UnitTestNet, []byte{}, 0)
	ic := bc.newInteropContext(trigger.System, bc.dao, nil, tx)
	ic.VM = vm.New()

	h, err := neo.GetCommitteeAddress(bc, bc.dao)
	require.NoError(t, err)

	t.Run("Default", func(t *testing.T) {
		g, err := neo.GetGASPerBlock(ic, 0)
		require.NoError(t, err)
		require.EqualValues(t, 5*native.GASFactor, g.Int64())
	})
	t.Run("Invalid", func(t *testing.T) {
		t.Run("InvalidSignature", func(t *testing.T) {
			setSigner(tx, util.Uint160{})
			ok, err := neo.SetGASPerBlock(ic, 10, big.NewInt(native.GASFactor))
			require.NoError(t, err)
			require.False(t, ok)
		})
		t.Run("TooBigValue", func(t *testing.T) {
			setSigner(tx, h)
			_, err := neo.SetGASPerBlock(ic, 10, big.NewInt(10*native.GASFactor+1))
			require.Error(t, err)
		})
	})
	t.Run("Valid", func(t *testing.T) {
		setSigner(tx, h)
		ok, err := neo.SetGASPerBlock(ic, 10, big.NewInt(native.GASFactor))
		require.NoError(t, err)
		require.True(t, ok)

		t.Run("Again", func(t *testing.T) {
			setSigner(tx, h)
			ok, err := neo.SetGASPerBlock(ic, 10, big.NewInt(native.GASFactor))
			require.NoError(t, err)
			require.True(t, ok)
		})

		g, err := neo.GetGASPerBlock(ic, 9)
		require.NoError(t, err)
		require.EqualValues(t, 5*native.GASFactor, g.Int64())

		g, err = neo.GetGASPerBlock(ic, 10)
		require.NoError(t, err)
		require.EqualValues(t, native.GASFactor, g.Int64())
	})
}

func TestNEO_CalculateBonus(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	neo := bc.contracts.NEO
	tx := transaction.New(netmode.UnitTestNet, []byte{}, 0)
	ic := bc.newInteropContext(trigger.System, bc.dao, nil, tx)
	t.Run("Invalid", func(t *testing.T) {
		_, err := neo.CalculateBonus(ic, new(big.Int).SetInt64(-1), 0, 1)
		require.Error(t, err)
	})
	t.Run("Zero", func(t *testing.T) {
		res, err := neo.CalculateBonus(ic, big.NewInt(0), 0, 100)
		require.NoError(t, err)
		require.EqualValues(t, 0, res.Int64())
	})
	t.Run("ManyBlocks", func(t *testing.T) {
		h, err := neo.GetCommitteeAddress(bc, bc.dao)
		require.NoError(t, err)
		setSigner(tx, h)
		ok, err := neo.SetGASPerBlock(ic, 10, big.NewInt(1*native.GASFactor))
		require.NoError(t, err)
		require.True(t, ok)

		res, err := neo.CalculateBonus(ic, big.NewInt(100), 5, 15)
		require.NoError(t, err)
		require.EqualValues(t, (100*5*5/10)+(100*5*1/10), res.Int64())

	})
}

func TestNEO_CommitteeBountyOnPersist(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	hs := make([]util.Uint160, testchain.CommitteeSize())
	for i := range hs {
		hs[i] = testchain.PrivateKeyByID(i).GetScriptHash()
	}

	bs := make(map[int]int64)
	checkBalances := func() {
		for i := 0; i < testchain.CommitteeSize(); i++ {
			require.EqualValues(t, bs[i], bc.GetUtilityTokenBalance(hs[i]).Int64())
		}
	}

	for i := 0; i < testchain.CommitteeSize()*2; i++ {
		require.NoError(t, bc.AddBlock(bc.newBlock()))
		bs[(i+1)%testchain.CommitteeSize()] += 25000000
		checkBalances()
	}
}
