package core_test

import (
	"fmt"
	"math/big"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func BenchmarkBlockchain_VerifyWitness(t *testing.B) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	tx := e.NewTx(t, []neotest.Signer{acc}, e.NativeHash(t, nativenames.Gas), "transfer", acc.ScriptHash(), acc.Script(), 1, nil)

	t.ResetTimer()
	for n := 0; n < t.N; n++ {
		_, err := bc.VerifyWitness(tx.Signers[0].Account, tx, &tx.Scripts[0], 100000000)
		require.NoError(t, err)
	}
}

func BenchmarkBlockchain_ForEachNEP17Transfer(t *testing.B) {
	var stores = map[string]func(testing.TB) storage.Store{
		"MemPS": func(t testing.TB) storage.Store {
			return storage.NewMemoryStore()
		},
		"BoltPS":  newBoltStoreForTesting,
		"LevelPS": newLevelDBForTesting,
	}
	startFrom := []int{1, 100, 1000}
	blocksToTake := []int{100, 1000}
	for psName, newPS := range stores {
		for _, startFromBlock := range startFrom {
			for _, nBlocksToTake := range blocksToTake {
				t.Run(fmt.Sprintf("%s_StartFromBlockN-%d_Take%dBlocks", psName, startFromBlock, nBlocksToTake), func(t *testing.B) {
					ps := newPS(t)
					t.Cleanup(func() { ps.Close() })
					benchmarkForEachNEP17Transfer(t, ps, startFromBlock, nBlocksToTake)
				})
			}
		}
	}
}

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

func benchmarkForEachNEP17Transfer(t *testing.B, ps storage.Store, startFromBlock, nBlocksToTake int) {
	var (
		chainHeight       = 2_100                            // constant chain height to be able to compare paging results
		transfersPerBlock = state.TokenTransferBatchSize/4 + // 4 blocks per batch
			state.TokenTransferBatchSize/32 // shift
	)

	bc, validators, committee := chain.NewMultiWithCustomConfigAndStore(t, nil, ps, true)

	e := neotest.NewExecutor(t, bc, validators, committee)
	gasHash := e.NativeHash(t, nativenames.Gas)

	acc := random.Uint160()
	from := e.Validator.ScriptHash()

	for j := 0; j < chainHeight; j++ {
		w := io.NewBufBinWriter()
		for i := 0; i < transfersPerBlock; i++ {
			emit.AppCall(w.BinWriter, gasHash, "transfer", callflag.All, from, acc, 1, nil)
			emit.Opcodes(w.BinWriter, opcode.ASSERT)
		}
		require.NoError(t, w.Err)
		script := w.Bytes()
		tx := transaction.New(script, int64(1100_0000*transfersPerBlock))
		tx.NetworkFee = 1_0000_000
		tx.ValidUntilBlock = bc.BlockHeight() + 1
		tx.Nonce = neotest.Nonce()
		tx.Signers = []transaction.Signer{{Account: from, Scopes: transaction.CalledByEntry}}
		require.NoError(t, validators.SignTx(netmode.UnitTestNet, tx))
		e.AddNewBlock(t, tx)
		e.CheckHalt(t, tx.Hash())
	}

	newestB, err := bc.GetBlock(bc.GetHeaderHash(int(bc.BlockHeight()) - startFromBlock + 1))
	require.NoError(t, err)
	newestTimestamp := newestB.Timestamp
	oldestB, err := bc.GetBlock(bc.GetHeaderHash(int(newestB.Index) - nBlocksToTake))
	require.NoError(t, err)
	oldestTimestamp := oldestB.Timestamp

	t.ResetTimer()
	t.ReportAllocs()
	t.StartTimer()
	for i := 0; i < t.N; i++ {
		require.NoError(t, bc.ForEachNEP17Transfer(acc, newestTimestamp, func(t *state.NEP17Transfer) (bool, error) {
			if t.Timestamp < oldestTimestamp {
				// iterating from newest to oldest, already have reached the needed height
				return false, nil
			}
			return true, nil
		}))
	}
	t.StopTimer()
}

func newLevelDBForTesting(t testing.TB) storage.Store {
	dbPath := t.TempDir()
	dbOptions := storage.LevelDBOptions{
		DataDirectoryPath: dbPath,
	}
	newLevelStore, err := storage.NewLevelDBStore(dbOptions)
	require.Nil(t, err, "NewLevelDBStore error")
	return newLevelStore
}

func newBoltStoreForTesting(t testing.TB) storage.Store {
	d := t.TempDir()
	dbPath := filepath.Join(d, "test_bolt_db")
	boltDBStore, err := storage.NewBoltDBStore(storage.BoltDBOptions{FilePath: dbPath})
	require.NoError(t, err)
	return boltDBStore
}

func benchmarkGasPerVote(t *testing.B, ps storage.Store, nRewardRecords int, rewardDistance int) {
	bc, validators, committee := chain.NewMultiWithCustomConfigAndStore(t, nil, ps, true)
	cfg := bc.GetConfig()

	e := neotest.NewExecutor(t, bc, validators, committee)
	neoHash := e.NativeHash(t, nativenames.Neo)
	gasHash := e.NativeHash(t, nativenames.Gas)
	neoSuperInvoker := e.NewInvoker(neoHash, validators, committee)
	neoValidatorsInvoker := e.ValidatorInvoker(neoHash)
	gasValidatorsInvoker := e.ValidatorInvoker(gasHash)

	// Vote for new committee.
	sz := len(cfg.StandbyCommittee)
	voters := make([]*wallet.Account, sz)
	candidates := make(keys.PublicKeys, sz)
	txs := make([]*transaction.Transaction, 0, len(voters)*3)
	for i := 0; i < sz; i++ {
		priv, err := keys.NewPrivateKey()
		require.NoError(t, err)
		candidates[i] = priv.PublicKey()
		voters[i], err = wallet.NewAccount()
		require.NoError(t, err)
		registerTx := neoSuperInvoker.PrepareInvoke(t, "registerCandidate", candidates[i].Bytes())
		txs = append(txs, registerTx)

		to := voters[i].Contract.ScriptHash()
		transferNeoTx := neoValidatorsInvoker.PrepareInvoke(t, "transfer", e.Validator.ScriptHash(), to, big.NewInt(int64(sz-i)*1000000).Int64(), nil)
		txs = append(txs, transferNeoTx)

		transferGasTx := gasValidatorsInvoker.PrepareInvoke(t, "transfer", e.Validator.ScriptHash(), to, int64(1_000_000_000), nil)
		txs = append(txs, transferGasTx)
	}
	e.AddNewBlock(t, txs...)
	for _, tx := range txs {
		e.CheckHalt(t, tx.Hash())
	}
	voteTxs := make([]*transaction.Transaction, 0, sz)
	for i := 0; i < sz; i++ {
		priv := voters[i].PrivateKey()
		h := priv.GetScriptHash()
		voteTx := e.NewTx(t, []neotest.Signer{neotest.NewSingleSigner(voters[i])}, neoHash, "vote", h, candidates[i].Bytes())
		voteTxs = append(voteTxs, voteTx)
	}
	e.AddNewBlock(t, voteTxs...)
	for _, tx := range voteTxs {
		e.CheckHalt(t, tx.Hash())
	}

	// Collect set of nRewardRecords reward records for each voter.
	e.GenerateNewBlocks(t, len(cfg.StandbyCommittee))

	// Transfer some more NEO to first voter to update his balance height.
	to := voters[0].Contract.ScriptHash()
	neoValidatorsInvoker.Invoke(t, true, "transfer", e.Validator.ScriptHash(), to, int64(1), nil)

	// Advance chain one more time to avoid same start/end rewarding bounds.
	e.GenerateNewBlocks(t, rewardDistance)
	end := bc.BlockHeight()

	t.ResetTimer()
	t.ReportAllocs()
	t.StartTimer()
	for i := 0; i < t.N; i++ {
		_, err := bc.CalculateClaimable(to, end)
		require.NoError(t, err)
	}
	t.StopTimer()
}
