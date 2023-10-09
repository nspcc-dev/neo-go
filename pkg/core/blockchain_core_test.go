package core

import (
	"encoding/binary"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestVerifyHeader(t *testing.T) {
	bc := newTestChain(t)
	prev := bc.topBlock.Load().(*block.Block).Header
	t.Run("Invalid", func(t *testing.T) {
		t.Run("Hash", func(t *testing.T) {
			h := prev.Hash()
			h[0] = ^h[0]
			hdr := newBlock(bc.config.ProtocolConfiguration, 1, h).Header
			require.ErrorIs(t, bc.verifyHeader(&hdr, &prev), ErrHdrHashMismatch)
		})
		t.Run("Index", func(t *testing.T) {
			hdr := newBlock(bc.config.ProtocolConfiguration, 3, prev.Hash()).Header
			require.ErrorIs(t, bc.verifyHeader(&hdr, &prev), ErrHdrIndexMismatch)
		})
		t.Run("Timestamp", func(t *testing.T) {
			hdr := newBlock(bc.config.ProtocolConfiguration, 1, prev.Hash()).Header
			hdr.Timestamp = 0
			require.ErrorIs(t, bc.verifyHeader(&hdr, &prev), ErrHdrInvalidTimestamp)
		})
	})
	t.Run("Valid", func(t *testing.T) {
		hdr := newBlock(bc.config.ProtocolConfiguration, 1, prev.Hash()).Header
		require.NoError(t, bc.verifyHeader(&hdr, &prev))
	})
}

func TestAddBlock(t *testing.T) {
	const size = 3
	bc := newTestChain(t)
	blocks, err := bc.genBlocks(size)
	require.NoError(t, err)

	lastBlock := blocks[len(blocks)-1]
	assert.Equal(t, lastBlock.Index, bc.HeaderHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())

	// This one tests persisting blocks, so it does need to persist()
	_, err = bc.persist(false)
	require.NoError(t, err)

	key := make([]byte, 1+util.Uint256Size)
	key[0] = byte(storage.DataExecutable)
	for _, block := range blocks {
		copy(key[1:], block.Hash().BytesBE())
		_, err := bc.dao.Store.Get(key)
		require.NoErrorf(t, err, "block %s not persisted", block.Hash())
	}

	assert.Equal(t, lastBlock.Index, bc.BlockHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())
}

func TestRemoveOldTransfers(t *testing.T) {
	// Creating proper number of transfers/blocks takes unnecessary time, so emulate
	// some DB with stale entries.
	bc := newTestChain(t)
	h, err := bc.GetHeader(bc.GetHeaderHash(0))
	require.NoError(t, err)
	older := h.Timestamp - 1000
	newer := h.Timestamp + 1000
	acc1 := util.Uint160{1}
	acc2 := util.Uint160{2}
	acc3 := util.Uint160{3}
	ttl := state.TokenTransferLog{Raw: []byte{1}} // It's incorrect, but who cares.

	for i := uint32(0); i < 3; i++ {
		bc.dao.PutTokenTransferLog(acc1, older, i, false, &ttl)
	}
	for i := uint32(0); i < 3; i++ {
		bc.dao.PutTokenTransferLog(acc2, newer, i, false, &ttl)
	}
	for i := uint32(0); i < 2; i++ {
		bc.dao.PutTokenTransferLog(acc3, older, i, true, &ttl)
	}
	for i := uint32(0); i < 2; i++ {
		bc.dao.PutTokenTransferLog(acc3, newer, i, true, &ttl)
	}

	_, err = bc.dao.Persist()
	require.NoError(t, err)
	_ = bc.removeOldTransfers(0)

	for i := uint32(0); i < 2; i++ {
		log, err := bc.dao.GetTokenTransferLog(acc1, older, i, false)
		require.NoError(t, err)
		require.Equal(t, 0, len(log.Raw))
	}

	log, err := bc.dao.GetTokenTransferLog(acc1, older, 2, false)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(log.Raw))

	for i := uint32(0); i < 3; i++ {
		log, err = bc.dao.GetTokenTransferLog(acc2, newer, i, false)
		require.NoError(t, err)
		require.NotEqual(t, 0, len(log.Raw))
	}

	log, err = bc.dao.GetTokenTransferLog(acc3, older, 0, true)
	require.NoError(t, err)
	require.Equal(t, 0, len(log.Raw))

	log, err = bc.dao.GetTokenTransferLog(acc3, older, 1, true)
	require.NoError(t, err)
	require.NotEqual(t, 0, len(log.Raw))

	for i := uint32(0); i < 2; i++ {
		log, err = bc.dao.GetTokenTransferLog(acc3, newer, i, true)
		require.NoError(t, err)
		require.NotEqual(t, 0, len(log.Raw))
	}
}

func TestBlockchain_InitWithIncompleteStateJump(t *testing.T) {
	var (
		stateSyncInterval        = 4
		maxTraceable      uint32 = 6
	)
	spountCfg := func(c *config.Config) {
		c.ApplicationConfiguration.RemoveUntraceableBlocks = true
		c.ProtocolConfiguration.StateRootInHeader = true
		c.ProtocolConfiguration.P2PStateExchangeExtensions = true
		c.ProtocolConfiguration.StateSyncInterval = stateSyncInterval
		c.ProtocolConfiguration.MaxTraceableBlocks = maxTraceable
		c.ApplicationConfiguration.KeepOnlyLatestState = true
	}
	bcSpout := newTestChainWithCustomCfg(t, spountCfg)

	// Generate some content.
	for i := 0; i < len(bcSpout.GetConfig().StandbyCommittee); i++ {
		require.NoError(t, bcSpout.AddBlock(bcSpout.newBlock()))
	}

	// reach next to the latest state sync point and pretend that we've just restored
	stateSyncPoint := (int(bcSpout.BlockHeight())/stateSyncInterval + 1) * stateSyncInterval
	for i := bcSpout.BlockHeight() + 1; i <= uint32(stateSyncPoint); i++ {
		require.NoError(t, bcSpout.AddBlock(bcSpout.newBlock()))
	}
	require.Equal(t, uint32(stateSyncPoint), bcSpout.BlockHeight())
	b := bcSpout.newBlock()
	require.NoError(t, bcSpout.AddHeaders(&b.Header))

	// put storage items with STTemp prefix
	batch := storage.NewMemCachedStore(bcSpout.dao.Store)
	tempPrefix := storage.STTempStorage
	if bcSpout.dao.Version.StoragePrefix == tempPrefix {
		tempPrefix = storage.STStorage
	}
	bPrefix := make([]byte, 1)
	bPrefix[0] = byte(bcSpout.dao.Version.StoragePrefix)
	bcSpout.dao.Store.Seek(storage.SeekRange{Prefix: bPrefix}, func(k, v []byte) bool {
		key := slice.Copy(k)
		key[0] = byte(tempPrefix)
		value := slice.Copy(v)
		batch.Put(key, value)
		return true
	})
	_, err := batch.Persist()
	require.NoError(t, err)

	checkNewBlockchainErr := func(t *testing.T, cfg func(c *config.Config), store storage.Store, errText string) {
		unitTestNetCfg, err := config.Load("../../config", testchain.Network())
		require.NoError(t, err)
		cfg(&unitTestNetCfg)
		log := zaptest.NewLogger(t)
		_, err = NewBlockchain(store, unitTestNetCfg.Blockchain(), log)
		if len(errText) != 0 {
			require.Error(t, err)
			require.True(t, strings.Contains(err.Error(), errText))
		} else {
			require.NoError(t, err)
		}
	}
	boltCfg := func(c *config.Config) {
		spountCfg(c)
		c.ApplicationConfiguration.KeepOnlyLatestState = true
	}
	// manually store statejump stage to check statejump recover process
	bPrefix[0] = byte(storage.SYSStateChangeStage)
	t.Run("invalid state jump stage format", func(t *testing.T) {
		bcSpout.dao.Store.Put(bPrefix, []byte{0x01, 0x02})
		checkNewBlockchainErr(t, boltCfg, bcSpout.dao.Store, "invalid state jump stage format")
	})
	t.Run("missing state sync point", func(t *testing.T) {
		bcSpout.dao.Store.Put(bPrefix, []byte{byte(stateJumpStarted)})
		checkNewBlockchainErr(t, boltCfg, bcSpout.dao.Store, "failed to get state sync point from the storage")
	})
	t.Run("invalid RemoveUntraceableBlocks setting", func(t *testing.T) {
		bcSpout.dao.Store.Put(bPrefix, []byte{byte(stateJumpStarted)})
		point := make([]byte, 4)
		binary.LittleEndian.PutUint32(point, uint32(stateSyncPoint))
		bcSpout.dao.Store.Put([]byte{byte(storage.SYSStateSyncPoint)}, point)
		checkNewBlockchainErr(t, func(c *config.Config) {
			boltCfg(c)
			c.ApplicationConfiguration.RemoveUntraceableBlocks = false
		}, bcSpout.dao.Store, "P2PStateExchangeExtensions can be enabled either on MPT-complete node")
	})
	t.Run("invalid state sync point", func(t *testing.T) {
		bcSpout.dao.Store.Put(bPrefix, []byte{byte(stateJumpStarted)})
		point := make([]byte, 4)
		binary.LittleEndian.PutUint32(point, bcSpout.lastHeaderIndex()+1)
		bcSpout.dao.Store.Put([]byte{byte(storage.SYSStateSyncPoint)}, point)
		checkNewBlockchainErr(t, boltCfg, bcSpout.dao.Store, "invalid state sync point")
	})
	for _, stage := range []stateChangeStage{stateJumpStarted, newStorageItemsAdded, staleBlocksRemoved, 0x03} {
		t.Run(fmt.Sprintf("state jump stage %d", stage), func(t *testing.T) {
			bcSpout.dao.Store.Put(bPrefix, []byte{byte(stage)})
			point := make([]byte, 4)
			binary.LittleEndian.PutUint32(point, uint32(stateSyncPoint))
			bcSpout.dao.Store.Put([]byte{byte(storage.SYSStateSyncPoint)}, point)
			var errText string
			if stage == 0x03 {
				errText = "unknown state jump stage"
			}
			checkNewBlockchainErr(t, spountCfg, bcSpout.dao.Store, errText)
		})
	}
}

func TestChainWithVolatileNumOfValidators(t *testing.T) {
	bc := newTestChainWithCustomCfg(t, func(c *config.Config) {
		c.ProtocolConfiguration.ValidatorsCount = 0
		c.ProtocolConfiguration.CommitteeHistory = map[uint32]uint32{
			0:  1,
			4:  4,
			24: 6,
		}
		c.ProtocolConfiguration.ValidatorsHistory = map[uint32]uint32{
			0: 1,
			4: 4,
		}
		require.NoError(t, c.ProtocolConfiguration.Validate())
	})
	require.Equal(t, uint32(0), bc.BlockHeight())

	priv0 := testchain.PrivateKeyByID(0)

	vals, err := bc.GetValidators()
	require.NoError(t, err)
	script, err := smartcontract.CreateDefaultMultiSigRedeemScript(vals)
	require.NoError(t, err)
	curWit := transaction.Witness{
		VerificationScript: script,
	}
	for i := 1; i < 26; i++ {
		comm, err := bc.GetCommittee()
		require.NoError(t, err)
		if i < 5 {
			require.Equal(t, 1, len(comm))
		} else if i < 25 {
			require.Equal(t, 4, len(comm))
		} else {
			require.Equal(t, 6, len(comm))
		}
		// Mimic consensus.
		if bc.config.ShouldUpdateCommitteeAt(uint32(i)) {
			vals, err = bc.GetValidators()
		} else {
			vals, err = bc.GetNextBlockValidators()
		}
		require.NoError(t, err)
		if i < 4 {
			require.Equalf(t, 1, len(vals), "at %d", i)
		} else {
			require.Equalf(t, 4, len(vals), "at %d", i)
		}
		require.NoError(t, err)
		script, err := smartcontract.CreateDefaultMultiSigRedeemScript(vals)
		require.NoError(t, err)
		nextWit := transaction.Witness{
			VerificationScript: script,
		}
		b := &block.Block{
			Header: block.Header{
				NextConsensus: nextWit.ScriptHash(),
				Script:        curWit,
			},
		}
		curWit = nextWit
		b.PrevHash = bc.GetHeaderHash(uint32(i) - 1)
		b.Timestamp = uint64(time.Now().UTC().Unix())*1000 + uint64(i)
		b.Index = uint32(i)
		b.RebuildMerkleRoot()
		if i < 5 {
			signa := priv0.SignHashable(uint32(bc.config.Magic), b)
			b.Script.InvocationScript = append([]byte{byte(opcode.PUSHDATA1), byte(len(signa))}, signa...)
		} else {
			b.Script.InvocationScript = testchain.Sign(b)
		}
		err = bc.AddBlock(b)
		require.NoErrorf(t, err, "at %d", i)
	}
}

func setSigner(tx *transaction.Transaction, h util.Uint160) {
	tx.Signers = []transaction.Signer{{
		Account: h,
		Scopes:  transaction.Global,
	}}
}

// This test checks that value of BaseExecFee returned from corresponding Blockchain's method matches
// the one provided to the constructor of new interop context.
func TestBlockchain_BaseExecFeeBaseStoragePrice_Compat(t *testing.T) {
	bc := newTestChain(t)

	check := func(t *testing.T) {
		ic := bc.newInteropContext(trigger.Application, bc.dao, bc.topBlock.Load().(*block.Block), nil)
		require.Equal(t, bc.GetBaseExecFee(), ic.BaseExecFee())
		require.Equal(t, bc.GetStoragePrice(), ic.BaseStorageFee())
	}
	t.Run("zero block", func(t *testing.T) {
		check(t)
	})
	t.Run("non-zero block", func(t *testing.T) {
		require.NoError(t, bc.AddBlock(bc.newBlock()))
		check(t)
	})
}

func TestBlockchain_IsRunning(t *testing.T) {
	chain := initTestChain(t, nil, nil)
	require.False(t, chain.isRunning.Load().(bool))
	oldPersisted := atomic.LoadUint32(&chain.persistedHeight)

	go chain.Run()
	require.NoError(t, chain.AddBlock(chain.newBlock()))
	require.Eventually(t, func() bool {
		persisted := atomic.LoadUint32(&chain.persistedHeight)
		return persisted > oldPersisted
	}, 2*persistInterval, 100*time.Millisecond)
	require.True(t, chain.isRunning.Load().(bool))

	chain.Close()
	require.False(t, chain.isRunning.Load().(bool))
}

func TestNewBlockchain_InitHardforks(t *testing.T) {
	t.Run("empty set", func(t *testing.T) {
		bc := newTestChainWithCustomCfg(t, func(c *config.Config) {
			c.ProtocolConfiguration.Hardforks = map[string]uint32{}
			require.NoError(t, c.ProtocolConfiguration.Validate())
		})
		require.Equal(t, map[string]uint32{
			config.HFAspidochelone.String(): 0,
			config.HFBasilisk.String():      0,
		}, bc.GetConfig().Hardforks)
	})
	t.Run("missing old", func(t *testing.T) {
		bc := newTestChainWithCustomCfg(t, func(c *config.Config) {
			c.ProtocolConfiguration.Hardforks = map[string]uint32{config.HFBasilisk.String(): 5}
			require.NoError(t, c.ProtocolConfiguration.Validate())
		})
		require.Equal(t, map[string]uint32{
			config.HFAspidochelone.String(): 0,
			config.HFBasilisk.String():      5,
		}, bc.GetConfig().Hardforks)
	})
	t.Run("missing new", func(t *testing.T) {
		bc := newTestChainWithCustomCfg(t, func(c *config.Config) {
			c.ProtocolConfiguration.Hardforks = map[string]uint32{config.HFAspidochelone.String(): 5}
			require.NoError(t, c.ProtocolConfiguration.Validate())
		})
		require.Equal(t, map[string]uint32{
			config.HFAspidochelone.String(): 5,
		}, bc.GetConfig().Hardforks)
	})
	t.Run("all present", func(t *testing.T) {
		bc := newTestChainWithCustomCfg(t, func(c *config.Config) {
			c.ProtocolConfiguration.Hardforks = map[string]uint32{config.HFAspidochelone.String(): 5, config.HFBasilisk.String(): 10}
			require.NoError(t, c.ProtocolConfiguration.Validate())
		})
		require.Equal(t, map[string]uint32{
			config.HFAspidochelone.String(): 5,
			config.HFBasilisk.String():      10,
		}, bc.GetConfig().Hardforks)
	})
}
