package statesync_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/stretchr/testify/require"
)

func TestStateSyncModule_Init(t *testing.T) {
	const (
		stateSyncInterval = 2
		maxTraceable      = 3
	)
	spoutCfg := func(c *config.ProtocolConfiguration) {
		c.StateRootInHeader = true
		c.P2PStateExchangeExtensions = true
		c.StateSyncInterval = stateSyncInterval
		c.MaxTraceableBlocks = maxTraceable
	}
	bcSpout, validators, committee := chain.NewMultiWithCustomConfig(t, spoutCfg)
	e := neotest.NewExecutor(t, bcSpout, validators, committee)
	for i := 0; i <= 2*stateSyncInterval+int(maxTraceable)+1; i++ {
		e.AddNewBlock(t)
	}

	boltCfg := func(c *config.ProtocolConfiguration) {
		spoutCfg(c)
		c.KeepOnlyLatestState = true
		c.RemoveUntraceableBlocks = true
	}
	t.Run("error: module disabled by config", func(t *testing.T) {
		bcBolt, _, _ := chain.NewMultiWithCustomConfig(t, func(c *config.ProtocolConfiguration) {
			boltCfg(c)
			c.RemoveUntraceableBlocks = false
		})
		module := bcBolt.GetStateSyncModule()
		require.Error(t, module.Init(bcSpout.BlockHeight())) // module inactive (non-archival node)
	})

	t.Run("inactive: spout chain is too low to start state sync process", func(t *testing.T) {
		bcBolt, _, _ := chain.NewMultiWithCustomConfig(t, boltCfg)
		module := bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(uint32(2*stateSyncInterval-1)))
		require.False(t, module.IsActive())
	})

	t.Run("inactive: bolt chain height is close enough to spout chain height", func(t *testing.T) {
		bcBolt, _, _ := chain.NewMultiWithCustomConfig(t, boltCfg)
		for i := uint32(1); i < bcSpout.BlockHeight()-stateSyncInterval; i++ {
			b, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, bcBolt.AddBlock(b))
		}
		module := bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.False(t, module.IsActive())
	})

	t.Run("error: bolt chain is too low to start state sync process", func(t *testing.T) {
		bcBolt, validatorsBolt, committeeBolt := chain.NewMultiWithCustomConfig(t, boltCfg)
		eBolt := neotest.NewExecutor(t, bcBolt, validatorsBolt, committeeBolt)
		eBolt.AddNewBlock(t)

		module := bcBolt.GetStateSyncModule()
		require.Error(t, module.Init(uint32(3*stateSyncInterval)))
	})

	t.Run("initialized: no previous state sync point", func(t *testing.T) {
		bcBolt, _, _ := chain.NewMultiWithCustomConfig(t, boltCfg)

		module := bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.True(t, module.NeedHeaders())
		require.False(t, module.NeedMPTNodes())
	})

	t.Run("error: outdated state sync point in the storage", func(t *testing.T) {
		bcBolt, _, _ := chain.NewMultiWithCustomConfig(t, boltCfg)
		module := bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))

		module = bcBolt.GetStateSyncModule()
		require.Error(t, module.Init(bcSpout.BlockHeight()+2*uint32(stateSyncInterval)))
	})

	t.Run("initialized: valid previous state sync point in the storage", func(t *testing.T) {
		bcBolt, _, _ := chain.NewMultiWithCustomConfig(t, boltCfg)
		module := bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))

		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.True(t, module.NeedHeaders())
		require.False(t, module.NeedMPTNodes())
	})

	t.Run("initialization from headers/blocks/mpt synced stages", func(t *testing.T) {
		bcBolt, validatorsBolt, committeeBolt := chain.NewMultiWithCustomConfig(t, boltCfg)
		eBolt := neotest.NewExecutor(t, bcBolt, validatorsBolt, committeeBolt)
		module := bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))

		// firstly, fetch all headers to create proper DB state (where headers are in sync)
		stateSyncPoint := (bcSpout.BlockHeight() / stateSyncInterval) * stateSyncInterval
		var expectedHeader *block.Header
		for i := uint32(1); i <= bcSpout.HeaderHeight(); i++ {
			header, err := bcSpout.GetHeader(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, module.AddHeaders(header))
			if i == stateSyncPoint+1 {
				expectedHeader = header
			}
		}
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedMPTNodes())

		// then create new statesync module with the same DB and check that state is proper
		// (headers are in sync)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedMPTNodes())
		unknownNodes := module.GetUnknownMPTNodesBatch(2)
		require.Equal(t, 1, len(unknownNodes))
		require.Equal(t, expectedHeader.PrevStateRoot, unknownNodes[0])

		// add several blocks to create DB state where blocks are not in sync yet, but it's not a genesis.
		for i := stateSyncPoint - maxTraceable + 1; i <= stateSyncPoint-stateSyncInterval-1; i++ {
			block, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, module.AddBlock(block))
		}
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedMPTNodes())
		require.Equal(t, uint32(stateSyncPoint-stateSyncInterval-1), module.BlockHeight())

		// then create new statesync module with the same DB and check that state is proper
		// (blocks are not in sync yet)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedMPTNodes())
		unknownNodes = module.GetUnknownMPTNodesBatch(2)
		require.Equal(t, 1, len(unknownNodes))
		require.Equal(t, expectedHeader.PrevStateRoot, unknownNodes[0])
		require.Equal(t, uint32(stateSyncPoint-stateSyncInterval-1), module.BlockHeight())

		// add rest of blocks to create DB state where blocks are in sync
		for i := stateSyncPoint - stateSyncInterval; i <= stateSyncPoint; i++ {
			block, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, module.AddBlock(block))
		}
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedMPTNodes())
		lastBlock, err := bcBolt.GetBlock(expectedHeader.PrevHash)
		require.NoError(t, err)
		require.Equal(t, uint32(stateSyncPoint), lastBlock.Index)
		require.Equal(t, uint32(stateSyncPoint), module.BlockHeight())

		// then create new statesync module with the same DB and check that state is proper
		// (headers and blocks are in sync)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedMPTNodes())
		unknownNodes = module.GetUnknownMPTNodesBatch(2)
		require.Equal(t, 1, len(unknownNodes))
		require.Equal(t, expectedHeader.PrevStateRoot, unknownNodes[0])
		require.Equal(t, uint32(stateSyncPoint), module.BlockHeight())

		// add a few MPT nodes to create DB state where some of MPT nodes are missing
		count := 5
		for {
			unknownHashes := module.GetUnknownMPTNodesBatch(1) // restore nodes one-by-one
			if len(unknownHashes) == 0 {
				break
			}
			err := bcSpout.GetStateSyncModule().Traverse(unknownHashes[0], func(node mpt.Node, nodeBytes []byte) bool {
				require.NoError(t, module.AddMPTNodes([][]byte{nodeBytes}))
				return true // add nodes one-by-one
			})
			require.NoError(t, err)
			count--
			if count < 0 {
				break
			}
		}

		// then create new statesync module with the same DB and check that state is proper
		// (headers and blocks are in sync, mpt is not yet synced)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedMPTNodes())
		unknownNodes = module.GetUnknownMPTNodesBatch(100)
		require.True(t, len(unknownNodes) > 0)
		require.NotContains(t, unknownNodes, expectedHeader.PrevStateRoot)
		require.Equal(t, uint32(stateSyncPoint), module.BlockHeight())

		// add the rest of MPT nodes and jump to state
		alreadyRequested := make(map[util.Uint256]struct{})
		for {
			unknownHashes := module.GetUnknownMPTNodesBatch(1) // restore nodes one-by-one
			if len(unknownHashes) == 0 {
				break
			}
			if _, ok := alreadyRequested[unknownHashes[0]]; ok {
				t.Fatal("bug: node was requested twice")
			}
			alreadyRequested[unknownHashes[0]] = struct{}{}
			var callbackCalled bool
			err := bcSpout.GetStateSyncModule().Traverse(unknownHashes[0], func(node mpt.Node, nodeBytes []byte) bool {
				require.NoError(t, module.AddMPTNodes([][]byte{slice.Copy(nodeBytes)}))
				callbackCalled = true
				return true // add nodes one-by-one
			})
			require.NoError(t, err)
			require.True(t, callbackCalled)
		}

		// check that module is inactive and statejump is completed
		require.False(t, module.IsActive())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedMPTNodes())
		unknownNodes = module.GetUnknownMPTNodesBatch(1)
		require.True(t, len(unknownNodes) == 0)
		require.Equal(t, uint32(stateSyncPoint), module.BlockHeight())
		require.Equal(t, uint32(stateSyncPoint), bcBolt.BlockHeight())

		// create new module from completed state: the module should recognise that state sync is completed
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.False(t, module.IsActive())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedMPTNodes())
		unknownNodes = module.GetUnknownMPTNodesBatch(1)
		require.True(t, len(unknownNodes) == 0)
		require.Equal(t, uint32(stateSyncPoint), module.BlockHeight())
		require.Equal(t, uint32(stateSyncPoint), bcBolt.BlockHeight())

		// add one more block to the restored chain and start new module: the module should recognise state sync is completed
		// and regular blocks processing was started
		eBolt.AddNewBlock(t)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.False(t, module.IsActive())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedMPTNodes())
		unknownNodes = module.GetUnknownMPTNodesBatch(1)
		require.True(t, len(unknownNodes) == 0)
		require.Equal(t, uint32(stateSyncPoint)+1, module.BlockHeight())
		require.Equal(t, uint32(stateSyncPoint)+1, bcBolt.BlockHeight())
	})
}

func TestStateSyncModule_RestoreBasicChain(t *testing.T) {
	check := func(t *testing.T, spoutEnableGC bool) {
		const (
			stateSyncInterval = 4
			maxTraceable      = 6
			stateSyncPoint    = 24
		)
		spoutCfg := func(c *config.ProtocolConfiguration) {
			c.KeepOnlyLatestState = spoutEnableGC
			c.RemoveUntraceableBlocks = spoutEnableGC
			c.StateRootInHeader = true
			c.P2PStateExchangeExtensions = true
			c.StateSyncInterval = stateSyncInterval
			c.MaxTraceableBlocks = maxTraceable
			c.P2PSigExtensions = true // `basicchain.Init` assumes Notary is enabled.
		}
		bcSpoutStore := storage.NewMemoryStore()
		bcSpout, validators, committee := chain.NewMultiWithCustomConfigAndStore(t, spoutCfg, bcSpoutStore, false)
		go bcSpout.Run() // Will close it manually at the end.
		e := neotest.NewExecutor(t, bcSpout, validators, committee)
		basicchain.Init(t, "../../../", e)

		// make spout chain higher than latest state sync point (add several blocks up to stateSyncPoint+2)
		e.AddNewBlock(t)
		e.AddNewBlock(t) // This block is stateSyncPoint-th block.
		e.AddNewBlock(t)
		require.Equal(t, stateSyncPoint+2, int(bcSpout.BlockHeight()))

		boltCfg := func(c *config.ProtocolConfiguration) {
			spoutCfg(c)
			c.KeepOnlyLatestState = true
			c.RemoveUntraceableBlocks = true
		}
		bcBoltStore := storage.NewMemoryStore()
		bcBolt, _, _ := chain.NewMultiWithCustomConfigAndStore(t, boltCfg, bcBoltStore, false)
		go bcBolt.Run() // Will close it manually at the end.
		module := bcBolt.GetStateSyncModule()

		t.Run("error: add headers before initialisation", func(t *testing.T) {
			h, err := bcSpout.GetHeader(bcSpout.GetHeaderHash(1))
			require.NoError(t, err)
			require.Error(t, module.AddHeaders(h))
		})
		t.Run("no error: add blocks before initialisation", func(t *testing.T) {
			b, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(bcSpout.BlockHeight()))
			require.NoError(t, err)
			require.NoError(t, module.AddBlock(b))
		})
		t.Run("error: add MPT nodes without initialisation", func(t *testing.T) {
			require.Error(t, module.AddMPTNodes([][]byte{}))
		})

		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.True(t, module.NeedHeaders())
		require.False(t, module.NeedMPTNodes())

		// add headers to module
		headers := make([]*block.Header, 0, bcSpout.HeaderHeight())
		for i := uint32(1); i <= bcSpout.HeaderHeight(); i++ {
			h, err := bcSpout.GetHeader(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			headers = append(headers, h)
		}
		require.NoError(t, module.AddHeaders(headers...))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedMPTNodes())
		require.Equal(t, bcSpout.HeaderHeight(), bcBolt.HeaderHeight())

		// add blocks
		t.Run("error: unexpected block index", func(t *testing.T) {
			b, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(stateSyncPoint - maxTraceable))
			require.NoError(t, err)
			require.Error(t, module.AddBlock(b))
		})
		t.Run("error: missing state root in block header", func(t *testing.T) {
			b := &block.Block{
				Header: block.Header{
					Index:            uint32(stateSyncPoint) - maxTraceable + 1,
					StateRootEnabled: false,
				},
			}
			require.Error(t, module.AddBlock(b))
		})
		t.Run("error: invalid block merkle root", func(t *testing.T) {
			b := &block.Block{
				Header: block.Header{
					Index:            uint32(stateSyncPoint) - maxTraceable + 1,
					StateRootEnabled: true,
					MerkleRoot:       util.Uint256{1, 2, 3},
				},
			}
			require.Error(t, module.AddBlock(b))
		})

		for i := uint32(stateSyncPoint - maxTraceable + 1); i <= stateSyncPoint; i++ {
			b, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, module.AddBlock(b))
		}
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedMPTNodes())
		require.Equal(t, uint32(stateSyncPoint), module.BlockHeight())

		// add MPT nodes in batches
		h, err := bcSpout.GetHeader(bcSpout.GetHeaderHash(stateSyncPoint + 1))
		require.NoError(t, err)
		unknownHashes := module.GetUnknownMPTNodesBatch(100)
		require.Equal(t, 1, len(unknownHashes))
		require.Equal(t, h.PrevStateRoot, unknownHashes[0])
		nodesMap := make(map[util.Uint256][]byte)

		sm := bcSpout.GetStateModule()
		sroo, err := sm.GetStateRoot(uint32(stateSyncPoint))
		require.NoError(t, err)
		require.Equal(t, sroo.Root, h.PrevStateRoot)
		err = bcSpout.GetStateSyncModule().Traverse(h.PrevStateRoot, func(n mpt.Node, nodeBytes []byte) bool {
			nodesMap[n.Hash()] = nodeBytes
			return false
		})
		require.NoError(t, err)
		for {
			need := module.GetUnknownMPTNodesBatch(10)
			if len(need) == 0 {
				break
			}
			add := make([][]byte, len(need))
			for i, h := range need {
				nodeBytes, ok := nodesMap[h]
				if !ok {
					t.Fatal("unknown or restored node requested")
				}
				add[i] = nodeBytes
				delete(nodesMap, h)
			}
			require.NoError(t, module.AddMPTNodes(add))
		}
		require.False(t, module.IsActive())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedMPTNodes())
		unknownNodes := module.GetUnknownMPTNodesBatch(1)
		require.True(t, len(unknownNodes) == 0)
		require.Equal(t, uint32(stateSyncPoint), module.BlockHeight())
		require.Equal(t, uint32(stateSyncPoint), bcBolt.BlockHeight())

		// add missing blocks to bcBolt: should be ok, because state is synced
		for i := uint32(stateSyncPoint + 1); i <= bcSpout.BlockHeight(); i++ {
			b, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, bcBolt.AddBlock(b))
		}
		require.Equal(t, bcSpout.BlockHeight(), bcBolt.BlockHeight())

		// compare storage states
		fetchStorage := func(ps storage.Store, storagePrefix byte) []storage.KeyValue {
			var kv []storage.KeyValue
			ps.Seek(storage.SeekRange{Prefix: []byte{storagePrefix}}, func(k, v []byte) bool {
				key := slice.Copy(k)
				value := slice.Copy(v)
				if key[0] == byte(storage.STTempStorage) {
					key[0] = byte(storage.STStorage)
				}
				kv = append(kv, storage.KeyValue{
					Key:   key,
					Value: value,
				})
				return true
			})
			return kv
		}
		// Both blockchains are running, so we need to wait until recent changes will be persisted
		// to the underlying backend store. Close blockchains to ensure persist was completed.
		bcSpout.Close()
		bcBolt.Close()
		expected := fetchStorage(bcSpoutStore, byte(storage.STStorage))
		actual := fetchStorage(bcBoltStore, byte(storage.STTempStorage))
		require.ElementsMatch(t, expected, actual)

		// no temp items should be left
		var haveItems bool
		bcBoltStore.Seek(storage.SeekRange{Prefix: []byte{byte(storage.STStorage)}}, func(_, _ []byte) bool {
			haveItems = true
			return false
		})
		require.False(t, haveItems)
	}
	t.Run("source node is archive", func(t *testing.T) {
		check(t, false)
	})
	t.Run("source node is light with GC", func(t *testing.T) {
		check(t, true)
	})
}
