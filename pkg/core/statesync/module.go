/*
Package statesync implements module for the P2P state synchronisation process. The
module manages state synchronisation for non-archival nodes which are joining the
network and don't have the ability to resync from the genesis block.

Given the currently available state synchronisation point P, the state sync process
includes the following stages:

1. Fetching headers from height 0 to P+1.
2. Fetching state data for height P, starting from the corresponding state root.
This uses contract storage items from an external source (e.g., NeoFS) when
enabled, or MPT nodes via P2P. If storage-based sync fails to reach P with the
expected state root, MPT nodes are requested to complete synchronisation.
3. Fetching blocks from height P-MaxTraceableBlocks (or 0) to P.

Steps 2 and 3 are performed in parallel. Once all data are collected and stored
in the db, an atomic state jump occurs to the state sync point P. Further node
operations use the standard sync mechanism until the node reaches a synchronised
state.
*/
package statesync

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/zap"
)

// stateSyncStage is a type of state synchronisation stage.
type stateSyncStage uint8

const (
	// inactive means that state exchange is disabled by the protocol configuration.
	// Can't be combined with other states.
	inactive stateSyncStage = 1 << iota
	// none means that state exchange is enabled in the configuration, but
	// initialisation of the state sync module wasn't yet performed, i.e.
	// (*Module).Init wasn't called. Can't be combined with other states.
	none
	// initialized means that (*Module).Init was called, but other sync stages
	// are not yet reached (i.e. that headers are requested, but not yet fetched).
	// Can't be combined with other states.
	initialized
	// headersSynced means that headers for the current state sync point are fetched.
	// May be combined with mptSynced and/or blocksSynced.
	headersSynced
	// mptSynced means that MPT nodes for the current state sync point are fetched.
	// Always combined with headersSynced; may be combined with blocksSynced.
	mptSynced
	// blocksSynced means that blocks up to the current state sync point are stored.
	// Always combined with headersSynced; may be combined with mptSynced.
	blocksSynced
)

const (
	// maxPendingPairs is the maximum number of key-value pairs before flushing.
	maxPendingPairs = 1000
)

// Ledger is the interface required from Blockchain for Module to operate.
type Ledger interface {
	AddHeaders(...*block.Header) error
	BlockHeight() uint32
	GetConfig() config.Blockchain
	GetHeader(hash util.Uint256) (*block.Header, error)
	GetHeaderHash(uint32) util.Uint256
	HeaderHeight() uint32
}

// checkpointMetadata stores the state of an interrupted storage-based sync.
type checkpointMetadata struct {
	SyncHeight   uint32
	MPTRoot      util.Uint256 // Computed MPT root at checkpoint.
	ExpectedRoot util.Uint256 // Expected state root for validation.
	LastKey      []byte       // Last processed storage key.
}

// Module represents state sync module and aimed to gather state-related data to
// perform an atomic state jump.
type Module struct {
	lock sync.RWMutex
	log  *zap.Logger

	// syncPoint is the state synchronisation point P we're currently working against.
	syncPoint uint32
	// syncStage is the stage of the sync process.
	syncStage stateSyncStage
	// syncInterval is the delta between two adjacent state sync points.
	syncInterval uint32
	// blockHeight is the index of the latest stored block.
	blockHeight uint32
	// storageSync indicates whether the module is in storage-based sync mode.
	storageSync bool
	// needMPTAfterStorage indicates MPT nodes are needed after incomplete storage sync.
	needMPTAfterStorage bool
	// pendingPairs holds key-value pairs before flushing.
	pendingPairs map[string][]byte
	// currentSyncHeight tracks the sync height of pending pairs.
	currentSyncHeight uint32
	// currentExpectedRoot tracks the expected root of pending pairs.
	currentExpectedRoot util.Uint256
	// lastCheckpointHeight caches the latest checkpoint SyncHeight.
	lastCheckpointHeight uint32

	dao      *dao.Simple
	bc       Ledger
	stateMod *stateroot.Module
	mptpool  *Pool

	billet *mpt.Billet
	// localTrie is used for storage-based state synchronization.
	localTrie *mpt.Trie

	jumpCallback func(p uint32) error

	// stageChangedCallback is an optional callback that is triggered whenever
	// the sync stage changes.
	stageChangedCallback func()
}

// NewModule returns new instance of statesync module.
func NewModule(bc Ledger, stateMod *stateroot.Module, log *zap.Logger, s *dao.Simple, jumpCallback func(p uint32) error) *Module {
	if !(bc.GetConfig().P2PStateExchangeExtensions && bc.GetConfig().Ledger.RemoveUntraceableBlocks) && !bc.GetConfig().NeoFSStateSyncExtensions {
		return &Module{
			dao:       s,
			bc:        bc,
			stateMod:  stateMod,
			syncStage: inactive,
		}
	}
	return &Module{
		dao:                 s,
		bc:                  bc,
		stateMod:            stateMod,
		log:                 log,
		syncInterval:        uint32(bc.GetConfig().StateSyncInterval),
		mptpool:             NewPool(),
		syncStage:           none,
		jumpCallback:        jumpCallback,
		storageSync:         bc.GetConfig().NeoFSStateSyncExtensions,
		pendingPairs:        make(map[string][]byte),
		currentSyncHeight:   0,
		currentExpectedRoot: util.Uint256{},
	}
}

// Init initializes state sync module for the current chain's height with given
// callback for MPT nodes requests.
func (s *Module) Init(currChainHeight uint32) error {
	oldStage := s.syncStage
	s.lock.Lock()
	defer func() {
		if s.syncStage != oldStage {
			s.notifyStageChanged()
		}
	}()
	defer s.lock.Unlock()

	if s.syncStage != none {
		return errors.New("already initialized or inactive")
	}

	p := (currChainHeight / s.syncInterval) * s.syncInterval
	if p < 2*s.syncInterval {
		// chain is too low to start state exchange process, use the standard sync mechanism
		s.syncStage = inactive
		return nil
	}
	pOld, err := s.dao.GetStateSyncPoint()
	if err == nil && pOld >= p-s.syncInterval {
		// old point is still valid, so try to resync states for this point.
		p = pOld
	} else {
		if s.bc.BlockHeight() > p-2*s.syncInterval {
			// chain has already been synchronised up to old state sync point and regular blocks processing was started.
			// Current block height is enough to start regular blocks processing.
			s.syncStage = inactive
			return nil
		}
		if err == nil {
			// pOld was found, it is outdated, and chain wasn't completely synchronised for pOld. Need to drop the db.
			return fmt.Errorf("state sync point %d is found in the storage, "+
				"but sync process wasn't completed and point is outdated. Please, drop the database manually and restart the node to run state sync process", pOld)
		}
		if s.bc.BlockHeight() != 0 {
			// pOld wasn't found, but blocks processing was started in a regular manner and latest stored block is too outdated
			// to start regular blocks processing again. Need to drop the db.
			return fmt.Errorf("current chain's height is too low to start regular blocks processing from the oldest sync point %d. "+
				"Please, drop the database manually and restart the node to run state sync process", p-s.syncInterval)
		}

		// We've reached this point, so chain has genesis block only. As far as we can't ruin
		// current chain's state until new state is completely fetched, outdated state-related data
		// will be removed from storage during (*Blockchain).jumpToState(...) execution.
		// All we need to do right now is to remove genesis-related MPT nodes.
		err = s.stateMod.CleanStorage()
		if err != nil {
			return fmt.Errorf("failed to remove outdated MPT data from storage: %w", err)
		}
	}

	s.syncPoint = p
	s.dao.PutStateSyncPoint(p)
	s.syncStage = initialized
	s.log.Info("try to sync state for the latest state synchronisation point",
		zap.Uint32("point", p),
		zap.Uint32("evaluated chain's blockHeight", currChainHeight))

	// Initialize localTrie for storage-based sync.
	if s.storageSync {
		metadata, err := s.loadCheckpointMetadata()
		if err == nil {
			s.localTrie = mpt.NewTrie(mpt.NewHashNode(metadata.MPTRoot), mpt.ModeAll, s.dao.Store)
			if computedRoot := s.localTrie.StateRoot(); !computedRoot.Equals(metadata.MPTRoot) {
				s.log.Warn("checkpointed state is invalid, starting from scratch",
					zap.String("computed_root", computedRoot.String()),
					zap.String("expected_root", metadata.MPTRoot.String()))
				s.localTrie = mpt.NewTrie(mpt.EmptyNode{}, mpt.ModeAll, s.dao.Store)
			}
			s.lastCheckpointHeight = metadata.SyncHeight
		} else {
			s.localTrie = mpt.NewTrie(mpt.EmptyNode{}, mpt.ModeAll, s.dao.Store)
		}
	}

	return s.defineSyncStage()
}

// SetOnStageChanged sets callback that is triggered whenever the sync stage changes.
func (s *Module) SetOnStageChanged(cb func()) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.stageChangedCallback = cb
}

// notifyStageChanged triggers stage change callback if it's set.
func (s *Module) notifyStageChanged() {
	if s.stageChangedCallback != nil {
		s.stageChangedCallback()
	}
}

// TemporaryPrefix accepts current storage prefix and returns prefix
// to use for storing intermediate items during synchronization.
func TemporaryPrefix(currPrefix storage.KeyPrefix) storage.KeyPrefix {
	switch currPrefix {
	case storage.STStorage:
		return storage.STTempStorage
	case storage.STTempStorage:
		return storage.STStorage
	default:
		panic(fmt.Sprintf("invalid storage prefix: %x", currPrefix))
	}
}

// defineSyncStage sequentially checks and sets sync state process stage after Module
// initialization. It also performs initialization of MPT Billet if necessary.
func (s *Module) defineSyncStage() error {
	// check headers sync stage first
	ltstHeaderHeight := s.bc.HeaderHeight()
	if ltstHeaderHeight > s.syncPoint {
		s.syncStage = headersSynced
		s.log.Info("headers are in sync",
			zap.Uint32("headerHeight", s.bc.HeaderHeight()))
	}

	// check blocks sync stage
	s.blockHeight = s.getLatestSavedBlock(s.syncPoint)
	if s.blockHeight >= s.syncPoint {
		s.syncStage |= blocksSynced
		s.log.Info("blocks are in sync",
			zap.Uint32("blockHeight", s.blockHeight))
	}

	// check MPT sync stage
	if s.blockHeight > s.syncPoint {
		s.syncStage |= mptSynced
		s.log.Info("MPT is in sync",
			zap.Uint32("stateroot height", s.stateMod.CurrentLocalHeight()))
	} else if s.syncStage&headersSynced != 0 {
		if s.storageSync {
			// Check if storage-based sync is complete.
			metadata, err := s.loadCheckpointMetadata()
			if err == nil && metadata.SyncHeight == s.syncPoint {
				sr, err := s.stateMod.GetStateRoot(s.syncPoint)
				if err == nil && sr.Root.Equals(metadata.MPTRoot) && metadata.MPTRoot.Equals(metadata.ExpectedRoot) {
					s.syncStage |= mptSynced
					s.log.Info("storage-based state is in sync",
						zap.Uint32("height", s.syncPoint),
						zap.String("root", metadata.MPTRoot.String()))
				}
			}
			if err == nil {
				s.lastCheckpointHeight = metadata.SyncHeight
			}
		} else {
			// Initialize MPT-based sync.
			header, err := s.bc.GetHeader(s.bc.GetHeaderHash(s.syncPoint + 1))
			if err != nil {
				return fmt.Errorf("failed to get header to initialize MPT billet: %w", err)
			}
			var mode mpt.TrieMode
			if s.bc.GetConfig().Ledger.KeepOnlyLatestState || s.bc.GetConfig().Ledger.RemoveUntraceableBlocks {
				mode |= mpt.ModeLatest
			}
			s.billet = mpt.NewBillet(header.PrevStateRoot, mode,
				TemporaryPrefix(s.dao.Version.StoragePrefix), s.dao.Store)
			s.log.Info("MPT billet initialized",
				zap.Uint32("height", s.syncPoint),
				zap.String("state root", header.PrevStateRoot.StringBE()))
			pool := NewPool()
			pool.Add(header.PrevStateRoot, []byte{})
			err = s.billet.Traverse(func(_ []byte, n mpt.Node, _ []byte) bool {
				nPaths, ok := pool.TryGet(n.Hash())
				if !ok {
					// if this situation occurs, then it's a bug in MPT pool or Traverse.
					panic("failed to get MPT node from the pool")
				}
				pool.Remove(n.Hash())
				childrenPaths := make(map[util.Uint256][][]byte)
				for _, path := range nPaths {
					nChildrenPaths := mpt.GetChildrenPaths(path, n)
					for hash, paths := range nChildrenPaths {
						childrenPaths[hash] = append(childrenPaths[hash], paths...)
					}
				}
				pool.Update(nil, childrenPaths)
				return false
			}, true)
			if err != nil {
				return fmt.Errorf("failed to traverse MPT during initialization: %w", err)
			}
			s.mptpool.Update(nil, pool.GetAll())
			if s.mptpool.Count() == 0 {
				s.syncStage |= mptSynced
				s.log.Info("MPT is in sync",
					zap.Uint32("stateroot height", s.syncPoint))
			}
		}
	}

	if s.syncStage == headersSynced|blocksSynced|mptSynced {
		s.log.Info("state is in sync, starting regular blocks processing")
		s.syncStage = inactive
	}
	return nil
}

// getLatestSavedBlock returns either current block index (if it's still relevant
// to continue state sync process) or H-1 where H is the index of the earliest
// block that should be saved next.
func (s *Module) getLatestSavedBlock(p uint32) uint32 {
	var result uint32
	mtb := s.bc.GetConfig().MaxTraceableBlocks
	if p > mtb {
		result = p - mtb
	}
	storedH, err := s.dao.GetStateSyncCurrentBlockHeight()
	if err == nil && storedH > result {
		result = storedH
	}
	actualH := s.bc.BlockHeight()
	if actualH > result {
		result = actualH
	}
	return result
}

// AddHeaders validates and adds specified headers to the chain.
func (s *Module) AddHeaders(hdrs ...*block.Header) error {
	oldStage := s.syncStage
	s.lock.Lock()
	defer func() {
		if s.syncStage != oldStage {
			s.notifyStageChanged()
		}
	}()
	defer s.lock.Unlock()

	if s.syncStage != initialized {
		return errors.New("headers were not requested")
	}

	hdrsErr := s.bc.AddHeaders(hdrs...)
	if s.bc.HeaderHeight() > s.syncPoint {
		err := s.defineSyncStage()
		if err != nil {
			return fmt.Errorf("failed to define current sync stage: %w", err)
		}
	}
	return hdrsErr
}

// AddBlock verifies and saves block skipping executable scripts.
func (s *Module) AddBlock(block *block.Block) error {
	oldStage := s.syncStage
	s.lock.Lock()
	defer func() {
		if s.syncStage != oldStage {
			s.notifyStageChanged()
		}
	}()
	defer s.lock.Unlock()

	if s.syncStage&headersSynced == 0 || s.syncStage&blocksSynced != 0 {
		return nil
	}

	if s.blockHeight == s.syncPoint {
		return nil
	}
	expectedHeight := s.blockHeight + 1
	if expectedHeight != block.Index {
		return fmt.Errorf("expected %d, got %d: invalid block index", expectedHeight, block.Index)
	}
	if s.bc.GetConfig().StateRootInHeader != block.StateRootEnabled {
		return fmt.Errorf("stateroot setting mismatch: %v != %v", s.bc.GetConfig().StateRootInHeader, block.StateRootEnabled)
	}
	if !s.bc.GetConfig().SkipBlockVerification {
		merkle := block.ComputeMerkleRoot()
		if !block.MerkleRoot.Equals(merkle) {
			return errors.New("invalid block: MerkleRoot mismatch")
		}
	}
	cache := s.dao.GetPrivate()
	if err := cache.StoreAsBlock(block, nil, nil); err != nil {
		return err
	}

	cache.PutStateSyncCurrentBlockHeight(block.Index)

	for _, tx := range block.Transactions {
		if err := cache.StoreAsTransaction(tx, block.Index, nil); err != nil {
			return err
		}
	}

	_, err := cache.Persist()
	if err != nil {
		return fmt.Errorf("failed to persist results: %w", err)
	}
	s.blockHeight = block.Index
	if s.blockHeight == s.syncPoint {
		s.syncStage |= blocksSynced
		s.log.Info("blocks are in sync",
			zap.Uint32("blockHeight", s.blockHeight))
		s.checkSyncIsCompleted()
	}
	return nil
}

// AddMPTNodes tries to add provided set of MPT nodes to the MPT billet if they are
// not yet collected.
func (s *Module) AddMPTNodes(nodes [][]byte) error {
	if s.storageSync && !s.needMPTAfterStorage {
		return errors.New("MPT nodes not expected in storage-based sync mode")
	}
	oldStage := s.syncStage
	s.lock.Lock()
	defer func() {
		if s.syncStage != oldStage {
			s.notifyStageChanged()
		}
	}()
	defer s.lock.Unlock()

	if s.syncStage&headersSynced == 0 || s.syncStage&mptSynced != 0 {
		return errors.New("MPT nodes were not requested")
	}

	for _, nBytes := range nodes {
		var n mpt.NodeObject
		r := io.NewBinReaderFromBuf(nBytes)
		n.DecodeBinary(r)
		if r.Err != nil {
			return fmt.Errorf("failed to decode MPT node: %w", r.Err)
		}
		err := s.restoreNode(n.Node)
		if err != nil {
			return err
		}
	}
	if s.mptpool.Count() == 0 {
		s.syncStage |= mptSynced
		s.log.Info("MPT is in sync",
			zap.Uint32("height", s.syncPoint))
		s.checkSyncIsCompleted()
	}
	return nil
}

// AddContractStorageData adds a single key-value pair for storage-based sync.
func (s *Module) AddContractStorageData(key string, value []byte, syncHeight uint32, expectedRoot util.Uint256) error {
	if !s.storageSync {
		return errors.New("storage pair not expected in MPT-based mode")
	}
	oldStage := s.syncStage
	s.lock.Lock()
	defer func() {
		if s.syncStage != oldStage {
			s.notifyStageChanged()
		}
	}()
	defer s.lock.Unlock()

	if s.syncStage&headersSynced == 0 || s.syncStage&mptSynced != 0 {
		return errors.New("storage pair not requested")
	}

	if s.localTrie == nil {
		return errors.New("local trie not initialized")
	}

	if len(key) == 0 || len(key) > mpt.MaxKeyLength {
		return fmt.Errorf("invalid storage key length: %d", len(key))
	}

	// Check if syncHeight is outdated.
	if syncHeight < s.lastCheckpointHeight {
		s.log.Debug("skipping storage pair due to higher checkpoint",
			zap.Uint32("syncHeight", syncHeight),
			zap.Uint32("checkpointHeight", s.lastCheckpointHeight))
		return nil
	}

	// Check if key already exists.
	storeKey := append([]byte{byte(TemporaryPrefix(s.dao.Version.StoragePrefix))}, []byte(key)...)
	if _, err := s.dao.Store.Get(storeKey); err == nil {
		s.log.Debug("skipping storage pair, key already exists",
			zap.String("key", hex.EncodeToString([]byte(key))))
		return nil
	}
	if s.currentSyncHeight != 0 && (syncHeight != s.currentSyncHeight || len(s.pendingPairs) >= maxPendingPairs) {
		if err := s.flushPendingPairs(); err != nil {
			return fmt.Errorf("failed to flush pending pairs: %w", err)
		}
	}

	if s.currentSyncHeight == 0 {
		s.currentSyncHeight = syncHeight
		s.currentExpectedRoot = expectedRoot
	}

	s.pendingPairs[key] = append([]byte{}, value...)
	if len(s.pendingPairs) >= maxPendingPairs {
		if err := s.flushPendingPairs(); err != nil {
			return fmt.Errorf("failed to flush pending pairs: %w", err)
		}
	}

	return nil
}

// flushPendingPairs persists pending key-value pairs and updates state.
func (s *Module) flushPendingPairs() error {
	if len(s.pendingPairs) == 0 {
		return nil
	}

	prefix := TemporaryPrefix(s.dao.Version.StoragePrefix)
	var lastKey []byte
	for key, value := range s.pendingPairs {
		s.dao.Store.Put(append([]byte{byte(prefix)}, []byte(key)...), value)
		if err := s.localTrie.Put([]byte(key), value); err != nil {
			return fmt.Errorf("failed to put key %s into trie: %w", hex.EncodeToString([]byte(key)), err)
		}
		lastKey = append([]byte{}, key...)
	}
	s.localTrie.Flush(s.currentSyncHeight)
	if _, err := s.dao.PersistSync(); err != nil {
		return fmt.Errorf("failed to persist storage batch: %w", err)
	}

	metadata := checkpointMetadata{
		SyncHeight:   s.currentSyncHeight,
		MPTRoot:      s.localTrie.StateRoot(),
		ExpectedRoot: s.currentExpectedRoot,
		LastKey:      lastKey,
	}
	if err := s.saveCheckpointMetadata(metadata); err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}
	s.lastCheckpointHeight = s.currentSyncHeight

	s.log.Debug("flushed storage batch",
		zap.Int("items", len(s.pendingPairs)),
		zap.String("new_root", s.localTrie.StateRoot().String()),
		zap.Uint32("syncHeight", s.currentSyncHeight))

	// Check if syncHeight is sufficient to complete sync.
	if s.currentSyncHeight < s.syncPoint || s.localTrie.StateRoot() != s.currentExpectedRoot {
		s.needMPTAfterStorage = true
		s.log.Debug("storage sync incomplete, requesting MPT nodes",
			zap.Uint32("syncHeight", s.currentSyncHeight),
			zap.Uint32("targetPoint", s.syncPoint),
			zap.String("stateRoot", s.localTrie.StateRoot().StringLE()),
			zap.String("currentExpectedRoot", s.currentExpectedRoot.StringLE()))
	} else {
		s.tryJumpToState(s.currentSyncHeight, s.currentExpectedRoot)
	}

	s.pendingPairs = make(map[string][]byte)
	s.currentSyncHeight = 0
	s.currentExpectedRoot = util.Uint256{}

	return nil
}

// tryJumpToState attempts a state jump if all storage items are synced.
func (s *Module) tryJumpToState(syncHeight uint32, expectedRoot util.Uint256) {
	if s.localTrie == nil {
		s.log.Error("local trie not initialized")
		return
	}

	computedRoot := s.localTrie.StateRoot()
	if !computedRoot.Equals(expectedRoot) {
		s.log.Debug("state jump not ready",
			zap.String("computed_root", computedRoot.String()),
			zap.String("expected_root", expectedRoot.String()))
		return
	}

	sr := &state.MPTRoot{
		Index: syncHeight,
		Root:  computedRoot,
	}
	s.stateMod.JumpToState(sr)

	if n, err := s.dao.PersistSync(); err != nil {
		s.log.Error("failed to persist final state",
			zap.Error(err),
			zap.Int("items", n))
		return
	}

	s.dao.Store.Delete([]byte{byte(storage.SYSTempStateCheckpoint)})
	s.lastCheckpointHeight = 0

	s.log.Info("completed storage-based state sync",
		zap.Uint32("height", syncHeight),
		zap.String("root", computedRoot.String()))

	s.syncStage |= mptSynced
	s.checkSyncIsCompleted()
}

// saveCheckpointMetadata stores checkpoint metadata.
func (s *Module) saveCheckpointMetadata(metadata checkpointMetadata) error {
	if metadata.MPTRoot == (util.Uint256{}) {
		return errors.New("invalid MPT root")
	}
	data := io.NewBufBinWriter()
	data.WriteU32LE(metadata.SyncHeight)
	data.WriteBytes(metadata.MPTRoot[:])
	data.WriteBytes(metadata.ExpectedRoot[:])
	data.WriteVarBytes(metadata.LastKey)
	if data.Err != nil {
		return fmt.Errorf("failed to encode checkpoint metadata: %w", data.Err)
	}
	s.dao.Store.Put([]byte{byte(storage.SYSTempStateCheckpoint)}, data.Bytes())
	return nil
}

// loadCheckpointMetadata retrieves checkpoint metadata.
func (s *Module) loadCheckpointMetadata() (checkpointMetadata, error) {
	var metadata checkpointMetadata
	data, err := s.dao.Store.Get([]byte{byte(storage.SYSTempStateCheckpoint)})
	if err != nil {
		return metadata, fmt.Errorf("no checkpoint found: %w", err)
	}
	br := io.NewBinReaderFromBuf(data)
	metadata.SyncHeight = br.ReadU32LE()
	br.ReadBytes(metadata.MPTRoot[:])
	br.ReadBytes(metadata.ExpectedRoot[:])
	metadata.LastKey = br.ReadVarBytes()
	if br.Err != nil {
		return metadata, fmt.Errorf("failed to decode checkpoint metadata: %w", br.Err)
	}
	if metadata.MPTRoot == (util.Uint256{}) {
		return metadata, errors.New("invalid checkpoint metadata")
	}
	return metadata, nil
}

func (s *Module) restoreNode(n mpt.Node) error {
	nPaths, ok := s.mptpool.TryGet(n.Hash())
	if !ok {
		// it can easily happen after receiving the same data from different peers.
		return nil
	}
	var childrenPaths = make(map[util.Uint256][][]byte)
	for _, path := range nPaths {
		// Must clone here in order to avoid future collapse collisions. If the node's refcount>1 then MPT pool
		// will manage all paths for this node and call RestoreHashNode separately for each of the paths.
		err := s.billet.RestoreHashNode(path, n.Clone())
		if err != nil {
			return fmt.Errorf("failed to restore MPT node with hash %s and path %s: %w", n.Hash().StringBE(), hex.EncodeToString(path), err)
		}
		for h, paths := range mpt.GetChildrenPaths(path, n) {
			childrenPaths[h] = append(childrenPaths[h], paths...)
		}
	}

	s.mptpool.Update(map[util.Uint256][][]byte{n.Hash(): nPaths}, childrenPaths)

	for h := range childrenPaths {
		if child, err := s.billet.GetFromStore(h); err == nil {
			// child is already in the storage, so we don't need to request it one more time.
			err = s.restoreNode(child)
			if err != nil {
				return fmt.Errorf("unable to restore saved children: %w", err)
			}
		}
	}
	return nil
}

// checkSyncIsCompleted checks whether state sync process is completed, i.e. headers up to P+1
// height are fetched, blocks up to P height are stored and MPT nodes for P height are stored.
// If so, then jumping to P state sync point occurs. It is not protected by lock, thus caller
// should take care of it.
func (s *Module) checkSyncIsCompleted() {
	oldStage := s.syncStage
	defer func() {
		if s.syncStage != oldStage {
			s.notifyStageChanged()
		}
	}()
	if s.syncStage != headersSynced|mptSynced|blocksSynced {
		return
	}
	s.log.Info("state is in sync",
		zap.Uint32("state sync point", s.syncPoint))
	err := s.jumpCallback(s.syncPoint)
	if err != nil {
		s.log.Fatal("failed to jump to the latest state sync point", zap.Error(err))
	}
	s.syncStage = inactive
	s.dispose()
}

// dispose cleans up resources.
func (s *Module) dispose() {
	s.billet = nil
	s.localTrie = nil
	s.pendingPairs = make(map[string][]byte)
	s.currentSyncHeight = 0
	s.currentExpectedRoot = util.Uint256{}
	s.lastCheckpointHeight = 0
}

// BlockHeight returns index of the last stored block.
func (s *Module) BlockHeight() uint32 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.blockHeight
}

// IsActive tells whether state sync module is on and still gathering state
// synchronisation data (headers, blocks or MPT nodes).
func (s *Module) IsActive() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return !(s.syncStage == inactive || (s.syncStage == headersSynced|mptSynced|blocksSynced))
}

// IsInitialized tells whether state sync module does not require initialization.
// If `false` is returned then Init can be safely called.
func (s *Module) IsInitialized() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.syncStage != none
}

// NeedHeaders tells whether the module hasn't completed headers synchronisation.
func (s *Module) NeedHeaders() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.syncStage == initialized
}

// NeedMPTNodes returns whether the module hasn't completed MPT-based state synchronisation.
func (s *Module) NeedMPTNodes() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return (s.syncStage&headersSynced != 0 && s.syncStage&mptSynced == 0) &&
		(!s.storageSync || s.needMPTAfterStorage)
}

// NeedContractStorageData checks if storage-based state sync is incomplete.
func (s *Module) NeedContractStorageData() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.storageSync && !s.needMPTAfterStorage &&
		s.syncStage&headersSynced != 0 && s.syncStage&mptSynced == 0
}

// NeedBlocks returns whether the module hasn't completed blocks synchronisation.
func (s *Module) NeedBlocks() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.syncStage&headersSynced != 0 && s.syncStage&blocksSynced == 0
}

// Traverse traverses local MPT nodes starting from the specified root down to its
// children calling `process` for each serialised node until stop condition is satisfied.
func (s *Module) Traverse(root util.Uint256, process func(node mpt.Node, nodeBytes []byte) bool) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var mode mpt.TrieMode
	// GC must be turned off here to allow access to the archived nodes.
	if s.bc.GetConfig().Ledger.KeepOnlyLatestState || s.bc.GetConfig().Ledger.RemoveUntraceableBlocks {
		mode |= mpt.ModeLatest
	}
	b := mpt.NewBillet(root, mode, mpt.DummySTTempStoragePrefix, storage.NewMemCachedStore(s.dao.Store))
	return b.Traverse(func(pathToNode []byte, node mpt.Node, nodeBytes []byte) bool {
		return process(node, nodeBytes)
	}, false)
}

// GetUnknownMPTNodesBatch returns set of currently unknown MPT nodes (`limit` at max).
func (s *Module) GetUnknownMPTNodesBatch(limit int) []util.Uint256 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.mptpool.GetBatch(limit)
}

// HeaderHeight returns the height of the latest stored header.
func (s *Module) HeaderHeight() uint32 {
	return s.bc.HeaderHeight()
}

// GetConfig returns current blockchain configuration.
func (s *Module) GetConfig() config.Blockchain {
	return s.bc.GetConfig()
}
