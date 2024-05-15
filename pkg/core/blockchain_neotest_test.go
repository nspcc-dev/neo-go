package core_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/internal/contracts"
	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeprices"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newLevelDBForTestingWithPath(t testing.TB, dbPath string) (storage.Store, string) {
	if dbPath == "" {
		dbPath = t.TempDir()
	}
	dbOptions := dbconfig.LevelDBOptions{
		DataDirectoryPath: dbPath,
	}
	newLevelStore, err := storage.NewLevelDBStore(dbOptions)
	require.Nil(t, err, "NewLevelDBStore error")
	return newLevelStore, dbPath
}

func TestBlockchain_StartFromExistingDB(t *testing.T) {
	ps, path := newLevelDBForTestingWithPath(t, "")
	customConfig := func(c *config.Blockchain) {
		c.StateRootInHeader = true // Need for P2PStateExchangeExtensions check.
		c.P2PSigExtensions = true  // Need for basic chain initializer.
	}
	bc, validators, committee, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, ps)
	require.NoError(t, err)
	go bc.Run()
	e := neotest.NewExecutor(t, bc, validators, committee)
	basicchain.Init(t, "../../", e)
	require.True(t, bc.BlockHeight() > 5, "ensure that basic chain is correctly initialised")

	// Information for further tests.
	h := bc.BlockHeight()
	cryptoLibHash, err := bc.GetNativeContractScriptHash(nativenames.CryptoLib)
	require.NoError(t, err)
	cryptoLibState := bc.GetContractState(cryptoLibHash)
	require.NotNil(t, cryptoLibState)
	var (
		managementID             = -1
		managementContractPrefix = 8
	)

	bc.Close() // Ensure persist is done and persistent store is properly closed.

	newPS := func(t *testing.T) storage.Store {
		ps, _ = newLevelDBForTestingWithPath(t, path)
		t.Cleanup(func() { require.NoError(t, ps.Close()) })
		return ps
	}
	t.Run("mismatch storage version", func(t *testing.T) {
		ps = newPS(t)
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		d := dao.NewSimple(cache, bc.GetConfig().StateRootInHeader)
		d.PutVersion(dao.Version{
			Value: "0.0.0",
		})
		_, err := d.Persist() // Persist to `cache` wrapper.
		require.NoError(t, err)
		_, _, _, err = chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "storage version mismatch"), err)
	})
	t.Run("mismatch StateRootInHeader", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.Blockchain) {
			customConfig(c)
			c.StateRootInHeader = false
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "StateRootInHeader setting mismatch"), err)
	})
	t.Run("mismatch P2PSigExtensions", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.Blockchain) {
			customConfig(c)
			c.P2PSigExtensions = false
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "P2PSigExtensions setting mismatch"), err)
	})
	t.Run("mismatch P2PStateExchangeExtensions", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.Blockchain) {
			customConfig(c)
			c.StateRootInHeader = true
			c.P2PStateExchangeExtensions = true
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "P2PStateExchangeExtensions setting mismatch"), err)
	})
	t.Run("mismatch KeepOnlyLatestState", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.Blockchain) {
			customConfig(c)
			c.Ledger.KeepOnlyLatestState = true
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "KeepOnlyLatestState setting mismatch"), err)
	})
	t.Run("Magic mismatch", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.Blockchain) {
			customConfig(c)
			c.Magic = 100500
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "protocol configuration Magic mismatch"), err)
	})
	t.Run("corrupted headers", func(t *testing.T) {
		ps = newPS(t)

		// Corrupt headers hashes batch.
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		// Make the chain think we're at 2000+ which will trigger page 0 read.
		buf := io.NewBufBinWriter()
		buf.WriteBytes(util.Uint256{}.BytesLE())
		buf.WriteU32LE(2000)
		cache.Put([]byte{byte(storage.SYSCurrentHeader)}, buf.Bytes())

		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "failed to retrieve header hash page"), err)
	})
	t.Run("corrupted current header height", func(t *testing.T) {
		ps = newPS(t)

		// Remove current header.
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		cache.Delete([]byte{byte(storage.SYSCurrentHeader)})

		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "failed to retrieve current header"), err)
	})
	t.Run("missing last batch of 2000 headers and missing last header", func(t *testing.T) {
		ps = newPS(t)

		// Remove latest headers hashes batch and current header.
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		cache.Delete([]byte{byte(storage.IXHeaderHashList)})
		currHeaderInfo, err := cache.Get([]byte{byte(storage.SYSCurrentHeader)})
		require.NoError(t, err)
		currHeaderHash, err := util.Uint256DecodeBytesLE(currHeaderInfo[:32])
		require.NoError(t, err)
		cache.Delete(append([]byte{byte(storage.DataExecutable)}, currHeaderHash.BytesBE()...))

		_, _, _, err = chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "could not get header"), err)
	})
	t.Run("missing last block", func(t *testing.T) {
		ps = newPS(t)

		// Remove current block from storage.
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		cache.Delete([]byte{byte(storage.SYSCurrentBlock)})

		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "failed to retrieve current block height"), err)
	})
	t.Run("missing last stateroot", func(t *testing.T) {
		ps = newPS(t)

		// Remove latest stateroot from storage.
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		key := make([]byte, 5)
		key[0] = byte(storage.DataMPTAux)
		binary.BigEndian.PutUint32(key[1:], h)
		cache.Delete(key)

		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "can't init MPT at height"), err)
	})
	t.Run("failed native Management initialisation", func(t *testing.T) {
		ps = newPS(t)

		// Corrupt serialised CryptoLib state.
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		key := make([]byte, 1+4+1+20)
		key[0] = byte(storage.STStorage)
		binary.LittleEndian.PutUint32(key[1:], uint32(managementID))
		key[5] = byte(managementContractPrefix)
		copy(key[6:], cryptoLibHash.BytesBE())
		cache.Put(key, []byte{1, 2, 3})

		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "can't init natives cache: failed to initialize cache for ContractManagement"), err)
	})
	t.Run("invalid native contract activation", func(t *testing.T) {
		ps = newPS(t)

		// Remove CryptoLib from storage.
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		key := make([]byte, 1+4+1+20)
		key[0] = byte(storage.STStorage)
		binary.LittleEndian.PutUint32(key[1:], uint32(managementID))
		key[5] = byte(managementContractPrefix)
		copy(key[6:], cryptoLibHash.BytesBE())
		cache.Delete(key)

		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), fmt.Sprintf("native contract %s is not stored, but should be active at height %d according to config", nativenames.CryptoLib, h)), err)
	})
	t.Run("stored and autogenerated native contract's states mismatch", func(t *testing.T) {
		ps = newPS(t)

		// Change stored CryptoLib state.
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		key := make([]byte, 1+4+1+20)
		key[0] = byte(storage.STStorage)
		binary.LittleEndian.PutUint32(key[1:], uint32(managementID))
		key[5] = byte(managementContractPrefix)
		copy(key[6:], cryptoLibHash.BytesBE())
		cs := *cryptoLibState
		cs.ID = -123
		csBytes, err := stackitem.SerializeConvertible(&cs)
		require.NoError(t, err)
		cache.Put(key, csBytes)

		_, _, _, err = chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), fmt.Sprintf("native %s: version mismatch for the latest hardfork Cockatrice (stored contract state differs from autogenerated one)", nativenames.CryptoLib)), err)
	})

	t.Run("good", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, ps)
		require.NoError(t, err)
	})
}

// TestBlockchain_InitializeNeoCache_Bug3181 is aimed to reproduce and check situation
// when panic occures on native Neo cache initialization due to access to native Policy
// cache when it's not yet initialized to recalculate candidates.
func TestBlockchain_InitializeNeoCache_Bug3181(t *testing.T) {
	ps, path := newLevelDBForTestingWithPath(t, "")
	bc, validators, committee, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, nil, ps)
	require.NoError(t, err)
	go bc.Run()
	e := neotest.NewExecutor(t, bc, validators, committee)

	// Add at least one registered candidate to enable candidates Policy check.
	acc := e.NewAccount(t, 10000_0000_0000) // block #1
	neo := e.NewInvoker(e.NativeHash(t, nativenames.Neo), acc)
	neo.Invoke(t, true, "registerCandidate", acc.(neotest.SingleSigner).Account().PublicKey().Bytes()) // block #2

	// Put some empty blocks to reach N-1 block height, so that newEpoch cached
	// values of native Neo contract require an update on the subsequent cache
	// initialization.
	for i := 0; i < len(bc.GetConfig().StandbyCommittee)-1-2; i++ {
		e.AddNewBlock(t)
	}
	bc.Close() // Ensure persist is done and persistent store is properly closed.

	ps, _ = newLevelDBForTestingWithPath(t, path)
	t.Cleanup(func() { require.NoError(t, ps.Close()) })

	// Start chain from the existing database that should trigger an update of native
	// Neo newEpoch* cached values during initializaition. This update requires candidates
	// list recalculation and policies checks, thus, access to native Policy cache
	// that is not yet initialized by that moment.
	require.NotPanics(t, func() {
		_, _, _, err = chain.NewMultiWithCustomConfigAndStoreNoCheck(t, nil, ps)
		require.NoError(t, err)
	})
}

// This test enables Notary native contract at non-zero height and checks that no
// Notary cache initialization is performed before that height on node restart.
/*
func TestBlockchain_InitializeNativeCacheWrtNativeActivations(t *testing.T) {
	const notaryEnabledHeight = 3
	ps, path := newLevelDBForTestingWithPath(t, "")
	customConfig := func(c *config.Blockchain) {
		c.P2PSigExtensions = true
		c.NativeUpdateHistories = make(map[string][]uint32)
		for _, n := range []string{
			nativenames.Neo,
			nativenames.Gas,
			nativenames.Designation,
			nativenames.Management,
			nativenames.CryptoLib,
			nativenames.Ledger,
			nativenames.Management,
			nativenames.Oracle,
			nativenames.Policy,
			nativenames.StdLib,
			nativenames.Notary,
		} {
			if n == nativenames.Notary {
				c.NativeUpdateHistories[n] = []uint32{notaryEnabledHeight}
			} else {
				c.NativeUpdateHistories[n] = []uint32{0}
			}
		}
	}
	bc, validators, committee, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, ps)
	require.NoError(t, err)
	go bc.Run()
	e := neotest.NewExecutor(t, bc, validators, committee)
	e.AddNewBlock(t)
	bc.Close() // Ensure persist is done and persistent store is properly closed.

	ps, _ = newLevelDBForTestingWithPath(t, path)

	// If NativeActivations are not taken into account during native cache initialization,
	// bs.init() will panic on Notary cache initialization as it's not deployed yet.
	require.NotPanics(t, func() {
		bc, _, _, err = chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, ps)
		require.NoError(t, err)
	})
	go bc.Run()
	defer bc.Close()
	e = neotest.NewExecutor(t, bc, validators, committee)
	h := e.Chain.BlockHeight()

	// Notary isn't initialized yet, so accessing Notary cache should return error.
	_, err = e.Chain.GetMaxNotValidBeforeDelta()
	require.Error(t, err)

	// Ensure Notary will be properly initialized and accessing Notary cache works
	// as expected.
	for i := 0; i < notaryEnabledHeight; i++ {
		require.NotPanics(t, func() {
			e.AddNewBlock(t)
		}, h+uint32(i)+1)
	}
	_, err = e.Chain.GetMaxNotValidBeforeDelta()
	require.NoError(t, err)
}
*/

func TestBlockchain_AddHeaders(t *testing.T) {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.StateRootInHeader = true
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	newHeader := func(t *testing.T, index uint32, prevHash util.Uint256, timestamp uint64) *block.Header {
		b := e.NewUnsignedBlock(t)
		b.Index = index
		b.PrevHash = prevHash
		b.PrevStateRoot = util.Uint256{}
		b.Timestamp = timestamp
		e.SignBlock(b)
		return &b.Header
	}
	b1 := e.NewUnsignedBlock(t)
	h1 := &e.SignBlock(b1).Header
	h2 := newHeader(t, h1.Index+1, h1.Hash(), h1.Timestamp+1)
	h3 := newHeader(t, h2.Index+1, h2.Hash(), h2.Timestamp+1)

	require.NoError(t, bc.AddHeaders())
	require.NoError(t, bc.AddHeaders(h1, h2))
	require.NoError(t, bc.AddHeaders(h2, h3))

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	// Add them again, they should not be added.
	require.NoError(t, bc.AddHeaders(h3, h2, h1))

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	h4Bad := newHeader(t, h3.Index+1, h3.Hash().Reverse(), h3.Timestamp+1)
	h5Bad := newHeader(t, h4Bad.Index+1, h4Bad.Hash(), h4Bad.Timestamp+1)

	assert.Error(t, bc.AddHeaders(h4Bad, h5Bad))
	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	h4Bad2 := newHeader(t, h3.Index+1, h3.Hash().Reverse(), h3.Timestamp+1)
	h4Bad2.Script.InvocationScript = []byte{}
	assert.Error(t, bc.AddHeaders(h4Bad2))
	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())
}

func TestBlockchain_AddBlockStateRoot(t *testing.T) {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.StateRootInHeader = true
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	sr, err := bc.GetStateModule().GetStateRoot(bc.BlockHeight())
	require.NoError(t, err)

	b := e.NewUnsignedBlock(t)
	b.StateRootEnabled = false
	b.PrevStateRoot = util.Uint256{}
	e.SignBlock(b)
	err = bc.AddBlock(b)
	require.ErrorIs(t, err, core.ErrHdrStateRootSetting)

	u := sr.Root
	u[0] ^= 0xFF
	b = e.NewUnsignedBlock(t)
	b.PrevStateRoot = u
	e.SignBlock(b)
	err = bc.AddBlock(b)
	require.ErrorIs(t, err, core.ErrHdrInvalidStateRoot)

	b = e.NewUnsignedBlock(t)
	e.SignBlock(b)
	require.NoError(t, bc.AddBlock(b))
}

func TestBlockchain_AddHeadersStateRoot(t *testing.T) {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.StateRootInHeader = true
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	b := e.NewUnsignedBlock(t)
	e.SignBlock(b)
	h1 := b.Header
	r := bc.GetStateModule().CurrentLocalStateRoot()

	// invalid stateroot
	h1.PrevStateRoot[0] ^= 0xFF
	require.ErrorIs(t, bc.AddHeaders(&h1), core.ErrHdrInvalidStateRoot)

	// valid stateroot
	h1.PrevStateRoot = r
	require.NoError(t, bc.AddHeaders(&h1))

	// unable to verify stateroot (stateroot is computed for block #0 only => can
	// verify stateroot of header #1 only) => just store the header
	b = e.NewUnsignedBlock(t)
	b.PrevHash = h1.Hash()
	b.Timestamp = h1.Timestamp + 1
	b.PrevStateRoot = util.Uint256{}
	b.Index = h1.Index + 1
	e.SignBlock(b)
	require.NoError(t, bc.AddHeaders(&b.Header))
}

func TestBlockchain_AddBadBlock(t *testing.T) {
	check := func(t *testing.T, b *block.Block, cfg func(c *config.Blockchain)) {
		bc, _ := chain.NewSingleWithCustomConfig(t, cfg)
		err := bc.AddBlock(b)
		if cfg == nil {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	neoHash := e.NativeHash(t, nativenames.Neo)

	tx := e.NewUnsignedTx(t, neoHash, "transfer", acc.ScriptHash(), util.Uint160{1, 2, 3}, 1, nil)
	tx.ValidUntilBlock = 0 // Intentionally make the transaction invalid.
	e.SignTx(t, tx, -1, acc)
	b := e.NewUnsignedBlock(t, tx)
	e.SignBlock(b)
	check(t, b, nil)
	check(t, b, func(c *config.Blockchain) {
		c.SkipBlockVerification = true
	})

	b = e.NewUnsignedBlock(t)
	b.PrevHash = util.Uint256{} // Intentionally make block invalid.
	e.SignBlock(b)
	check(t, b, nil)
	check(t, b, func(c *config.Blockchain) {
		c.SkipBlockVerification = true
	})

	tx = e.NewUnsignedTx(t, neoHash, "transfer", acc.ScriptHash(), util.Uint160{1, 2, 3}, 1, nil) // Check the good tx.
	e.SignTx(t, tx, -1, acc)
	b = e.NewUnsignedBlock(t, tx)
	e.SignBlock(b)
	check(t, b, func(c *config.Blockchain) {
		c.VerifyTransactions = true
		c.SkipBlockVerification = false
	})
}

func TestBlockchain_GetHeader(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	block := e.AddNewBlock(t)
	hash := block.Hash()
	header, err := bc.GetHeader(hash)
	require.NoError(t, err)
	assert.Equal(t, &block.Header, header)

	b2 := e.NewUnsignedBlock(t)
	_, err = bc.GetHeader(b2.Hash())
	assert.Error(t, err)
}

func TestBlockchain_GetBlock(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	blocks := e.GenerateNewBlocks(t, 10)
	neoValidatorInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Neo))

	for i := 0; i < len(blocks); i++ {
		block, err := bc.GetBlock(blocks[i].Hash())
		require.NoErrorf(t, err, "can't get block %d: %s", i, err)
		assert.Equal(t, blocks[i].Index, block.Index)
		assert.Equal(t, blocks[i].Hash(), block.Hash())
	}

	t.Run("store only header", func(t *testing.T) {
		t.Run("non-empty block", func(t *testing.T) {
			tx := neoValidatorInvoker.PrepareInvoke(t, "transfer", acc.ScriptHash(), util.Uint160{1, 2, 3}, 1, nil)
			b := e.NewUnsignedBlock(t, tx)
			e.SignBlock(b)
			require.NoError(t, bc.AddHeaders(&b.Header))

			_, err := bc.GetBlock(b.Hash())
			require.Error(t, err)

			_, err = bc.GetHeader(b.Hash())
			require.NoError(t, err)

			require.NoError(t, bc.AddBlock(b))

			_, err = bc.GetBlock(b.Hash())
			require.NoError(t, err)
		})
		t.Run("empty block", func(t *testing.T) {
			b := e.NewUnsignedBlock(t)
			e.SignBlock(b)

			require.NoError(t, bc.AddHeaders(&b.Header))

			_, err := bc.GetBlock(b.Hash())
			require.NoError(t, err)
		})
	})
}

func TestBlockchain_VerifyHashAgainstScript(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	cs, csInvalid := contracts.GetTestContractState(t, pathToInternalContracts, 0, 1, acc.ScriptHash())
	c1 := &neotest.Contract{
		Hash:     cs.Hash,
		NEF:      &cs.NEF,
		Manifest: &cs.Manifest,
	}
	c2 := &neotest.Contract{
		Hash:     csInvalid.Hash,
		NEF:      &csInvalid.NEF,
		Manifest: &csInvalid.Manifest,
	}
	e.DeployContract(t, c1, nil)
	e.DeployContract(t, c2, nil)

	gas := bc.GetMaxVerificationGAS()
	t.Run("Contract", func(t *testing.T) {
		t.Run("Missing", func(t *testing.T) {
			newH := cs.Hash
			newH[0] = ^newH[0]
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH4)}}
			_, err := bc.VerifyWitness(newH, nil, w, gas)
			require.ErrorIs(t, err, core.ErrUnknownVerificationContract)
		})
		t.Run("Invalid", func(t *testing.T) {
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH4)}}
			_, err := bc.VerifyWitness(csInvalid.Hash, nil, w, gas)
			require.ErrorIs(t, err, core.ErrInvalidVerificationContract)
		})
		t.Run("ValidSignature", func(t *testing.T) {
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH4)}}
			_, err := bc.VerifyWitness(cs.Hash, nil, w, gas)
			require.NoError(t, err)
		})
		t.Run("InvalidSignature", func(t *testing.T) {
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH3)}}
			_, err := bc.VerifyWitness(cs.Hash, nil, w, gas)
			require.ErrorIs(t, err, core.ErrVerificationFailed)
		})
	})
	t.Run("NotEnoughGas", func(t *testing.T) {
		verif := []byte{byte(opcode.PUSH1)}
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.NOP)},
			VerificationScript: verif,
		}
		_, err := bc.VerifyWitness(hash.Hash160(verif), nil, w, 1)
		require.ErrorIs(t, err, core.ErrVerificationFailed)
	})
	t.Run("NoResult", func(t *testing.T) {
		verif := []byte{byte(opcode.DROP)}
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.PUSH1)},
			VerificationScript: verif,
		}
		_, err := bc.VerifyWitness(hash.Hash160(verif), nil, w, gas)
		require.ErrorIs(t, err, core.ErrVerificationFailed)
	})
	t.Run("BadResult", func(t *testing.T) {
		verif := make([]byte, keys.SignatureLen+2)
		verif[0] = byte(opcode.PUSHDATA1)
		verif[1] = keys.SignatureLen
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.NOP)},
			VerificationScript: verif,
		}
		_, err := bc.VerifyWitness(hash.Hash160(verif), nil, w, gas)
		require.ErrorIs(t, err, core.ErrVerificationFailed)
	})
	t.Run("TooManyResults", func(t *testing.T) {
		verif := []byte{byte(opcode.NOP)}
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.PUSH1), byte(opcode.PUSH1)},
			VerificationScript: verif,
		}
		_, err := bc.VerifyWitness(hash.Hash160(verif), nil, w, gas)
		require.ErrorIs(t, err, core.ErrVerificationFailed)
	})
}

func TestBlockchain_IsTxStillRelevant(t *testing.T) {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.P2PSigExtensions = true
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	mp := bc.GetMemPool()

	t.Run("small ValidUntilBlock", func(t *testing.T) {
		tx := e.PrepareInvocation(t, []byte{byte(opcode.PUSH1)}, []neotest.Signer{acc}, bc.BlockHeight()+1)

		require.True(t, bc.IsTxStillRelevant(tx, nil, false))
		e.AddNewBlock(t)
		require.False(t, bc.IsTxStillRelevant(tx, nil, false))
	})

	t.Run("tx is already persisted", func(t *testing.T) {
		tx := e.PrepareInvocation(t, []byte{byte(opcode.PUSH1)}, []neotest.Signer{acc}, bc.BlockHeight()+2)

		require.True(t, bc.IsTxStillRelevant(tx, nil, false))
		e.AddNewBlock(t, tx)
		require.False(t, bc.IsTxStillRelevant(tx, nil, false))
	})

	t.Run("tx with Conflicts attribute", func(t *testing.T) {
		tx1 := e.PrepareInvocation(t, []byte{byte(opcode.PUSH1)}, []neotest.Signer{acc}, bc.BlockHeight()+5)

		tx2 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		tx2.Nonce = neotest.Nonce()
		tx2.ValidUntilBlock = e.Chain.BlockHeight() + 5
		tx2.Attributes = []transaction.Attribute{{
			Type:  transaction.ConflictsT,
			Value: &transaction.Conflicts{Hash: tx1.Hash()},
		}}
		e.SignTx(t, tx2, -1, acc)

		require.True(t, bc.IsTxStillRelevant(tx1, mp, false))
		require.NoError(t, bc.PoolTx(tx2))
		require.False(t, bc.IsTxStillRelevant(tx1, mp, false))
	})
	t.Run("NotValidBefore", func(t *testing.T) {
		tx3 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		tx3.Nonce = neotest.Nonce()
		tx3.Attributes = []transaction.Attribute{{
			Type:  transaction.NotValidBeforeT,
			Value: &transaction.NotValidBefore{Height: bc.BlockHeight() + 1},
		}}
		tx3.ValidUntilBlock = bc.BlockHeight() + 2
		e.SignTx(t, tx3, -1, acc)

		require.False(t, bc.IsTxStillRelevant(tx3, nil, false))
		e.AddNewBlock(t)
		require.True(t, bc.IsTxStillRelevant(tx3, nil, false))
	})
	t.Run("contract witness check fails", func(t *testing.T) {
		src := fmt.Sprintf(`package verify
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/contract"
			"github.com/nspcc-dev/neo-go/pkg/interop/lib/address"
		)
		func Verify() bool {
			addr := address.ToHash160("`+address.Uint160ToString(e.NativeHash(t, nativenames.Ledger))+`")
			currentHeight := contract.Call(addr, "currentIndex", contract.ReadStates)
			return currentHeight.(int) < %d
		}`, bc.BlockHeight()+2) // deploy + next block
		c := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(src), &compiler.Options{
			Name: "verification_contract",
		})
		e.DeployContract(t, c, nil)

		tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = neotest.Nonce()
		tx.ValidUntilBlock = bc.BlockHeight() + 2
		tx.Signers = []transaction.Signer{
			{
				Account: c.Hash,
				Scopes:  transaction.None,
			},
		}
		tx.NetworkFee += 10_000_000
		tx.Scripts = []transaction.Witness{{}}

		require.True(t, bc.IsTxStillRelevant(tx, mp, false))
		e.AddNewBlock(t)
		require.False(t, bc.IsTxStillRelevant(tx, mp, false))
	})
}

func TestBlockchain_MemPoolRemoval(t *testing.T) {
	const added = 16
	const notAdded = 32
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	addedTxes := make([]*transaction.Transaction, added)
	notAddedTxes := make([]*transaction.Transaction, notAdded)
	for i := range addedTxes {
		addedTxes[i] = e.PrepareInvocation(t, []byte{byte(opcode.PUSH1)}, []neotest.Signer{acc}, 100)
		require.NoError(t, bc.PoolTx(addedTxes[i]))
	}
	for i := range notAddedTxes {
		notAddedTxes[i] = e.PrepareInvocation(t, []byte{byte(opcode.PUSH1)}, []neotest.Signer{acc}, 100)
		require.NoError(t, bc.PoolTx(notAddedTxes[i]))
	}
	mempool := bc.GetMemPool()
	e.AddNewBlock(t, addedTxes...)
	for _, tx := range addedTxes {
		require.False(t, mempool.ContainsKey(tx.Hash()))
	}
	for _, tx := range notAddedTxes {
		require.True(t, mempool.ContainsKey(tx.Hash()))
	}
}

func TestBlockchain_HasBlock(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	blocks := e.GenerateNewBlocks(t, 10)

	for i := 0; i < len(blocks); i++ {
		assert.True(t, bc.HasBlock(blocks[i].Hash()))
	}
	newBlock := e.NewUnsignedBlock(t)
	assert.False(t, bc.HasBlock(newBlock.Hash()))
}

func TestBlockchain_GetTransaction(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	tx1 := e.PrepareInvocation(t, []byte{byte(opcode.PUSH1)}, []neotest.Signer{acc})
	e.AddNewBlock(t, tx1)

	tx2 := e.PrepareInvocation(t, []byte{byte(opcode.PUSH2)}, []neotest.Signer{acc})
	tx2Size := io.GetVarSize(tx2)
	b := e.AddNewBlock(t, tx2)

	tx, height, err := bc.GetTransaction(tx2.Hash())
	require.Nil(t, err)
	assert.Equal(t, b.Index, height)
	assert.Equal(t, tx2Size, tx.Size())
	assert.Equal(t, b.Transactions[0], tx)
}

func TestBlockchain_GetClaimable(t *testing.T) {
	bc, acc := chain.NewSingle(t)

	t.Run("first generation period", func(t *testing.T) {
		amount, err := bc.CalculateClaimable(acc.ScriptHash(), 1)
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(5*native.GASFactor/10), amount)
	})
}

func TestBlockchain_Close(t *testing.T) {
	st := storage.NewMemoryStore()
	bc, acc := chain.NewSingleWithCustomConfigAndStore(t, nil, st, false)
	e := neotest.NewExecutor(t, bc, acc, acc)
	go bc.Run()
	e.GenerateNewBlocks(t, 10)
	bc.Close()
	// It's a hack, but we use internal knowledge of MemoryStore
	// implementation which makes it completely unusable (up to panicing)
	// after Close().
	require.Panics(t, func() {
		_ = st.PutChangeSet(map[string][]byte{"0": {1}}, nil)
	})
}

func TestBlockchain_Subscriptions(t *testing.T) {
	// We use buffering here as a substitute for reader goroutines, events
	// get queued up and we read them one by one here.
	const chBufSize = 16
	blockCh := make(chan *block.Block, chBufSize)
	txCh := make(chan *transaction.Transaction, chBufSize)
	notificationCh := make(chan *state.ContainedNotificationEvent, chBufSize)
	executionCh := make(chan *state.AppExecResult, chBufSize)

	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	nativeGASHash := e.NativeHash(t, nativenames.Gas)
	bc.SubscribeForBlocks(blockCh)
	bc.SubscribeForTransactions(txCh)
	bc.SubscribeForNotifications(notificationCh)
	bc.SubscribeForExecutions(executionCh)

	assert.Empty(t, notificationCh)
	assert.Empty(t, executionCh)
	assert.Empty(t, blockCh)
	assert.Empty(t, txCh)

	generatedB := e.AddNewBlock(t)
	require.Eventually(t, func() bool { return len(blockCh) != 0 }, time.Second, 10*time.Millisecond)
	assert.Len(t, notificationCh, 1) // validator bounty
	assert.Len(t, executionCh, 2)
	assert.Empty(t, txCh)

	b := <-blockCh
	assert.Equal(t, generatedB, b)
	assert.Empty(t, blockCh)

	aer := <-executionCh
	assert.Equal(t, b.Hash(), aer.Container)
	aer = <-executionCh
	assert.Equal(t, b.Hash(), aer.Container)

	notif := <-notificationCh
	require.Equal(t, bc.UtilityTokenHash(), notif.ScriptHash)

	script := io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("yay!"))
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeNotify)
	require.NoError(t, script.Err)
	txGood1 := e.PrepareInvocation(t, script.Bytes(), []neotest.Signer{acc})

	// Reset() reuses the script buffer and we need to keep scripts.
	script = io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("nay!"))
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeNotify)
	emit.Opcodes(script.BinWriter, opcode.THROW)
	require.NoError(t, script.Err)
	txBad := e.PrepareInvocation(t, script.Bytes(), []neotest.Signer{acc})

	script = io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("yay! yay! yay!"))
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeNotify)
	require.NoError(t, script.Err)
	txGood2 := e.PrepareInvocation(t, script.Bytes(), []neotest.Signer{acc})

	invBlock := e.AddNewBlock(t, txGood1, txBad, txGood2)

	require.Eventually(t, func() bool {
		return len(blockCh) != 0 && len(txCh) != 0 &&
			len(notificationCh) != 0 && len(executionCh) != 0
	}, time.Second, 10*time.Millisecond)

	b = <-blockCh
	require.Equal(t, invBlock, b)
	assert.Empty(t, blockCh)

	exec := <-executionCh
	require.Equal(t, b.Hash(), exec.Container)
	require.Equal(t, exec.VMState, vmstate.Halt)

	// 3 burn events for every tx and 1 mint for primary node
	require.True(t, len(notificationCh) >= 4)
	for i := 0; i < 4; i++ {
		notif := <-notificationCh
		require.Equal(t, nativeGASHash, notif.ScriptHash)
	}

	// Follow in-block transaction order.
	for _, txExpected := range invBlock.Transactions {
		tx := <-txCh
		require.Equal(t, txExpected, tx)
		exec := <-executionCh
		require.Equal(t, tx.Hash(), exec.Container)
		if exec.VMState == vmstate.Halt {
			notif := <-notificationCh
			require.Equal(t, hash.Hash160(tx.Script), notif.ScriptHash)
		}
	}
	assert.Empty(t, txCh)
	assert.Len(t, notificationCh, 1)
	assert.Len(t, executionCh, 1)

	notif = <-notificationCh
	require.Equal(t, bc.UtilityTokenHash(), notif.ScriptHash)

	exec = <-executionCh
	require.Equal(t, b.Hash(), exec.Container)
	require.Equal(t, exec.VMState, vmstate.Halt)

	bc.UnsubscribeFromBlocks(blockCh)
	bc.UnsubscribeFromTransactions(txCh)
	bc.UnsubscribeFromNotifications(notificationCh)
	bc.UnsubscribeFromExecutions(executionCh)

	// Ensure that new blocks are processed correctly after unsubscription.
	e.GenerateNewBlocks(t, 2*chBufSize)
}

func TestBlockchain_RemoveUntraceable(t *testing.T) {
	neoCommitteeKey := []byte{0xfb, 0xff, 0xff, 0xff, 0x0e}
	check := func(t *testing.T, bc *core.Blockchain, tHash, bHash, sHash util.Uint256, errorExpected bool) {
		_, _, err := bc.GetTransaction(tHash)
		if errorExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		_, err = bc.GetAppExecResults(tHash, trigger.Application)
		if errorExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		_, err = bc.GetBlock(bHash)
		if errorExpected {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		_, err = bc.GetHeader(bHash)
		require.NoError(t, err)
		if !sHash.Equals(util.Uint256{}) {
			sm := bc.GetStateModule()
			_, err = sm.GetState(sHash, neoCommitteeKey)
			if errorExpected {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}
	}
	t.Run("P2PStateExchangeExtensions off", func(t *testing.T) {
		bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
			c.MaxTraceableBlocks = 2
			c.Ledger.GarbageCollectionPeriod = 2
			c.Ledger.RemoveUntraceableBlocks = true
		})
		e := neotest.NewExecutor(t, bc, acc, acc)
		neoValidatorInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Neo))

		tx1Hash := neoValidatorInvoker.Invoke(t, true, "transfer", acc.ScriptHash(), util.Uint160{1, 2, 3}, 1, nil)
		tx1Height := bc.BlockHeight()
		b1 := e.TopBlock(t)
		sRoot, err := bc.GetStateModule().GetStateRoot(tx1Height)
		require.NoError(t, err)

		neoValidatorInvoker.Invoke(t, true, "transfer", acc.ScriptHash(), util.Uint160{1, 2, 3}, 1, nil)

		_, h1, err := bc.GetTransaction(tx1Hash)
		require.NoError(t, err)
		require.Equal(t, tx1Height, h1)

		check(t, bc, tx1Hash, b1.Hash(), sRoot.Root, false)
		e.GenerateNewBlocks(t, 4)

		sm := bc.GetStateModule()
		require.Eventually(t, func() bool {
			_, err = sm.GetState(sRoot.Root, neoCommitteeKey)
			return err != nil
		}, 2*bcPersistInterval, 10*time.Millisecond)
		check(t, bc, tx1Hash, b1.Hash(), sRoot.Root, true)
	})
	t.Run("P2PStateExchangeExtensions on", func(t *testing.T) {
		bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
			c.MaxTraceableBlocks = 2
			c.Ledger.GarbageCollectionPeriod = 2
			c.Ledger.RemoveUntraceableBlocks = true
			c.P2PStateExchangeExtensions = true
			c.StateSyncInterval = 2
			c.StateRootInHeader = true
		})
		e := neotest.NewExecutor(t, bc, acc, acc)
		neoValidatorInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Neo))

		tx1Hash := neoValidatorInvoker.Invoke(t, true, "transfer", acc.ScriptHash(), util.Uint160{1, 2, 3}, 1, nil)
		tx1Height := bc.BlockHeight()
		b1 := e.TopBlock(t)
		sRoot, err := bc.GetStateModule().GetStateRoot(tx1Height)
		require.NoError(t, err)

		tx2Hash := neoValidatorInvoker.Invoke(t, true, "transfer", acc.ScriptHash(), util.Uint160{1, 2, 3}, 1, nil)
		tx2Height := bc.BlockHeight()
		b2 := e.TopBlock(t)

		_, h1, err := bc.GetTransaction(tx1Hash)
		require.NoError(t, err)
		require.Equal(t, tx1Height, h1)

		e.GenerateNewBlocks(t, 3)

		check(t, bc, tx1Hash, b1.Hash(), sRoot.Root, false)
		check(t, bc, tx2Hash, b2.Hash(), sRoot.Root, false)

		e.AddNewBlock(t)

		check(t, bc, tx1Hash, b1.Hash(), util.Uint256{}, true)
		check(t, bc, tx2Hash, b2.Hash(), util.Uint256{}, false)
		_, h2, err := bc.GetTransaction(tx2Hash)
		require.NoError(t, err)
		require.Equal(t, tx2Height, h2)
	})
}

func TestBlockchain_InvalidNotification(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	cs, _ := contracts.GetTestContractState(t, pathToInternalContracts, 0, 1, acc.ScriptHash())
	e.DeployContract(t, &neotest.Contract{
		Hash:     cs.Hash,
		NEF:      &cs.NEF,
		Manifest: &cs.Manifest,
	}, nil)

	cValidatorInvoker := e.ValidatorInvoker(cs.Hash)
	cValidatorInvoker.InvokeAndCheck(t, func(t testing.TB, stack []stackitem.Item) {
		require.Equal(t, 1, len(stack))
		require.Nil(t, stack[0])
	}, "invalidStack1")
	cValidatorInvoker.Invoke(t, stackitem.NewInterop(nil), "invalidStack2")
}

// Test that deletion of non-existent doesn't result in error in tx or block addition.
func TestBlockchain_MPTDeleteNoKey(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)

	cs, _ := contracts.GetTestContractState(t, pathToInternalContracts, 0, 1, acc.ScriptHash())
	e.DeployContract(t, &neotest.Contract{
		Hash:     cs.Hash,
		NEF:      &cs.NEF,
		Manifest: &cs.Manifest,
	}, nil)

	cValidatorInvoker := e.ValidatorInvoker(cs.Hash)
	cValidatorInvoker.Invoke(t, stackitem.Null{}, "delValue", "non-existent-key")
}

// Test that all default configurations are loadable.
func TestConfig_LoadDefaultConfigs(t *testing.T) {
	var prefixPath = filepath.Join("..", "..", "config")
	check := func(t *testing.T, cfgFileSuffix any) {
		cfgPath := filepath.Join(prefixPath, fmt.Sprintf("protocol.%s.yml", cfgFileSuffix))
		_, err := config.LoadFile(cfgPath)
		require.NoError(t, err, fmt.Errorf("failed to load %s", cfgPath))
	}
	testCases := []any{
		netmode.MainNet,
		netmode.PrivNet,
		netmode.TestNet,
		netmode.UnitTestNet,
		"privnet.docker.one",
		"privnet.docker.two",
		"privnet.docker.three",
		"privnet.docker.four",
		"privnet.docker.single",
		"unit_testnet.single",
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s", tc), func(t *testing.T) {
			check(t, tc)
		})
	}
}

func TestBlockchain_VerifyTx(t *testing.T) {
	bc, validator, committee := chain.NewMultiWithCustomConfig(t, func(c *config.Blockchain) {
		c.P2PSigExtensions = true
		c.ReservedAttributes = true
	})
	e := neotest.NewExecutor(t, bc, validator, committee)

	accs := make([]*wallet.Account, 5)
	for i := range accs {
		var err error
		accs[i], err = wallet.NewAccount()
		require.NoError(t, err)
	}

	notaryServiceFeePerKey := bc.GetNotaryServiceFeePerKey()

	oracleAcc := accs[2]
	oraclePubs := keys.PublicKeys{oracleAcc.PublicKey()}
	require.NoError(t, oracleAcc.ConvertMultisig(1, oraclePubs))

	neoHash := e.NativeHash(t, nativenames.Neo)
	gasHash := e.NativeHash(t, nativenames.Gas)
	policyHash := e.NativeHash(t, nativenames.Policy)
	designateHash := e.NativeHash(t, nativenames.Designation)
	notaryHash := e.NativeHash(t, nativenames.Notary)
	oracleHash := e.NativeHash(t, nativenames.Oracle)

	neoValidatorsInvoker := e.ValidatorInvoker(neoHash)
	gasValidatorsInvoker := e.ValidatorInvoker(gasHash)
	policySuperInvoker := e.NewInvoker(policyHash, validator, committee)
	designateSuperInvoker := e.NewInvoker(designateHash, validator, committee)
	neoOwner := validator.ScriptHash()

	neoAmount := int64(1_000_000)
	gasAmount := int64(1_000_000_000)
	txs := make([]*transaction.Transaction, 0, 2*len(accs)+2)
	for _, a := range accs {
		txs = append(txs, neoValidatorsInvoker.PrepareInvoke(t, "transfer", neoOwner, a.Contract.ScriptHash(), neoAmount, nil))
		txs = append(txs, gasValidatorsInvoker.PrepareInvoke(t, "transfer", neoOwner, a.Contract.ScriptHash(), gasAmount, nil))
	}
	txs = append(txs, neoValidatorsInvoker.PrepareInvoke(t, "transfer", neoOwner, committee.ScriptHash(), neoAmount, nil))
	txs = append(txs, gasValidatorsInvoker.PrepareInvoke(t, "transfer", neoOwner, committee.ScriptHash(), gasAmount, nil))
	e.AddNewBlock(t, txs...)
	for _, tx := range txs {
		e.CheckHalt(t, tx.Hash(), stackitem.NewBool(true))
	}
	policySuperInvoker.Invoke(t, true, "blockAccount", accs[1].PrivateKey().GetScriptHash().BytesBE())

	checkErr := func(t *testing.T, expectedErr error, tx *transaction.Transaction) {
		err := bc.VerifyTx(tx)
		require.ErrorIs(t, err, expectedErr)
	}

	testScript := []byte{byte(opcode.PUSH1)}
	newTestTx := func(t *testing.T, signer util.Uint160, script []byte) *transaction.Transaction {
		tx := transaction.New(script, 1_000_000)
		tx.Nonce = neotest.Nonce()
		tx.ValidUntilBlock = e.Chain.BlockHeight() + 5
		tx.Signers = []transaction.Signer{{
			Account: signer,
			Scopes:  transaction.CalledByEntry,
		}}
		tx.NetworkFee = int64(io.GetVarSize(tx)+200 /* witness */) * bc.FeePerByte()
		tx.NetworkFee += 1_000_000 // verification cost
		return tx
	}

	h := accs[0].PrivateKey().GetScriptHash()
	t.Run("Expired", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		tx.ValidUntilBlock = 1
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, core.ErrTxExpired, tx)
	})
	t.Run("BlockedAccount", func(t *testing.T) {
		tx := newTestTx(t, accs[1].PrivateKey().GetScriptHash(), testScript)
		require.NoError(t, accs[1].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, core.ErrPolicy, tx)
	})
	t.Run("InsufficientGas", func(t *testing.T) {
		balance := bc.GetUtilityTokenBalance(h)
		tx := newTestTx(t, h, testScript)
		tx.SystemFee = balance.Int64() + 1
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, core.ErrInsufficientFunds, tx)
	})
	t.Run("TooBigSystemFee", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		tx.SystemFee = bc.GetConfig().MaxBlockSystemFee + 100500
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, core.ErrPolicy, tx)
	})
	t.Run("TooBigTx", func(t *testing.T) {
		script := make([]byte, transaction.MaxTransactionSize)
		tx := newTestTx(t, h, script)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, core.ErrTxTooBig, tx)
	})
	t.Run("NetworkFee", func(t *testing.T) {
		t.Run("SmallNetworkFee", func(t *testing.T) {
			tx := newTestTx(t, h, testScript)
			tx.NetworkFee = 1
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			checkErr(t, core.ErrTxSmallNetworkFee, tx)
		})
		t.Run("AlmostEnoughNetworkFee", func(t *testing.T) {
			tx := newTestTx(t, h, testScript)
			verificationNetFee, calcultedScriptSize := fee.Calculate(bc.GetBaseExecFee(), accs[0].Contract.Script)
			expectedSize := io.GetVarSize(tx) + calcultedScriptSize
			calculatedNetFee := verificationNetFee + int64(expectedSize)*bc.FeePerByte()
			tx.NetworkFee = calculatedNetFee - 1
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			require.Equal(t, expectedSize, io.GetVarSize(tx))
			checkErr(t, core.ErrVerificationFailed, tx)
		})
		t.Run("EnoughNetworkFee", func(t *testing.T) {
			tx := newTestTx(t, h, testScript)
			verificationNetFee, calcultedScriptSize := fee.Calculate(bc.GetBaseExecFee(), accs[0].Contract.Script)
			expectedSize := io.GetVarSize(tx) + calcultedScriptSize
			calculatedNetFee := verificationNetFee + int64(expectedSize)*bc.FeePerByte()
			tx.NetworkFee = calculatedNetFee
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			require.Equal(t, expectedSize, io.GetVarSize(tx))
			require.NoError(t, bc.VerifyTx(tx))
		})
		t.Run("CalculateNetworkFee, signature script", func(t *testing.T) {
			tx := newTestTx(t, h, testScript)
			expectedSize := io.GetVarSize(tx)
			verificationNetFee, calculatedScriptSize := fee.Calculate(bc.GetBaseExecFee(), accs[0].Contract.Script)
			expectedSize += calculatedScriptSize
			expectedNetFee := verificationNetFee + int64(expectedSize)*bc.FeePerByte()
			tx.NetworkFee = expectedNetFee
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			actualSize := io.GetVarSize(tx)
			require.Equal(t, expectedSize, actualSize)
			gasConsumed, err := bc.VerifyWitness(h, tx, &tx.Scripts[0], -1)
			require.NoError(t, err)
			require.Equal(t, verificationNetFee, gasConsumed)
			require.Equal(t, expectedNetFee, bc.FeePerByte()*int64(actualSize)+gasConsumed)
		})
		t.Run("CalculateNetworkFee, multisignature script", func(t *testing.T) {
			multisigAcc := accs[4]
			pKeys := keys.PublicKeys{multisigAcc.PublicKey()}
			require.NoError(t, multisigAcc.ConvertMultisig(1, pKeys))
			multisigHash := hash.Hash160(multisigAcc.Contract.Script)
			tx := newTestTx(t, multisigHash, testScript)
			verificationNetFee, calculatedScriptSize := fee.Calculate(bc.GetBaseExecFee(), multisigAcc.Contract.Script)
			expectedSize := io.GetVarSize(tx) + calculatedScriptSize
			expectedNetFee := verificationNetFee + int64(expectedSize)*bc.FeePerByte()
			tx.NetworkFee = expectedNetFee
			require.NoError(t, multisigAcc.SignTx(netmode.UnitTestNet, tx))
			actualSize := io.GetVarSize(tx)
			require.Equal(t, expectedSize, actualSize)
			gasConsumed, err := bc.VerifyWitness(multisigHash, tx, &tx.Scripts[0], -1)
			require.NoError(t, err)
			require.Equal(t, verificationNetFee, gasConsumed)
			require.Equal(t, expectedNetFee, bc.FeePerByte()*int64(actualSize)+gasConsumed)
		})
	})
	t.Run("InvalidTxScript", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		tx.Script = append(tx.Script, 0xff)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, core.ErrInvalidScript, tx)
	})
	t.Run("InvalidVerificationScript", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		verif := []byte{byte(opcode.JMP), 3, 0xff, byte(opcode.PUSHT)}
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: hash.Hash160(verif),
			Scopes:  transaction.Global,
		})
		tx.NetworkFee += 1000000
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		tx.Scripts = append(tx.Scripts, transaction.Witness{
			InvocationScript:   []byte{},
			VerificationScript: verif,
		})
		checkErr(t, core.ErrInvalidVerificationScript, tx)
	})
	t.Run("InvalidInvocationScript", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		verif := []byte{byte(opcode.PUSHT)}
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: hash.Hash160(verif),
			Scopes:  transaction.Global,
		})
		tx.NetworkFee += 1000000
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		tx.Scripts = append(tx.Scripts, transaction.Witness{
			InvocationScript:   []byte{byte(opcode.JMP), 3, 0xff},
			VerificationScript: verif,
		})
		checkErr(t, core.ErrInvalidInvocationScript, tx)
	})
	t.Run("Conflict", func(t *testing.T) {
		balance := bc.GetUtilityTokenBalance(h).Int64()
		tx := newTestTx(t, h, testScript)
		tx.NetworkFee = balance / 2
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		require.NoError(t, bc.PoolTx(tx))

		tx2 := newTestTx(t, h, testScript)
		tx2.NetworkFee = balance / 2
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx2))
		err := bc.PoolTx(tx2)
		require.ErrorIs(t, err, core.ErrMemPoolConflict)
	})
	t.Run("InvalidWitnessHash", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		tx.Scripts[0].VerificationScript = []byte{byte(opcode.PUSHT)}
		checkErr(t, core.ErrWitnessHashMismatch, tx)
	})
	t.Run("InvalidWitnessSignature", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		tx.Scripts[0].InvocationScript[10] = ^tx.Scripts[0].InvocationScript[10]
		checkErr(t, core.ErrVerificationFailed, tx)
	})
	t.Run("InsufficientNetworkFeeForSecondWitness", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: accs[3].PrivateKey().GetScriptHash(),
			Scopes:  transaction.Global,
		})
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		require.NoError(t, accs[3].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, core.ErrVerificationFailed, tx)
	})
	t.Run("OldTX", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		e.AddNewBlock(t, tx)

		checkErr(t, core.ErrAlreadyExists, tx)
	})
	t.Run("MemPooledTX", func(t *testing.T) {
		tx := newTestTx(t, h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		require.NoError(t, bc.PoolTx(tx))

		err := bc.PoolTx(tx)
		require.ErrorIs(t, err, core.ErrAlreadyInPool)
	})
	t.Run("MemPoolOOM", func(t *testing.T) {
		mp := mempool.New(1, 0, false, nil)
		tx1 := newTestTx(t, h, testScript)
		tx1.NetworkFee += 10000 // Give it more priority.
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx1))
		require.NoError(t, bc.PoolTx(tx1, mp))

		tx2 := newTestTx(t, h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx2))
		err := bc.PoolTx(tx2, mp)
		require.ErrorIs(t, err, core.ErrOOM)
	})
	t.Run("Attribute", func(t *testing.T) {
		t.Run("InvalidHighPriority", func(t *testing.T) {
			tx := newTestTx(t, h, testScript)
			tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.HighPriority})
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			checkErr(t, core.ErrInvalidAttribute, tx)
		})
		t.Run("ValidHighPriority", func(t *testing.T) {
			tx := newTestTx(t, h, testScript)
			tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.HighPriority})
			tx.NetworkFee += 4_000_000 // multisig check
			tx.Signers = []transaction.Signer{{
				Account: committee.ScriptHash(),
				Scopes:  transaction.None,
			}}
			rawScript := committee.Script()
			size := io.GetVarSize(tx)
			netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), rawScript)
			tx.NetworkFee += netFee
			tx.NetworkFee += int64(size+sizeDelta) * bc.FeePerByte()
			tx.Scripts = []transaction.Witness{{
				InvocationScript:   committee.SignHashable(uint32(netmode.UnitTestNet), tx),
				VerificationScript: rawScript,
			}}
			require.NoError(t, bc.VerifyTx(tx))
		})
		t.Run("Oracle", func(t *testing.T) {
			cs := contracts.GetOracleContractState(t, pathToInternalContracts, validator.ScriptHash(), 0)
			e.DeployContract(t, &neotest.Contract{
				Hash:     cs.Hash,
				NEF:      &cs.NEF,
				Manifest: &cs.Manifest,
			}, nil)
			cInvoker := e.ValidatorInvoker(cs.Hash)

			const gasForResponse int64 = 10_000_000
			cInvoker.Invoke(t, stackitem.Null{}, "requestURL", "https://get.1234", "", "handle", []byte{}, gasForResponse)

			oracleScript, err := smartcontract.CreateMajorityMultiSigRedeemScript(oraclePubs)
			require.NoError(t, err)
			oracleMultisigHash := hash.Hash160(oracleScript)

			respScript := native.CreateOracleResponseScript(oracleHash)

			// We need to create new transaction,
			// because hashes are cached after signing.
			getOracleTx := func(t *testing.T) *transaction.Transaction {
				tx := transaction.New(respScript, 1000_0000)
				tx.Nonce = neotest.Nonce()
				tx.ValidUntilBlock = bc.BlockHeight() + 1
				resp := &transaction.OracleResponse{
					ID:     0,
					Code:   transaction.Success,
					Result: []byte{1, 2, 3},
				}
				tx.Attributes = []transaction.Attribute{{
					Type:  transaction.OracleResponseT,
					Value: resp,
				}}
				tx.NetworkFee += 4_000_000 // multisig check
				tx.SystemFee = gasForResponse - tx.NetworkFee
				tx.Signers = []transaction.Signer{{
					Account: oracleMultisigHash,
					Scopes:  transaction.None,
				}}
				size := io.GetVarSize(tx)
				netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), oracleScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * bc.FeePerByte()
				return tx
			}

			t.Run("NoOracleNodes", func(t *testing.T) {
				tx := getOracleTx(t)
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, core.ErrInvalidAttribute, tx)
			})

			keys := make([]any, 0, len(oraclePubs))
			for _, p := range oraclePubs {
				keys = append(keys, p.Bytes())
			}
			designateSuperInvoker.Invoke(t, stackitem.Null{}, "designateAsRole",
				int64(noderoles.Oracle), keys)

			t.Run("Valid", func(t *testing.T) {
				tx := getOracleTx(t)
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				require.NoError(t, bc.VerifyTx(tx))

				t.Run("NativeVerify", func(t *testing.T) {
					tx.Signers = append(tx.Signers, transaction.Signer{
						Account: oracleHash,
						Scopes:  transaction.None,
					})
					tx.Scripts = append(tx.Scripts, transaction.Witness{})
					t.Run("NonZeroVerification", func(t *testing.T) {
						w := io.NewBufBinWriter()
						emit.Opcodes(w.BinWriter, opcode.ABORT)
						emit.Bytes(w.BinWriter, util.Uint160{}.BytesBE())
						emit.Int(w.BinWriter, 0)
						emit.String(w.BinWriter, nativenames.Oracle)
						tx.Scripts[len(tx.Scripts)-1].VerificationScript = w.Bytes()
						err := bc.VerifyTx(tx)
						require.ErrorIs(t, err, core.ErrNativeContractWitness)
					})
					t.Run("Good", func(t *testing.T) {
						tx.Scripts[len(tx.Scripts)-1].VerificationScript = nil
						require.NoError(t, bc.VerifyTx(tx))
					})
				})
			})
			t.Run("InvalidRequestID", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.Attributes[0].Value.(*transaction.OracleResponse).ID = 2
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, core.ErrInvalidAttribute, tx)
			})
			t.Run("InvalidScope", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.Signers[0].Scopes = transaction.Global
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, core.ErrInvalidAttribute, tx)
			})
			t.Run("InvalidScript", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.Script = append(tx.Script, byte(opcode.NOP))
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, core.ErrInvalidAttribute, tx)
			})
			t.Run("InvalidSigner", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.Signers[0].Account = accs[0].Contract.ScriptHash()
				require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
				checkErr(t, core.ErrInvalidAttribute, tx)
			})
			t.Run("SmallFee", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.SystemFee = 0
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, core.ErrInvalidAttribute, tx)
			})
		})
		t.Run("NotValidBefore", func(t *testing.T) {
			getNVBTx := func(e *neotest.Executor, height uint32) *transaction.Transaction {
				tx := newTestTx(t, h, testScript)
				tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: height}})
				tx.NetworkFee += 4_000_000 // multisig check
				tx.Signers = []transaction.Signer{{
					Account: e.Validator.ScriptHash(),
					Scopes:  transaction.None,
				}}
				size := io.GetVarSize(tx)
				rawScript := e.Validator.Script()
				netFee, sizeDelta := fee.Calculate(e.Chain.GetBaseExecFee(), rawScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * e.Chain.FeePerByte()
				tx.Scripts = []transaction.Witness{{
					InvocationScript:   e.Validator.SignHashable(uint32(netmode.UnitTestNet), tx),
					VerificationScript: rawScript,
				}}
				return tx
			}
			t.Run("Disabled", func(t *testing.T) { // check that NVB attribute is not an extension anymore.
				bcBad, validatorBad, committeeBad := chain.NewMultiWithCustomConfig(t, func(c *config.Blockchain) {
					c.P2PSigExtensions = false
					c.ReservedAttributes = false
				})
				eBad := neotest.NewExecutor(t, bcBad, validatorBad, committeeBad)
				tx := getNVBTx(eBad, bcBad.BlockHeight())
				err := bcBad.VerifyTx(tx)
				require.NoError(t, err)
			})
			t.Run("Enabled", func(t *testing.T) {
				t.Run("NotYetValid", func(t *testing.T) {
					tx := getNVBTx(e, bc.BlockHeight()+1)
					require.ErrorIs(t, bc.VerifyTx(tx), core.ErrInvalidAttribute)
				})
				t.Run("positive", func(t *testing.T) {
					tx := getNVBTx(e, bc.BlockHeight())
					require.NoError(t, bc.VerifyTx(tx))
				})
			})
		})
		t.Run("Reserved", func(t *testing.T) {
			getReservedTx := func(e *neotest.Executor, attrType transaction.AttrType) *transaction.Transaction {
				tx := newTestTx(t, h, testScript)
				tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: attrType, Value: &transaction.Reserved{Value: []byte{1, 2, 3}}})
				tx.NetworkFee += 4_000_000 // multisig check
				tx.Signers = []transaction.Signer{{
					Account: e.Validator.ScriptHash(),
					Scopes:  transaction.None,
				}}
				rawScript := e.Validator.Script()
				size := io.GetVarSize(tx)
				netFee, sizeDelta := fee.Calculate(e.Chain.GetBaseExecFee(), rawScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * e.Chain.FeePerByte()
				tx.Scripts = []transaction.Witness{{
					InvocationScript:   e.Validator.SignHashable(uint32(netmode.UnitTestNet), tx),
					VerificationScript: rawScript,
				}}
				return tx
			}
			t.Run("Disabled", func(t *testing.T) {
				bcBad, validatorBad, committeeBad := chain.NewMultiWithCustomConfig(t, func(c *config.Blockchain) {
					c.P2PSigExtensions = false
					c.ReservedAttributes = false
				})
				eBad := neotest.NewExecutor(t, bcBad, validatorBad, committeeBad)
				tx := getReservedTx(eBad, transaction.ReservedLowerBound+3)
				err := bcBad.VerifyTx(tx)
				require.Error(t, err)
				require.True(t, strings.Contains(err.Error(), "invalid attribute: attribute of reserved type was found, but ReservedAttributes are disabled"))
			})
			t.Run("Enabled", func(t *testing.T) {
				tx := getReservedTx(e, transaction.ReservedLowerBound+3)
				require.NoError(t, bc.VerifyTx(tx))
			})
		})
		t.Run("Conflicts", func(t *testing.T) {
			getConflictsTx := func(e *neotest.Executor, hashes ...util.Uint256) *transaction.Transaction {
				tx := newTestTx(t, h, testScript)
				tx.Attributes = make([]transaction.Attribute, len(hashes))
				for i, h := range hashes {
					tx.Attributes[i] = transaction.Attribute{
						Type: transaction.ConflictsT,
						Value: &transaction.Conflicts{
							Hash: h,
						},
					}
				}
				tx.NetworkFee += 4_000_000 // multisig check
				tx.Signers = []transaction.Signer{{
					Account: e.Validator.ScriptHash(),
					Scopes:  transaction.None,
				}}
				rawScript := e.Validator.Script()
				size := io.GetVarSize(tx)
				netFee, sizeDelta := fee.Calculate(e.Chain.GetBaseExecFee(), rawScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * e.Chain.FeePerByte()
				tx.Scripts = []transaction.Witness{{
					InvocationScript:   e.Validator.SignHashable(uint32(netmode.UnitTestNet), tx),
					VerificationScript: rawScript,
				}}
				return tx
			}
			t.Run("disabled", func(t *testing.T) { // check that Conflicts attribute is not an extension anymore.
				bcBad, validatorBad, committeeBad := chain.NewMultiWithCustomConfig(t, func(c *config.Blockchain) {
					c.P2PSigExtensions = false
					c.ReservedAttributes = false
				})
				eBad := neotest.NewExecutor(t, bcBad, validatorBad, committeeBad)
				tx := getConflictsTx(eBad, util.Uint256{1, 2, 3})
				err := bcBad.VerifyTx(tx)
				require.NoError(t, err)
			})
			t.Run("enabled", func(t *testing.T) {
				t.Run("dummy on-chain conflict", func(t *testing.T) {
					t.Run("on-chain conflict signed by malicious party", func(t *testing.T) {
						tx := newTestTx(t, h, testScript)
						require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
						conflicting := transaction.New([]byte{byte(opcode.RET)}, 1000_0000)
						conflicting.ValidUntilBlock = bc.BlockHeight() + 1
						conflicting.Signers = []transaction.Signer{
							{
								Account: validator.ScriptHash(),
								Scopes:  transaction.CalledByEntry,
							},
						}
						conflicting.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: tx.Hash(),
								},
							},
						}
						conflicting.NetworkFee = 1000_0000
						require.NoError(t, validator.SignTx(netmode.UnitTestNet, conflicting))
						e.AddNewBlock(t, conflicting)
						// We expect `tx` to pass verification, because on-chained `conflicting` doesn't have
						// `tx`'s payer in the signers list, thus, `conflicting` should be considered as
						// malicious conflict.
						require.NoError(t, bc.VerifyTx(tx))
					})
					t.Run("multiple on-chain conflicts signed by malicious parties", func(t *testing.T) {
						m1 := e.NewAccount(t)
						m2 := e.NewAccount(t)
						m3 := e.NewAccount(t)
						good := e.NewAccount(t)

						// txGood doesn't conflict with anyone and signed by good signer.
						txGood := newTestTx(t, good.ScriptHash(), testScript)
						require.NoError(t, good.SignTx(netmode.UnitTestNet, txGood))

						// txM1 conflicts with txGood and signed by two malicious signers.
						txM1 := newTestTx(t, m1.ScriptHash(), testScript)
						txM1.Signers = append(txM1.Signers, transaction.Signer{Account: m2.ScriptHash()})
						txM1.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: txGood.Hash(),
								},
							},
						}
						txM1.NetworkFee = 1_000_0000
						require.NoError(t, m1.SignTx(netmode.UnitTestNet, txM1))
						require.NoError(t, m2.SignTx(netmode.UnitTestNet, txM1))
						e.AddNewBlock(t, txM1)

						// txM2 conflicts with txGood and signed by one malicious signer.
						txM2 := newTestTx(t, m3.ScriptHash(), testScript)
						txM2.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: txGood.Hash(),
								},
							},
						}
						txM2.NetworkFee = 1_000_0000
						require.NoError(t, m3.SignTx(netmode.UnitTestNet, txM2))
						e.AddNewBlock(t, txM2)

						// We expect `tx` to pass verification, because on-chained `conflicting` doesn't have
						// `tx`'s payer in the signers list, thus, `conflicting` should be considered as
						// malicious conflict.
						require.NoError(t, bc.VerifyTx(txGood))

						// After that txGood can be added to the chain normally.
						e.AddNewBlock(t, txGood)

						// And after that ErrAlreadyExist is expected on verification.
						require.ErrorIs(t, bc.VerifyTx(txGood), core.ErrAlreadyExists)
					})

					t.Run("multiple on-chain conflicts signed by [valid+malicious] parties", func(t *testing.T) {
						m1 := e.NewAccount(t)
						m2 := e.NewAccount(t)
						m3 := e.NewAccount(t)
						good := e.NewAccount(t)

						// txGood doesn't conflict with anyone and signed by good signer.
						txGood := newTestTx(t, good.ScriptHash(), testScript)
						require.NoError(t, good.SignTx(netmode.UnitTestNet, txGood))

						// txM1 conflicts with txGood and signed by one malicious and one good signers.
						txM1 := newTestTx(t, m1.ScriptHash(), testScript)
						txM1.Signers = append(txM1.Signers, transaction.Signer{Account: good.ScriptHash()})
						txM1.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: txGood.Hash(),
								},
							},
						}
						txM1.NetworkFee = 1_000_0000
						require.NoError(t, m1.SignTx(netmode.UnitTestNet, txM1))
						require.NoError(t, good.SignTx(netmode.UnitTestNet, txM1))
						e.AddNewBlock(t, txM1)

						// txM2 conflicts with txGood and signed by two malicious signers.
						txM2 := newTestTx(t, m2.ScriptHash(), testScript)
						txM2.Signers = append(txM2.Signers, transaction.Signer{Account: m3.ScriptHash()})
						txM2.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: txGood.Hash(),
								},
							},
						}
						txM2.NetworkFee = 1_000_0000
						require.NoError(t, m2.SignTx(netmode.UnitTestNet, txM2))
						require.NoError(t, m3.SignTx(netmode.UnitTestNet, txM2))
						e.AddNewBlock(t, txM2)

						// We expect `tx` to fail verification, because one of the on-chained `conflicting`
						// transactions has common signers with `tx`, thus, `conflicting` should be
						// considered as a valid conflict.
						require.ErrorIs(t, bc.VerifyTx(txGood), core.ErrHasConflicts)
					})

					t.Run("multiple on-chain conflicts signed by [malicious+valid] parties", func(t *testing.T) {
						m1 := e.NewAccount(t)
						m2 := e.NewAccount(t)
						m3 := e.NewAccount(t)
						good := e.NewAccount(t)

						// txGood doesn't conflict with anyone and signed by good signer.
						txGood := newTestTx(t, good.ScriptHash(), testScript)
						require.NoError(t, good.SignTx(netmode.UnitTestNet, txGood))

						// txM2 conflicts with txGood and signed by two malicious signers.
						txM2 := newTestTx(t, m2.ScriptHash(), testScript)
						txM2.Signers = append(txM2.Signers, transaction.Signer{Account: m3.ScriptHash()})
						txM2.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: txGood.Hash(),
								},
							},
						}
						txM2.NetworkFee = 1_000_0000
						require.NoError(t, m2.SignTx(netmode.UnitTestNet, txM2))
						require.NoError(t, m3.SignTx(netmode.UnitTestNet, txM2))
						e.AddNewBlock(t, txM2)

						// txM1 conflicts with txGood and signed by one malicious and one good signers.
						txM1 := newTestTx(t, m1.ScriptHash(), testScript)
						txM1.Signers = append(txM1.Signers, transaction.Signer{Account: good.ScriptHash()})
						txM1.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: txGood.Hash(),
								},
							},
						}
						txM1.NetworkFee = 1_000_0000
						require.NoError(t, m1.SignTx(netmode.UnitTestNet, txM1))
						require.NoError(t, good.SignTx(netmode.UnitTestNet, txM1))
						e.AddNewBlock(t, txM1)

						// We expect `tx` to fail verification, because one of the on-chained `conflicting`
						// transactions has common signers with `tx`, thus, `conflicting` should be
						// considered as a valid conflict.
						require.ErrorIs(t, bc.VerifyTx(txGood), core.ErrHasConflicts)
					})

					t.Run("multiple on-chain conflicts signed by [valid + malicious + valid] parties", func(t *testing.T) {
						m1 := e.NewAccount(t)
						m2 := e.NewAccount(t)
						m3 := e.NewAccount(t)
						good := e.NewAccount(t)

						// txGood doesn't conflict with anyone and signed by good signer.
						txGood := newTestTx(t, good.ScriptHash(), testScript)
						require.NoError(t, good.SignTx(netmode.UnitTestNet, txGood))

						// txM1 conflicts with txGood and signed by one malicious and one good signers.
						txM1 := newTestTx(t, m1.ScriptHash(), testScript)
						txM1.Signers = append(txM1.Signers, transaction.Signer{Account: good.ScriptHash()})
						txM1.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: txGood.Hash(),
								},
							},
						}
						txM1.NetworkFee = 1_000_0000
						require.NoError(t, m1.SignTx(netmode.UnitTestNet, txM1))
						require.NoError(t, good.SignTx(netmode.UnitTestNet, txM1))
						e.AddNewBlock(t, txM1)

						// txM2 conflicts with txGood and signed by two malicious signers.
						txM2 := newTestTx(t, m2.ScriptHash(), testScript)
						txM2.Signers = append(txM2.Signers, transaction.Signer{Account: m3.ScriptHash()})
						txM2.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: txGood.Hash(),
								},
							},
						}
						txM2.NetworkFee = 1_000_0000
						require.NoError(t, m2.SignTx(netmode.UnitTestNet, txM2))
						require.NoError(t, m3.SignTx(netmode.UnitTestNet, txM2))
						e.AddNewBlock(t, txM2)

						// txM3 conflicts with txGood and signed by one good and one malicious signers.
						txM3 := newTestTx(t, good.ScriptHash(), testScript)
						txM3.Signers = append(txM3.Signers, transaction.Signer{Account: m1.ScriptHash()})
						txM3.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: txGood.Hash(),
								},
							},
						}
						txM3.NetworkFee = 1_000_0000
						require.NoError(t, good.SignTx(netmode.UnitTestNet, txM3))
						require.NoError(t, m1.SignTx(netmode.UnitTestNet, txM3))
						e.AddNewBlock(t, txM3)

						// We expect `tx` to fail verification, because one of the on-chained `conflicting`
						// transactions has common signers with `tx`, thus, `conflicting` should be
						// considered as a valid conflict.
						require.ErrorIs(t, bc.VerifyTx(txGood), core.ErrHasConflicts)
					})

					t.Run("on-chain conflict signed by single valid sender", func(t *testing.T) {
						tx := newTestTx(t, h, testScript)
						tx.Signers = []transaction.Signer{{Account: validator.ScriptHash()}}
						require.NoError(t, validator.SignTx(netmode.UnitTestNet, tx))
						conflicting := transaction.New([]byte{byte(opcode.RET)}, 1000_0000)
						conflicting.ValidUntilBlock = bc.BlockHeight() + 1
						conflicting.Signers = []transaction.Signer{
							{
								Account: validator.ScriptHash(),
								Scopes:  transaction.CalledByEntry,
							},
						}
						conflicting.Attributes = []transaction.Attribute{
							{
								Type: transaction.ConflictsT,
								Value: &transaction.Conflicts{
									Hash: tx.Hash(),
								},
							},
						}
						conflicting.NetworkFee = 1000_0000
						require.NoError(t, validator.SignTx(netmode.UnitTestNet, conflicting))
						e.AddNewBlock(t, conflicting)
						// We expect `tx` to fail verification, because on-chained `conflicting` has
						// `tx`'s payer as a signer.
						require.ErrorIs(t, bc.VerifyTx(tx), core.ErrHasConflicts)
					})
				})
				t.Run("attribute on-chain conflict", func(t *testing.T) {
					tx := neoValidatorsInvoker.Invoke(t, stackitem.NewBool(true), "transfer", neoOwner, neoOwner, 1, nil)
					txConflict := getConflictsTx(e, tx)
					require.Error(t, bc.VerifyTx(txConflict))
				})
				t.Run("positive", func(t *testing.T) {
					tx := getConflictsTx(e, random.Uint256())
					require.NoError(t, bc.VerifyTx(tx))
				})
			})
		})
		t.Run("NotaryAssisted", func(t *testing.T) {
			notary, err := wallet.NewAccount()
			require.NoError(t, err)
			designateSuperInvoker.Invoke(t, stackitem.Null{}, "designateAsRole",
				int64(noderoles.P2PNotary), []any{notary.PublicKey().Bytes()})
			txSetNotary := transaction.New([]byte{byte(opcode.RET)}, 0)
			txSetNotary.Signers = []transaction.Signer{
				{
					Account: committee.ScriptHash(),
					Scopes:  transaction.Global,
				},
			}
			txSetNotary.Scripts = []transaction.Witness{{
				InvocationScript:   e.Committee.SignHashable(uint32(netmode.UnitTestNet), txSetNotary),
				VerificationScript: e.Committee.Script(),
			}}

			getNotaryAssistedTx := func(e *neotest.Executor, signaturesCount uint8, serviceFee int64) *transaction.Transaction {
				tx := newTestTx(t, h, testScript)
				tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{
					NKeys: signaturesCount,
				}})
				tx.NetworkFee += serviceFee // additional fee for NotaryAssisted attribute
				tx.NetworkFee += 4_000_000  // multisig check
				tx.Signers = []transaction.Signer{{
					Account: e.CommitteeHash,
					Scopes:  transaction.None,
				},
					{
						Account: notaryHash,
						Scopes:  transaction.None,
					},
				}
				rawScript := committee.Script()
				size := io.GetVarSize(tx)
				netFee, sizeDelta := fee.Calculate(e.Chain.GetBaseExecFee(), rawScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * e.Chain.FeePerByte()
				tx.Scripts = []transaction.Witness{
					{
						InvocationScript:   committee.SignHashable(uint32(netmode.UnitTestNet), tx),
						VerificationScript: rawScript,
					},
					{
						InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, notary.PrivateKey().SignHashable(uint32(netmode.UnitTestNet), tx)...),
					},
				}
				return tx
			}
			t.Run("Disabled", func(t *testing.T) {
				bcBad, validatorBad, committeeBad := chain.NewMultiWithCustomConfig(t, func(c *config.Blockchain) {
					c.P2PSigExtensions = false
					c.ReservedAttributes = false
				})
				eBad := neotest.NewExecutor(t, bcBad, validatorBad, committeeBad)
				tx := transaction.New(testScript, 1_000_000)
				tx.Nonce = neotest.Nonce()
				tx.ValidUntilBlock = e.Chain.BlockHeight() + 5
				tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{NKeys: 0}})
				tx.NetworkFee = 1_0000_0000
				eBad.SignTx(t, tx, 1_0000_0000, eBad.Committee)
				err := bcBad.VerifyTx(tx)
				require.Error(t, err)
				require.True(t, strings.Contains(err.Error(), "invalid attribute: NotaryAssisted attribute was found, but P2PSigExtensions are disabled"))
			})
			t.Run("Enabled, insufficient network fee", func(t *testing.T) {
				tx := getNotaryAssistedTx(e, 1, 0)
				require.Error(t, bc.VerifyTx(tx))
			})
			t.Run("Test verify", func(t *testing.T) {
				t.Run("no NotaryAssisted attribute", func(t *testing.T) {
					tx := getNotaryAssistedTx(e, 1, (1+1)*notaryServiceFeePerKey)
					tx.Attributes = []transaction.Attribute{}
					tx.Signers = []transaction.Signer{
						{
							Account: committee.ScriptHash(),
							Scopes:  transaction.None,
						},
						{
							Account: notaryHash,
							Scopes:  transaction.None,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript:   committee.SignHashable(uint32(netmode.UnitTestNet), tx),
							VerificationScript: committee.Script(),
						},
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, notary.PrivateKey().SignHashable(uint32(netmode.UnitTestNet), tx)...),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("no deposit", func(t *testing.T) {
					tx := getNotaryAssistedTx(e, 1, (1+1)*notaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: notaryHash,
							Scopes:  transaction.None,
						},
						{
							Account: committee.ScriptHash(),
							Scopes:  transaction.None,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, notary.PrivateKey().SignHashable(uint32(netmode.UnitTestNet), tx)...),
						},
						{
							InvocationScript:   committee.SignHashable(uint32(netmode.UnitTestNet), tx),
							VerificationScript: committee.Script(),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("bad Notary signer scope", func(t *testing.T) {
					tx := getNotaryAssistedTx(e, 1, (1+1)*notaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: committee.ScriptHash(),
							Scopes:  transaction.None,
						},
						{
							Account: notaryHash,
							Scopes:  transaction.CalledByEntry,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript:   committee.SignHashable(uint32(netmode.UnitTestNet), tx),
							VerificationScript: committee.Script(),
						},
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, notary.PrivateKey().SignHashable(uint32(netmode.UnitTestNet), tx)...),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("not signed by Notary", func(t *testing.T) {
					tx := getNotaryAssistedTx(e, 1, (1+1)*notaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: committee.ScriptHash(),
							Scopes:  transaction.None,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript:   committee.SignHashable(uint32(netmode.UnitTestNet), tx),
							VerificationScript: committee.Script(),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("bad Notary node witness", func(t *testing.T) {
					tx := getNotaryAssistedTx(e, 1, (1+1)*notaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: committee.ScriptHash(),
							Scopes:  transaction.None,
						},
						{
							Account: notaryHash,
							Scopes:  transaction.None,
						},
					}
					acc, err := keys.NewPrivateKey()
					require.NoError(t, err)
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript:   committee.SignHashable(uint32(netmode.UnitTestNet), tx),
							VerificationScript: committee.Script(),
						},
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, acc.SignHashable(uint32(netmode.UnitTestNet), tx)...),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("missing payer", func(t *testing.T) {
					tx := getNotaryAssistedTx(e, 1, (1+1)*notaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: notaryHash,
							Scopes:  transaction.None,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, notary.PrivateKey().SignHashable(uint32(netmode.UnitTestNet), tx)...),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("positive", func(t *testing.T) {
					tx := getNotaryAssistedTx(e, 1, (1+1)*notaryServiceFeePerKey)
					require.NoError(t, bc.VerifyTx(tx))
				})
			})
		})
	})
	t.Run("Partially-filled transaction", func(t *testing.T) {
		getPartiallyFilledTx := func(nvb uint32, validUntil uint32) *transaction.Transaction {
			tx := newTestTx(t, h, testScript)
			tx.ValidUntilBlock = validUntil
			tx.Attributes = []transaction.Attribute{
				{
					Type:  transaction.NotValidBeforeT,
					Value: &transaction.NotValidBefore{Height: nvb},
				},
				{
					Type:  transaction.NotaryAssistedT,
					Value: &transaction.NotaryAssisted{NKeys: 0},
				},
			}
			tx.Signers = []transaction.Signer{
				{
					Account: notaryHash,
					Scopes:  transaction.None,
				},
				{
					Account: validator.ScriptHash(),
					Scopes:  transaction.None,
				},
			}
			size := io.GetVarSize(tx)
			netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), validator.Script())
			tx.NetworkFee = netFee + // multisig witness verification price
				int64(size)*bc.FeePerByte() + // fee for unsigned size
				int64(sizeDelta)*bc.FeePerByte() + // fee for multisig size
				66*bc.FeePerByte() + // fee for Notary signature size (66 bytes for Invocation script and 0 bytes for Verification script)
				2*bc.FeePerByte() + // fee for the length of each script in Notary witness (they are nil, so we did not take them into account during `size` calculation)
				notaryServiceFeePerKey + // fee for Notary attribute
				fee.Opcode(bc.GetBaseExecFee(), // Notary verification script
					opcode.PUSHDATA1, opcode.RET, // invocation script
					opcode.PUSH0, opcode.SYSCALL, opcode.RET) + // Neo.Native.Call
				nativeprices.NotaryVerificationPrice*bc.GetBaseExecFee() // Notary witness verification price
			tx.Scripts = []transaction.Witness{
				{
					InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, make([]byte, keys.SignatureLen)...),
					VerificationScript: []byte{},
				},
				{
					InvocationScript:   validator.SignHashable(uint32(netmode.UnitTestNet), tx),
					VerificationScript: validator.Script(),
				},
			}
			return tx
		}

		mp := mempool.New(10, 1, false, nil)
		verificationF := func(tx *transaction.Transaction, data any) error {
			if data.(int) > 5 {
				return errors.New("bad data")
			}
			return nil
		}
		t.Run("failed pre-verification", func(t *testing.T) {
			tx := getPartiallyFilledTx(bc.BlockHeight(), bc.BlockHeight()+1)
			require.Error(t, bc.PoolTxWithData(tx, 6, mp, bc, verificationF)) // here and below let's use `bc` instead of proper NotaryFeer for the test simplicity.
		})
		t.Run("GasLimitExceeded during witness verification", func(t *testing.T) {
			tx := getPartiallyFilledTx(bc.BlockHeight(), bc.BlockHeight()+1)
			tx.NetworkFee-- // to check that NetworkFee was set correctly in getPartiallyFilledTx
			tx.Scripts = []transaction.Witness{
				{
					InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, make([]byte, keys.SignatureLen)...),
					VerificationScript: []byte{},
				},
				{
					InvocationScript:   validator.SignHashable(uint32(netmode.UnitTestNet), tx),
					VerificationScript: validator.Script(),
				},
			}
			require.Error(t, bc.PoolTxWithData(tx, 5, mp, bc, verificationF))
		})
		t.Run("bad NVB: too big", func(t *testing.T) {
			maxNVB, err := bc.GetMaxNotValidBeforeDelta()
			require.NoError(t, err)
			tx := getPartiallyFilledTx(bc.BlockHeight()+maxNVB+1, bc.BlockHeight()+1)
			require.ErrorIs(t, bc.PoolTxWithData(tx, 5, mp, bc, verificationF), core.ErrInvalidAttribute)
		})
		t.Run("bad ValidUntilBlock: too small", func(t *testing.T) {
			maxNVB, err := bc.GetMaxNotValidBeforeDelta()
			require.NoError(t, err)
			tx := getPartiallyFilledTx(bc.BlockHeight(), bc.BlockHeight()+maxNVB+1)
			require.ErrorIs(t, bc.PoolTxWithData(tx, 5, mp, bc, verificationF), core.ErrInvalidAttribute)
		})
		t.Run("good", func(t *testing.T) {
			tx := getPartiallyFilledTx(bc.BlockHeight(), bc.BlockHeight()+1)
			require.NoError(t, bc.PoolTxWithData(tx, 5, mp, bc, verificationF))
		})
	})
}

func TestBlockchain_Bug1728(t *testing.T) {
	bc, acc := chain.NewSingle(t)
	e := neotest.NewExecutor(t, bc, acc, acc)
	managementInvoker := e.ValidatorInvoker(e.NativeHash(t, nativenames.Management))

	src := `package example
	import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	func init() { if true { } else { } }
	func _deploy(_ any, isUpdate bool) {
		runtime.Log("Deploy")
	}`
	c := neotest.CompileSource(t, acc.ScriptHash(), strings.NewReader(src), &compiler.Options{Name: "TestContract"})
	managementInvoker.DeployContract(t, c, nil)
}

func TestBlockchain_ResetStateErrors(t *testing.T) {
	chainHeight := 3
	checkResetErr := func(t *testing.T, cfg func(c *config.Blockchain), h uint32, errText string) {
		db, path := newLevelDBForTestingWithPath(t, t.TempDir())
		bc, validators, committee := chain.NewMultiWithCustomConfigAndStore(t, cfg, db, false)
		e := neotest.NewExecutor(t, bc, validators, committee)
		go bc.Run()
		for i := 0; i < chainHeight; i++ {
			e.AddNewBlock(t) // get some height
		}
		bc.Close()

		db, _ = newLevelDBForTestingWithPath(t, path)
		defer db.Close()
		bc, _, _ = chain.NewMultiWithCustomConfigAndStore(t, cfg, db, false)
		err := bc.Reset(h)
		if errText != "" {
			require.Error(t, err)
			require.True(t, strings.Contains(err.Error(), errText), err)
		} else {
			require.NoError(t, err)
		}
	}
	t.Run("large height", func(t *testing.T) {
		checkResetErr(t, nil, uint32(chainHeight+1), "can't reset state to height 4")
	})
	t.Run("already at height", func(t *testing.T) {
		checkResetErr(t, nil, uint32(chainHeight), "")
	})
	t.Run("KeepOnlyLatestState is enabled", func(t *testing.T) {
		checkResetErr(t, func(c *config.Blockchain) {
			c.Ledger.KeepOnlyLatestState = true
		}, uint32(chainHeight-1), "KeepOnlyLatestState is enabled")
	})
	t.Run("some blocks where removed", func(t *testing.T) {
		checkResetErr(t, func(c *config.Blockchain) {
			c.Ledger.RemoveUntraceableBlocks = true
			c.MaxTraceableBlocks = 2
		}, uint32(chainHeight-3), "RemoveUntraceableBlocks is enabled, a necessary batch of traceable blocks has already been removed")
	})
}

// TestBlockchain_ResetState is based on knowledge about basic chain transactions,
// it performs basic chain reset and checks that reset chain has proper state.
func TestBlockchain_ResetState(t *testing.T) {
	// Create the DB.
	db, path := newLevelDBForTestingWithPath(t, t.TempDir())
	bc, validators, committee := chain.NewMultiWithCustomConfigAndStore(t, func(cfg *config.Blockchain) {
		cfg.P2PSigExtensions = true
	}, db, false)
	go bc.Run()
	e := neotest.NewExecutor(t, bc, validators, committee)
	basicchain.Init(t, "../../", e)

	// Gather some reference information.
	resetBlockIndex := uint32(15)
	staleID := basicchain.NFSOContractID // NEP11
	rublesH := e.ContractHash(t, basicchain.RublesContractID)
	nnsH := e.ContractHash(t, basicchain.NNSContractID)
	staleH := e.ContractHash(t, staleID)
	gasH := e.NativeHash(t, nativenames.Gas)
	neoH := e.NativeHash(t, nativenames.Neo)
	gasID := e.NativeID(t, nativenames.Gas)
	neoID := e.NativeID(t, nativenames.Neo)
	resetBlockHash := bc.GetHeaderHash(resetBlockIndex)
	resetBlockHeader, err := bc.GetHeader(resetBlockHash)
	require.NoError(t, err)
	topBlockHeight := bc.BlockHeight()
	topBH := bc.GetHeaderHash(bc.BlockHeight())
	staleBH := bc.GetHeaderHash(resetBlockIndex + 1)
	staleB, err := bc.GetBlock(staleBH)
	require.NoError(t, err)
	staleTx := staleB.Transactions[0]
	_, err = bc.GetAppExecResults(staleTx.Hash(), trigger.Application)
	require.NoError(t, err)
	sr, err := bc.GetStateModule().GetStateRoot(resetBlockIndex)
	require.NoError(t, err)
	staleSR, err := bc.GetStateModule().GetStateRoot(resetBlockIndex + 1)
	require.NoError(t, err)
	rublesKey := []byte("testkey")
	rublesStaleKey := []byte("aa")
	rublesStaleValue := bc.GetStorageItem(basicchain.RublesContractID, rublesKey) // check value is there
	require.Equal(t, []byte(basicchain.RublesNewTestvalue), []byte(rublesStaleValue))
	acc0 := e.Validator.(neotest.MultiSigner).Single(2) // priv0 index->order and order->index conversion
	priv0ScriptHash := acc0.ScriptHash()
	var (
		expectedNEP11t []*state.NEP11Transfer
		expectedNEP17t []*state.NEP17Transfer
	)
	require.NoError(t, bc.ForEachNEP11Transfer(priv0ScriptHash, resetBlockHeader.Timestamp, func(t *state.NEP11Transfer) (bool, error) {
		if t.Block <= resetBlockIndex {
			expectedNEP11t = append(expectedNEP11t, t)
		}
		return true, nil
	}))
	require.NoError(t, bc.ForEachNEP17Transfer(priv0ScriptHash, resetBlockHeader.Timestamp, func(t *state.NEP17Transfer) (bool, error) {
		if t.Block <= resetBlockIndex {
			expectedNEP17t = append(expectedNEP17t, t)
		}
		return true, nil
	}))

	// checkProof checks that some stale proof is reachable
	checkProof := func() {
		rublesStaleFullKey := make([]byte, 4)
		binary.LittleEndian.PutUint32(rublesStaleFullKey, uint32(basicchain.RublesContractID))
		rublesStaleFullKey = append(rublesStaleFullKey, rublesStaleKey...)
		proof, err := bc.GetStateModule().GetStateProof(staleSR.Root, rublesStaleFullKey)
		require.NoError(t, err)
		require.NotEmpty(t, proof)
	}
	checkProof()

	// Ensure all changes were persisted.
	bc.Close()

	// Start new chain with existing DB, but do not run it.
	db, _ = newLevelDBForTestingWithPath(t, path)
	bc, _, _ = chain.NewMultiWithCustomConfigAndStore(t, func(cfg *config.Blockchain) {
		cfg.P2PSigExtensions = true
	}, db, false)
	defer db.Close()
	require.Equal(t, topBlockHeight, bc.BlockHeight()) // ensure DB was properly initialized.

	// Reset state.
	require.NoError(t, bc.Reset(resetBlockIndex))

	// Check that state was properly reset.
	require.Equal(t, resetBlockIndex, bc.BlockHeight())
	require.Equal(t, resetBlockIndex, bc.HeaderHeight())
	require.Equal(t, resetBlockHash, bc.CurrentHeaderHash())
	require.Equal(t, resetBlockHash, bc.CurrentBlockHash())
	require.Equal(t, resetBlockIndex, bc.GetStateModule().CurrentLocalHeight())
	require.Equal(t, sr.Root, bc.GetStateModule().CurrentLocalStateRoot())
	require.Equal(t, uint32(0), bc.GetStateModule().CurrentValidatedHeight())

	// Try to get the latest block\header.
	bh := bc.GetHeaderHash(resetBlockIndex)
	require.Equal(t, resetBlockHash, bh)
	h, err := bc.GetHeader(bh)
	require.NoError(t, err)
	require.Equal(t, resetBlockHeader, h)
	actualRublesHash, err := bc.GetContractScriptHash(basicchain.RublesContractID)
	require.NoError(t, err)
	require.Equal(t, rublesH, actualRublesHash)

	// Check that stale blocks/headers/txs/aers/sr are not reachable.
	for i := resetBlockIndex + 1; i <= topBlockHeight; i++ {
		hHash := bc.GetHeaderHash(i)
		require.Equal(t, util.Uint256{}, hHash)
		_, err = bc.GetStateRoot(i)
		require.Error(t, err)
	}
	for _, h := range []util.Uint256{staleBH, topBH} {
		_, err = bc.GetHeader(h)
		require.Error(t, err)
		_, err = bc.GetHeader(h)
		require.Error(t, err)
	}
	_, _, err = bc.GetTransaction(staleTx.Hash())
	require.Error(t, err)
	_, err = bc.GetAppExecResults(staleTx.Hash(), trigger.Application)
	require.Error(t, err)

	// However, proofs and everything related to stale MPT nodes still should work properly,
	// because we don't remove stale MPT nodes.
	checkProof()

	// Check NEP-compatible contracts.
	nep11 := bc.GetNEP11Contracts()
	require.Equal(t, 1, len(nep11)) // NNS
	require.Equal(t, nnsH, nep11[0])
	nep17 := bc.GetNEP17Contracts()
	require.Equal(t, 3, len(nep17)) // Neo, Gas, Rubles
	require.ElementsMatch(t, []util.Uint160{gasH, neoH, rublesH}, nep17)

	// Retrieve stale contract.
	cs := bc.GetContractState(staleH)
	require.Nil(t, cs)

	// Retrieve stale storage item.
	rublesValue := bc.GetStorageItem(basicchain.RublesContractID, rublesKey)
	require.Equal(t, []byte(basicchain.RublesOldTestvalue), []byte(rublesValue))   // the one with historic state
	require.Nil(t, bc.GetStorageItem(basicchain.RublesContractID, rublesStaleKey)) // the one that was added after target reset block
	db.Seek(storage.SeekRange{
		Prefix: []byte{byte(storage.STStorage)}, // no items with old prefix
	}, func(k, v []byte) bool {
		t.Fatal("no stale items must be left in storage")
		return false
	})

	// Check transfers.
	var (
		actualNEP11t []*state.NEP11Transfer
		actualNEP17t []*state.NEP17Transfer
	)
	require.NoError(t, bc.ForEachNEP11Transfer(priv0ScriptHash, e.TopBlock(t).Timestamp, func(t *state.NEP11Transfer) (bool, error) {
		actualNEP11t = append(actualNEP11t, t)
		return true, nil
	}))
	require.NoError(t, bc.ForEachNEP17Transfer(priv0ScriptHash, e.TopBlock(t).Timestamp, func(t *state.NEP17Transfer) (bool, error) {
		actualNEP17t = append(actualNEP17t, t)
		return true, nil
	}))
	assert.Equal(t, expectedNEP11t, actualNEP11t)
	assert.Equal(t, expectedNEP17t, actualNEP17t)
	lub, err := bc.GetTokenLastUpdated(priv0ScriptHash)
	require.NoError(t, err)
	expectedLUB := map[int32]uint32{ // this information is extracted from basic chain initialization code
		basicchain.NNSContractID:    resetBlockIndex - 1, // `neo.com` registration
		basicchain.RublesContractID: 6,                   // transfer of 123 RUR to priv1
		gasID:                       resetBlockIndex,     // fee for `1.2.3.4` A record registration
		neoID:                       4,                   // transfer of 1000 NEO to priv1
	}
	require.Equal(t, expectedLUB, lub)
}

func TestBlockchain_GenesisTransactionExtension(t *testing.T) {
	priv0 := testchain.PrivateKeyByID(0)
	acc0 := wallet.NewAccountFromPrivateKey(priv0)
	require.NoError(t, acc0.ConvertMultisig(1, []*keys.PublicKey{priv0.PublicKey()}))
	from := acc0.ScriptHash()
	to := util.Uint160{1, 2, 3}
	amount := 1

	script := io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, from.BytesBE())
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeCheckWitness)
	emit.Bytes(script.BinWriter, to.BytesBE())
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeCheckWitness)
	emit.AppCall(script.BinWriter, nativehashes.NeoToken, "transfer", callflag.All, from, to, amount, nil)
	emit.Opcodes(script.BinWriter, opcode.ASSERT)

	var sysFee int64 = 1_0000_0000
	bc, acc := chain.NewSingleWithCustomConfig(t, func(blockchain *config.Blockchain) {
		blockchain.Genesis.Transaction = &config.GenesisTransaction{
			Script:    script.Bytes(),
			SystemFee: sysFee,
		}
	})
	e := neotest.NewExecutor(t, bc, acc, acc)
	b := e.GetBlockByIndex(t, 0)
	tx := b.Transactions[0]
	e.CheckHalt(t, tx.Hash(), stackitem.NewBool(true), stackitem.NewBool(false))
	e.CheckGASBalance(t, e.Validator.ScriptHash(), big.NewInt(core.DefaultInitialGAS-sysFee))
	actualNeo, lub := e.Chain.GetGoverningTokenBalance(to)
	require.Equal(t, int64(amount), actualNeo.Int64())
	require.Equal(t, 0, int(lub))
}

// TestNativenames ensures that nativenames.All contains all expected native contract names
// in the right order.
func TestNativenames(t *testing.T) {
	bc, _ := chain.NewSingleWithCustomConfig(t, func(cfg *config.Blockchain) {
		cfg.Hardforks = map[string]uint32{}
		cfg.P2PSigExtensions = true
	})
	natives := bc.GetNatives()
	require.Equal(t, len(natives), len(nativenames.All))
	for i, cs := range natives {
		require.Equal(t, cs.Manifest.Name, nativenames.All[i], i)
	}
}
