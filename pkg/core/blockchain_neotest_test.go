package core_test

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestBlockchain_DumpAndRestore(t *testing.T) {
	t.Run("no state root", func(t *testing.T) {
		testDumpAndRestore(t, func(c *config.ProtocolConfiguration) {
			c.StateRootInHeader = false
			c.P2PSigExtensions = true
		}, nil)
	})
	t.Run("with state root", func(t *testing.T) {
		testDumpAndRestore(t, func(c *config.ProtocolConfiguration) {
			c.StateRootInHeader = true
			c.P2PSigExtensions = true
		}, nil)
	})
	t.Run("remove untraceable", func(t *testing.T) {
		// Dump can only be created if all blocks and transactions are present.
		testDumpAndRestore(t, func(c *config.ProtocolConfiguration) {
			c.P2PSigExtensions = true
		}, func(c *config.ProtocolConfiguration) {
			c.MaxTraceableBlocks = 2
			c.RemoveUntraceableBlocks = true
			c.P2PSigExtensions = true
		})
	})
}

func testDumpAndRestore(t *testing.T, dumpF, restoreF func(c *config.ProtocolConfiguration)) {
	if restoreF == nil {
		restoreF = dumpF
	}

	bc, validators, committee := chain.NewMultiWithCustomConfig(t, dumpF)
	e := neotest.NewExecutor(t, bc, validators, committee)

	initBasicChain(t, e)
	require.True(t, bc.BlockHeight() > 5) // ensure that test is valid

	w := io.NewBufBinWriter()
	require.NoError(t, chaindump.Dump(bc, w.BinWriter, 0, bc.BlockHeight()+1))
	require.NoError(t, w.Err)

	buf := w.Bytes()
	t.Run("invalid start", func(t *testing.T) {
		bc2, _, _ := chain.NewMultiWithCustomConfig(t, restoreF)

		r := io.NewBinReaderFromBuf(buf)
		require.Error(t, chaindump.Restore(bc2, r, 2, 1, nil))
	})
	t.Run("good", func(t *testing.T) {
		bc2, _, _ := chain.NewMultiWithCustomConfig(t, dumpF)

		r := io.NewBinReaderFromBuf(buf)
		require.NoError(t, chaindump.Restore(bc2, r, 0, 2, nil))
		require.Equal(t, uint32(1), bc2.BlockHeight())

		r = io.NewBinReaderFromBuf(buf) // new reader because start is relative to dump
		require.NoError(t, chaindump.Restore(bc2, r, 2, 1, nil))
		t.Run("check handler", func(t *testing.T) {
			lastIndex := uint32(0)
			errStopped := errors.New("stopped")
			f := func(b *block.Block) error {
				lastIndex = b.Index
				if b.Index >= bc.BlockHeight()-1 {
					return errStopped
				}
				return nil
			}
			require.NoError(t, chaindump.Restore(bc2, r, 0, 1, f))
			require.Equal(t, bc2.BlockHeight(), lastIndex)

			r = io.NewBinReaderFromBuf(buf)
			err := chaindump.Restore(bc2, r, 4, bc.BlockHeight()-bc2.BlockHeight(), f)
			require.True(t, errors.Is(err, errStopped))
			require.Equal(t, bc.BlockHeight()-1, lastIndex)
		})
	})
}

func newLevelDBForTestingWithPath(t testing.TB, dbPath string) (storage.Store, string) {
	if dbPath == "" {
		dbPath = t.TempDir()
	}
	dbOptions := storage.LevelDBOptions{
		DataDirectoryPath: dbPath,
	}
	newLevelStore, err := storage.NewLevelDBStore(dbOptions)
	require.Nil(t, err, "NewLevelDBStore error")
	return newLevelStore, dbPath
}

func TestBlockchain_StartFromExistingDB(t *testing.T) {
	ps, path := newLevelDBForTestingWithPath(t, "")
	customConfig := func(c *config.ProtocolConfiguration) {
		c.StateRootInHeader = true // Need for P2PStateExchangeExtensions check.
		c.P2PSigExtensions = true  // Need for basic chain initializer.
	}
	bc, validators, committee, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, ps)
	require.NoError(t, err)
	go bc.Run()
	e := neotest.NewExecutor(t, bc, validators, committee)
	initBasicChain(t, e)
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
		d := dao.NewSimple(cache, bc.GetConfig().StateRootInHeader, bc.GetConfig().P2PStateExchangeExtensions)
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
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.ProtocolConfiguration) {
			customConfig(c)
			c.StateRootInHeader = false
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "StateRootInHeader setting mismatch"), err)
	})
	t.Run("mismatch P2PSigExtensions", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.ProtocolConfiguration) {
			customConfig(c)
			c.P2PSigExtensions = false
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "P2PSigExtensions setting mismatch"), err)
	})
	t.Run("mismatch P2PStateExchangeExtensions", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.ProtocolConfiguration) {
			customConfig(c)
			c.StateRootInHeader = true
			c.P2PStateExchangeExtensions = true
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "P2PStateExchangeExtensions setting mismatch"), err)
	})
	t.Run("mismatch KeepOnlyLatestState", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.ProtocolConfiguration) {
			customConfig(c)
			c.KeepOnlyLatestState = true
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "KeepOnlyLatestState setting mismatch"), err)
	})
	t.Run("corrupted headers", func(t *testing.T) {
		ps = newPS(t)

		// Corrupt headers hashes batch.
		cache := storage.NewMemCachedStore(ps) // Extra wrapper to avoid good DB corruption.
		key := make([]byte, 5)
		key[0] = byte(storage.IXHeaderHashList)
		binary.BigEndian.PutUint32(key[1:], 1)
		cache.Put(key, []byte{1, 2, 3})

		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, cache)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "failed to read batch of 2000"), err)
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
		binary.BigEndian.PutUint32(key, h)
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
		require.True(t, strings.Contains(err.Error(), "can't init cache for Management native contract"), err)
	})
	t.Run("invalid native contract deactivation", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, func(c *config.ProtocolConfiguration) {
			customConfig(c)
			c.NativeUpdateHistories = map[string][]uint32{
				nativenames.Policy:      {0},
				nativenames.Neo:         {0},
				nativenames.Gas:         {0},
				nativenames.Designation: {0},
				nativenames.StdLib:      {0},
				nativenames.Management:  {0},
				nativenames.Oracle:      {0},
				nativenames.Ledger:      {0},
				nativenames.Notary:      {0},
				nativenames.CryptoLib:   {h + 10},
			}
		}, ps)
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), fmt.Sprintf("native contract %s is already stored, but marked as inactive for height %d in config", nativenames.CryptoLib, h)), err)
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
		require.True(t, strings.Contains(err.Error(), fmt.Sprintf("native %s: version mismatch (stored contract state differs from autogenerated one)", nativenames.CryptoLib)), err)
	})

	t.Run("good", func(t *testing.T) {
		ps = newPS(t)
		_, _, _, err := chain.NewMultiWithCustomConfigAndStoreNoCheck(t, customConfig, ps)
		require.NoError(t, err)
	})
}
