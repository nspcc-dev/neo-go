package core

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func BenchmarkNEO_GetGASPerVote(t *testing.B) {
	var stores = map[string]func(testing.TB) storage.Store{
		"MemPS": func(t testing.TB) storage.Store {
			return storage.NewMemoryStore()
		},
		"BoltPS":  newBoltStoreForTesting,
		"LevelPS": newLevelDBForTesting,
	}
	for psName, newPS := range stores {
		for nRewardRecords := 10; nRewardRecords <= 1000; nRewardRecords *= 10 {
			for rewardDistance := 1; rewardDistance <= 1000; rewardDistance *= 10 {
				t.Run(fmt.Sprintf("%s_%dRewardRecords_%dRewardDistance", psName, nRewardRecords, rewardDistance), func(t *testing.B) {
					ps := newPS(t)
					t.Cleanup(func() { ps.Close() })
					benchmarkGasPerVote(t, ps, nRewardRecords, rewardDistance)
				})
			}
		}
	}
}

func benchmarkGasPerVote(t *testing.B, ps storage.Store, nRewardRecords int, rewardDistance int) {
	bc := newTestChainWithCustomCfgAndStore(t, ps, nil)

	neo := bc.contracts.NEO
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	ic := bc.newInteropContext(trigger.Application, bc.dao, nil, tx)
	ic.SpawnVM()
	ic.Block = bc.newBlock(tx)

	advanceChain := func(t *testing.B, count int) {
		for i := 0; i < count; i++ {
			require.NoError(t, bc.AddBlock(bc.newBlock()))
			ic.Block.Index++
		}
	}

	// Vote for new committee.
	sz := testchain.CommitteeSize()
	accs := make([]*wallet.Account, sz)
	candidates := make(keys.PublicKeys, sz)
	txs := make([]*transaction.Transaction, 0, len(accs))
	for i := 0; i < sz; i++ {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		candidates[i] = priv.PublicKey()
		accs[i], err = wallet.NewAccount()
		require.NoError(t, err)
		require.NoError(t, neo.RegisterCandidateInternal(ic, candidates[i]))

		to := accs[i].Contract.ScriptHash()
		w := io.NewBufBinWriter()
		emit.AppCall(w.BinWriter, bc.contracts.NEO.Hash, "transfer", callflag.All,
			neoOwner.BytesBE(), to.BytesBE(),
			big.NewInt(int64(sz-i)*1000000).Int64(), nil)
		emit.Opcodes(w.BinWriter, opcode.ASSERT)
		emit.AppCall(w.BinWriter, bc.contracts.GAS.Hash, "transfer", callflag.All,
			neoOwner.BytesBE(), to.BytesBE(),
			int64(1_000_000_000), nil)
		emit.Opcodes(w.BinWriter, opcode.ASSERT)
		require.NoError(t, w.Err)
		tx := transaction.New(w.Bytes(), 1000_000_000)
		tx.ValidUntilBlock = bc.BlockHeight() + 1
		setSigner(tx, testchain.MultisigScriptHash())
		require.NoError(t, testchain.SignTx(bc, tx))
		txs = append(txs, tx)
	}
	require.NoError(t, bc.AddBlock(bc.newBlock(txs...)))
	for _, tx := range txs {
		checkTxHalt(t, bc, tx.Hash())
	}
	for i := 0; i < sz; i++ {
		priv := accs[i].PrivateKey()
		h := priv.GetScriptHash()
		setSigner(tx, h)
		ic.VM.Load(priv.PublicKey().GetVerificationScript())
		require.NoError(t, neo.VoteInternal(ic, h, candidates[i]))
	}
	_, err := ic.DAO.Persist()
	require.NoError(t, err)

	// Collect set of nRewardRecords reward records for each voter.
	advanceChain(t, nRewardRecords*testchain.CommitteeSize())

	// Transfer some more NEO to first voter to update his balance height.
	to := accs[0].Contract.ScriptHash()
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, bc.contracts.NEO.Hash, "transfer", callflag.All,
		neoOwner.BytesBE(), to.BytesBE(), int64(1), nil)
	emit.Opcodes(w.BinWriter, opcode.ASSERT)
	require.NoError(t, w.Err)
	tx = transaction.New(w.Bytes(), 1000_000_000)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	setSigner(tx, testchain.MultisigScriptHash())
	require.NoError(t, testchain.SignTx(bc, tx))
	require.NoError(t, bc.AddBlock(bc.newBlock(tx)))

	aer, err := bc.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 1, len(aer))
	require.Equal(t, vm.HaltState, aer[0].VMState, aer[0].FaultException)

	// Advance chain one more time to avoid same start/end rewarding bounds.
	advanceChain(t, rewardDistance)
	end := bc.BlockHeight()

	t.ResetTimer()
	t.ReportAllocs()
	t.StartTimer()
	for i := 0; i < t.N; i++ {
		_, err := neo.CalculateBonus(ic.DAO, to, end)
		require.NoError(t, err)
	}
	t.StopTimer()
}
