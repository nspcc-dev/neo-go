package statesync_test

import (
	"bytes"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/basicchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestStateSyncModule_Init(t *testing.T) {
	const (
		stateSyncInterval = 2
		maxTraceable      = 3
	)
	spoutCfg := func(c *config.Blockchain) {
		c.StateRootInHeader = true
		c.StateSyncInterval = stateSyncInterval
		c.MaxTraceableBlocks = maxTraceable
	}
	bcSpout, validators, committee := chain.NewMultiWithCustomConfig(t, spoutCfg)
	e := neotest.NewExecutor(t, bcSpout, validators, committee)
	for range 2*stateSyncInterval + int(maxTraceable) + 2 {
		e.AddNewBlock(t)
	}

	boltCfg := func(c *config.Blockchain) {
		spoutCfg(c)
		c.P2PStateExchangeExtensions = true
		c.KeepOnlyLatestState = true
		c.RemoveUntraceableBlocks = true
	}

	boltCfgStorage := func(c *config.Blockchain) {
		spoutCfg(c)
		c.KeepOnlyLatestState = true
		c.RemoveUntraceableBlocks = true
		c.NeoFSStateSyncExtensions = true
		c.NeoFSStateFetcher.Enabled = true
		c.NeoFSBlockFetcher.Enabled = true
	}

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
		require.False(t, module.NeedStorageData())
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
		require.False(t, module.NeedStorageData())
	})

	check := func(t *testing.T, boltCfg func(c *config.Blockchain), storageItemsSync bool) {
		bcBolt, validatorsBolt, committeeBolt := chain.NewMultiWithCustomConfig(t, boltCfg)
		eBolt := neotest.NewExecutor(t, bcBolt, validatorsBolt, committeeBolt)
		module := bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))

		// firstly, fetch all headers to create proper DB state (where headers are in sync)
		stateSyncPoint := (bcSpout.BlockHeight() / stateSyncInterval) * stateSyncInterval
		require.Equal(t, stateSyncPoint, module.GetStateSyncPoint())
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
		require.True(t, module.NeedStorageData())
		require.Equal(t, bcSpout.HeaderHeight(), module.HeaderHeight())

		// then create new statesync module with the same DB and check that state is proper
		// (headers are in sync)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedStorageData())

		sm := bcSpout.GetStateModule()
		sroot, err := sm.GetStateRoot(stateSyncPoint)
		require.NoError(t, err)
		var lastKey []byte
		// add a few MPT nodes or contract storage items to create DB state where some of the elements are missing
		if !storageItemsSync {
			unknownNodes := module.GetUnknownMPTNodesBatch(2)
			require.Equal(t, 1, len(unknownNodes))
			require.Equal(t, expectedHeader.PrevStateRoot, unknownNodes[0])

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
		} else {
			// check AddContractStorageItems parameters
			require.ErrorContains(t, module.InitContractStorageSync(state.MPTRoot{Index: stateSyncPoint - 3, Root: sroot.Root}), "invalid sync height:")
			require.ErrorContains(t, module.AddContractStorageItems([]storage.KeyValue{}), "key-value pairs are empty")
			require.NoError(t, module.InitContractStorageSync(state.MPTRoot{Index: stateSyncPoint, Root: sroot.Root}))
			var batch []storage.KeyValue
			sm.SeekStates(sroot.Root, nil, func(k, v []byte) bool {
				batch = append(batch, storage.KeyValue{Key: k, Value: v})
				if len(batch) == 2 {
					require.NoError(t, module.AddContractStorageItems(batch))
					lastKey = batch[len(batch)-1].Key
					return false // stop seeking
				}
				return true
			})
			require.Equal(t, batch[len(batch)-1].Key, module.GetLastStoredKey())
		}

		// then create new statesync module with the same DB and check that state is proper
		// (headers are in sync, mpt is not yet synced)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedStorageData())
		require.False(t, module.NeedBlocks())
		if !storageItemsSync {
			unknownNodes := module.GetUnknownMPTNodesBatch(100)
			require.True(t, len(unknownNodes) > 0)
			require.NotContains(t, unknownNodes, expectedHeader.PrevStateRoot)
			require.Panicsf(t, func() { module.BlockHeight() }, "block height is not yet initialized since MPT is not in sync")

			// add the rest of MPT nodes and check that MPT is in sync
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
					require.NoError(t, module.AddMPTNodes([][]byte{bytes.Clone(nodeBytes)}))
					callbackCalled = true
					return true // add nodes one-by-one
				})
				require.NoError(t, err)
				require.True(t, callbackCalled)
			}
			unknownNodes = module.GetUnknownMPTNodesBatch(2)
			require.Equal(t, 0, len(unknownNodes))
		} else {
			require.NoError(t, module.InitContractStorageSync(state.MPTRoot{Index: stateSyncPoint, Root: sroot.Root}))
			require.Equal(t, lastKey, module.GetLastStoredKey())
			var skip bool
			sm.SeekStates(sroot.Root, nil, func(k, v []byte) bool {
				if skip {
					if bytes.Equal(k, lastKey) {
						skip = false
					}
					return true // skip this key
				}
				require.NoError(t, module.AddContractStorageItems([]storage.KeyValue{{Key: k, Value: v}}))
				return true
			})
			require.ErrorContains(t, module.AddContractStorageItems([]storage.KeyValue{{Key: []byte{1}, Value: []byte{1}}}), "contract storage items were not requested")
		}
		// check that module is active and storage data is in sync
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		require.True(t, module.NeedBlocks())

		// add several blocks to create DB state where blocks are not in sync yet, but it's not a genesis.
		for i := stateSyncPoint - maxTraceable + 1; i <= stateSyncPoint-stateSyncInterval-1; i++ {
			block, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, module.AddBlock(block))
		}
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		require.True(t, module.NeedBlocks())
		require.Equal(t, uint32(stateSyncPoint-stateSyncInterval-1), module.BlockHeight())

		// then create new statesync module with the same DB and check that state is proper
		// (blocks are not in sync yet, headers and MPT is in sync)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		require.True(t, module.NeedBlocks())
		if !storageItemsSync {
			unknownNodes := module.GetUnknownMPTNodesBatch(2)
			require.Equal(t, 0, len(unknownNodes))
		} else {
			require.ErrorContains(t, module.AddContractStorageItems([]storage.KeyValue{{Key: []byte{1}, Value: []byte{1}}}), "contract storage items were not requested")
		}
		require.Equal(t, uint32(stateSyncPoint-stateSyncInterval-1), module.BlockHeight())

		// add rest of blocks to create DB state where blocks are in sync and check state jump is performed
		for i := stateSyncPoint - stateSyncInterval; i <= stateSyncPoint; i++ {
			block, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, module.AddBlock(block))
		}
		require.False(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		require.False(t, module.NeedBlocks())
		lastBlock, err := bcBolt.GetBlock(expectedHeader.PrevHash)
		require.NoError(t, err)
		require.Equal(t, uint32(stateSyncPoint), lastBlock.Index)
		require.Equal(t, uint32(stateSyncPoint), module.BlockHeight())

		// then create new statesync module with the same DB and check that state is proper
		// (headers, blocks and MPT are in sync)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.False(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		require.False(t, module.NeedBlocks())

		// check that module is inactive and statejump is completed
		require.False(t, module.IsActive())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		require.False(t, module.NeedBlocks())
		if !storageItemsSync {
			unknownNodes := module.GetUnknownMPTNodesBatch(1)
			require.True(t, len(unknownNodes) == 0)
		} else {
			require.ErrorContains(t, module.AddContractStorageItems([]storage.KeyValue{{Key: []byte{1}, Value: []byte{1}}}), "contract storage items were not requested")
		}
		require.Equal(t, uint32(0), module.BlockHeight()) // inactive -> 0
		require.Equal(t, uint32(stateSyncPoint), bcBolt.BlockHeight())

		// create new module from completed state: the module should recognise that state sync is completed
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.False(t, module.IsActive())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		if !storageItemsSync {
			unknownNodes := module.GetUnknownMPTNodesBatch(1)
			require.True(t, len(unknownNodes) == 0)
		} else {
			require.Error(t, module.AddContractStorageItems([]storage.KeyValue{{Key: []byte{1}, Value: []byte{1}}}), "contract storage items were not requested")
		}
		require.Equal(t, uint32(0), module.BlockHeight()) // inactive -> 0
		require.Equal(t, uint32(stateSyncPoint), bcBolt.BlockHeight())

		// add one more block to the restored chain and start new module: the module should recognise state sync is completed
		// and regular blocks processing was started
		eBolt.AddNewBlock(t)
		module = bcBolt.GetStateSyncModule()
		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.False(t, module.IsActive())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		if !storageItemsSync {
			unknownNodes := module.GetUnknownMPTNodesBatch(1)
			require.True(t, len(unknownNodes) == 0)
		} else {
			require.ErrorContains(t, module.AddContractStorageItems([]storage.KeyValue{{Key: []byte{1}, Value: []byte{1}}}), "contract storage items were not requested")
		}
		require.Equal(t, uint32(0), module.BlockHeight()) // inactive -> 0
		require.Equal(t, uint32(stateSyncPoint)+1, bcBolt.BlockHeight())
	}

	t.Run("initialization from headers/blocks/mpt synced stages", func(t *testing.T) {
		check(t, boltCfg, false)
	})

	t.Run("initialization from headers/blocks/storage synced stages", func(t *testing.T) {
		check(t, boltCfgStorage, true)
	})
}

func TestStateSyncModule_RestoreBasicChain(t *testing.T) {
	check := func(t *testing.T, spoutEnableGC bool, enableStorageSync bool) {
		const (
			stateSyncInterval = 4
			maxTraceable      = 6
			stateSyncPoint    = 24
			trustedHeader     = stateSyncPoint - 2*maxTraceable + 2
		)
		spoutCfg := func(c *config.Blockchain) {
			c.KeepOnlyLatestState = spoutEnableGC
			c.RemoveUntraceableBlocks = spoutEnableGC
			c.StateRootInHeader = true
			c.StateSyncInterval = stateSyncInterval
			c.MaxTraceableBlocks = maxTraceable
			c.P2PStateExchangeExtensions = true // a tiny hack to avoid removal of untraceable headers from spout chain.
			c.Hardforks = map[string]uint32{
				config.HFAspidochelone.String(): 0,
				config.HFBasilisk.String():      0,
				config.HFCockatrice.String():    0,
				config.HFDomovoi.String():       0,
				config.HFEchidna.String():       0,
			}
			if !enableStorageSync {
				c.P2PStateExchangeExtensions = true
			}
		}
		bcSpoutStore := storage.NewMemoryStore()
		bcSpout, validators, committee := chain.NewMultiWithCustomConfigAndStore(t, spoutCfg, bcSpoutStore, false)
		go bcSpout.Run() // Will close it manually at the end.
		e := neotest.NewExecutor(t, bcSpout, validators, committee)
		basicchain.Init(t, "../../../", e)

		// make spout chain higher than latest state sync point (add several blocks up to stateSyncPoint+2),
		// consider keeping in sync with trustedHeader.
		e.AddNewBlock(t)
		e.AddNewBlock(t) // This block is stateSyncPoint-th block.
		e.AddNewBlock(t)
		require.Equal(t, stateSyncPoint+2, int(bcSpout.BlockHeight()))

		boltCfg := func(c *config.Blockchain) {
			spoutCfg(c)
			c.P2PStateExchangeExtensions = true
			c.KeepOnlyLatestState = true
			c.RemoveUntraceableBlocks = true
			if enableStorageSync {
				c.P2PStateExchangeExtensions = false
				c.NeoFSStateSyncExtensions = true
				c.NeoFSStateFetcher.Enabled = true
				c.NeoFSBlockFetcher.Enabled = true
			}
			if spoutEnableGC {
				// Use trusted header because spout chain doesn't have full header hashes chain
				// (they are removed along with old blocks/headers).
				c.TrustedHeader = config.HashIndex{
					Hash:  bcSpout.GetHeaderHash(trustedHeader),
					Index: trustedHeader,
				}
			}
		}
		bcBoltStore := storage.NewMemoryStore()
		bcBolt, _, _ := chain.NewMultiWithCustomConfigAndStore(t, boltCfg, bcBoltStore, false)
		go bcBolt.Run() // Will close it manually at the end.
		module := bcBolt.GetStateSyncModule()

		t.Run("error: add headers before initialisation", func(t *testing.T) {
			h, err := bcSpout.GetHeader(bcSpout.GetHeaderHash(bcSpout.HeaderHeight() - maxTraceable + 1))
			require.NoError(t, err)
			require.ErrorContains(t, module.AddHeaders(h), "headers were not requested")
		})
		t.Run("no error: add blocks before initialisation", func(t *testing.T) {
			b, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(bcSpout.BlockHeight()))
			require.NoError(t, err)
			require.NoError(t, module.AddBlock(b))
		})
		if enableStorageSync {
			t.Run("panic: add MPT nodes in ContractStorageBased mode", func(t *testing.T) {
				require.Panics(t, func() {
					err := module.AddMPTNodes([][]byte{})
					if err != nil {
						return
					}
				})
			})
			t.Run("error: add ContractStorage items without initialisation", func(t *testing.T) {
				require.Error(t, module.AddContractStorageItems([]storage.KeyValue{}))
			})
		} else {
			t.Run("panic: add contract storage items in MPTBased mode", func(t *testing.T) {
				require.Panics(t, func() {
					err := module.AddContractStorageItems([]storage.KeyValue{})
					if err != nil {
						return
					}
				})
			})
			t.Run("error: add MPT nodes without initialisation", func(t *testing.T) {
				require.ErrorContains(t, module.AddMPTNodes([][]byte{}), "MPT nodes were not requested")
			})
		}

		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.True(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		require.Panics(t, func() { module.BlockHeight() })

		// add headers to module starting from trusted height
		headers := make([]*block.Header, 0, bcSpout.HeaderHeight())
		start := 1
		if spoutEnableGC {
			start = trustedHeader
		}
		for i := uint32(start); i <= bcSpout.HeaderHeight(); i++ {
			h, err := bcSpout.GetHeader(bcSpout.GetHeaderHash(i))
			require.NoError(t, err, i)
			headers = append(headers, h)
		}
		require.NoError(t, module.AddHeaders(headers...))
		require.True(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.True(t, module.NeedStorageData())
		require.False(t, module.NeedBlocks())
		require.Equal(t, bcSpout.HeaderHeight(), bcBolt.HeaderHeight())

		// add MPT nodes or storage items in batches
		if enableStorageSync {
			sm := bcSpout.GetStateModule()
			sroot, err := bcSpout.GetStateModule().GetStateRoot(uint32(stateSyncPoint))
			require.NoError(t, err)

			require.NoError(t, module.InitContractStorageSync(state.MPTRoot{
				Index: uint32(stateSyncPoint),
				Root:  sroot.Root,
			}))
			var batch []storage.KeyValue
			sm.SeekStates(sroot.Root, nil, func(k, v []byte) bool {
				batch = append(batch, storage.KeyValue{Key: k, Value: v})
				if len(batch) >= 3 {
					err = module.AddContractStorageItems(batch)
					require.NoError(t, err)
					batch = batch[:0]
				}
				return true
			})
			if len(batch) > 0 {
				err = module.AddContractStorageItems(batch)
				require.NoError(t, err)
			}
			require.NoError(t, err)
		} else {
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
			unknownNodes := module.GetUnknownMPTNodesBatch(1)
			require.True(t, len(unknownNodes) == 0)
		}
		require.True(t, module.IsActive())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		require.Equal(t, uint32(stateSyncPoint-maxTraceable), module.BlockHeight())

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
		require.False(t, module.IsActive())
		require.True(t, module.IsInitialized())
		require.False(t, module.NeedHeaders())
		require.False(t, module.NeedStorageData())
		require.False(t, module.NeedBlocks())
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
				key := bytes.Clone(k)
				value := bytes.Clone(v)
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
		check(t, false, false)
	})
	t.Run("source node is light with GC", func(t *testing.T) {
		check(t, true, false)
	})
	t.Run("ContractStorageBased mode", func(t *testing.T) {
		check(t, false, true)
	})
}

func TestStateSyncModule_SetOnStageChanged(t *testing.T) {
	const (
		stateSyncInterval = 2
		maxTraceable      = 3
	)
	spoutCfg := func(c *config.Blockchain) {
		c.StateRootInHeader = true
		c.StateSyncInterval = stateSyncInterval
		c.MaxTraceableBlocks = maxTraceable
	}
	bcSpout, vals, comm := chain.NewMultiWithCustomConfig(t, spoutCfg)
	e := neotest.NewExecutor(t, bcSpout, vals, comm)
	for range 2*stateSyncInterval + maxTraceable + 2 {
		e.AddNewBlock(t)
	}

	mptCfg := func(c *config.Blockchain) {
		spoutCfg(c)
		c.P2PStateExchangeExtensions = true
		c.KeepOnlyLatestState = true
		c.RemoveUntraceableBlocks = true
	}
	storageCfg := func(c *config.Blockchain) {
		spoutCfg(c)
		c.KeepOnlyLatestState = true
		c.RemoveUntraceableBlocks = true
		c.NeoFSStateSyncExtensions = true
		c.NeoFSStateFetcher.Enabled = true
		c.NeoFSBlockFetcher.Enabled = true
	}

	check := func(t *testing.T, cfg func(*config.Blockchain), storageEnabled bool) {
		bcBolt, _, _ := chain.NewMultiWithCustomConfig(t, cfg)
		module := bcBolt.GetStateSyncModule()

		var calls int
		module.SetOnStageChanged(func() { calls++ })

		require.NoError(t, module.Init(bcSpout.BlockHeight()))
		require.Equal(t, 1, calls)

		syncPoint := module.GetStateSyncPoint()
		// AddHeaders up to P → headersSynced → active
		for i := uint32(1); i <= bcSpout.HeaderHeight(); i++ {
			h, err := bcSpout.GetHeader(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, module.AddHeaders(h))
		}
		require.Equal(t, 2, calls)
		require.True(t, module.IsActive())
		// AddMPTNodes or AddContractStorageItems up to P → storageSynced
		if !storageEnabled {
			for {
				unknown := module.GetUnknownMPTNodesBatch(3)
				if len(unknown) == 0 {
					break
				}
				require.NoError(t, bcSpout.GetStateSyncModule().Traverse(
					unknown[0],
					func(_ mpt.Node, nodeBytes []byte) bool {
						require.NoError(t, module.AddMPTNodes([][]byte{nodeBytes}))
						return true
					},
				))
			}
		} else {
			sm := bcSpout.GetStateModule()
			sroot, err := sm.GetStateRoot(syncPoint)
			require.NoError(t, err)
			require.NoError(t, module.InitContractStorageSync(state.MPTRoot{
				Index: syncPoint,
				Root:  sroot.Root,
			}))
			var all []storage.KeyValue
			sm.SeekStates(sroot.Root, nil, func(k, v []byte) bool {
				all = append(all, storage.KeyValue{Key: k, Value: v})
				return true
			})
			require.NoError(t, module.AddContractStorageItems(all))
		}
		require.Equal(t, 3, calls)

		// AddBlock up to P → blocksSynced → inactive
		for i := syncPoint - maxTraceable + 1; i <= syncPoint; i++ {
			b, err := bcSpout.GetBlock(bcSpout.GetHeaderHash(i))
			require.NoError(t, err)
			require.NoError(t, module.AddBlock(b))
		}
		require.Equal(t, 4, calls)
	}

	t.Run("MPT based ", func(t *testing.T) {
		check(t, mptCfg, false)
	})

	t.Run("ContractStorage based ", func(t *testing.T) {
		check(t, storageCfg, true)
	})
}
