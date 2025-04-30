/*
Package statesync implements module for the P2P state synchronisation process. The
module manages state synchronisation for non-archival nodes which are joining the
network and don't have the ability to resync from the genesis block.

Given the currently available state synchronisation point P, sate sync process
includes the following stages:

1. Fetching headers starting from height 0 up to P+1.
2. Fetching MPT nodes for height P stating from the corresponding state root.
3. Fetching blocks starting from height P-MaxTraceableBlocks(P) (or 0) up to P.

All steps are being performed sequentially. Once all the data are collected
and stored in the db, an atomic state jump is occurred to the state sync point P.
Further node operation process is performed using standard sync mechanism until
the node reaches synchronised state.
*/
package statesync

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
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
	// Always combined with headersSynced and always combined with mptSynced.
	blocksSynced
)

// Ledger is the interface required from Blockchain for Module to operate.
type Ledger interface {
	AddHeaders(...*block.Header) error
	BlockHeight() uint32
	IsHardforkEnabled(hf *config.Hardfork, blockHeight uint32) bool
	GetConfig() config.Blockchain
	GetHeader(hash util.Uint256) (*block.Header, error)
	GetHeaderHash(uint32) util.Uint256
	HeaderHeight() uint32
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

	dao      *dao.Simple
	bc       Ledger
	stateMod *stateroot.Module
	mptpool  *Pool

	billet *mpt.Billet

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
		dao:          s,
		bc:           bc,
		stateMod:     stateMod,
		log:          log,
		syncInterval: uint32(bc.GetConfig().StateSyncInterval),
		mptpool:      NewPool(),
		syncStage:    none,
		jumpCallback: jumpCallback,
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

	// check MPT sync stage
	if s.syncStage&headersSynced != 0 {
		header, err := s.bc.GetHeader(s.bc.GetHeaderHash(s.syncPoint + 1))
		if err != nil {
			return fmt.Errorf("failed to get header to initialize MPT billet: %w", err)
		}
		var mode mpt.TrieMode
		// No need to enable GC here, it only has latest things.
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
					childrenPaths[hash] = append(childrenPaths[hash], paths...) // it's OK to have duplicates, they'll be handled by mempool
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

	// Check blocks sync stage. mptSynced is required since MaxTraceableBlocks is a part of the state.
	if s.syncStage&mptSynced != 0 {
		s.blockHeight = s.getLatestSavedBlock(s.syncPoint)
		if s.blockHeight >= s.syncPoint {
			s.syncStage |= blocksSynced
			s.log.Info("blocks are in sync",
				zap.Uint32("blockHeight", s.blockHeight))
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
// block that should be saved next. It performs access to native Policy storage
// by temporary storage key hence it's a no-op to call this method if contract storage
// is not yet initialized.
func (s *Module) getLatestSavedBlock(p uint32) uint32 {
	var (
		result uint32
		mtb    = s.bc.GetConfig().MaxTraceableBlocks
		hf     = config.HFEchidna
	)
	if s.bc.IsHardforkEnabled(&hf, p) {
		// Retrieve MaxTraceableBlocks from DAO directly using temporary storage prefix.
		policyID := native.PolicyContractID
		key := make([]byte, 1+4+1)
		key[0] = byte(TemporaryPrefix(s.dao.Version.StoragePrefix))
		binary.LittleEndian.PutUint32(key[1:], uint32(policyID))
		copy(key[5:], native.MaxTraceableBlocksKey)
		si, err := s.dao.Store.Get(key)
		if err != nil {
			if errors.Is(err, storage.ErrKeyNotFound) {
				// The only situation when it's possible is when state sync was already completed,
				// DB storage prefix was swapped with temporary storage prefix and old storage items
				// were already removed during state jump. Hence, we need to use the current DB
				// prefix to retrieve MaxTraceableBlocks value.
				key[0] = byte(TemporaryPrefix(storage.KeyPrefix(key[0])))
				si, err = s.dao.Store.Get(key)
			}
			if err != nil {
				panic(fmt.Errorf("failed to retrieve MaxTraceableBlock storage item from Policy contract storage by key %s at height %d: %w", hex.EncodeToString(key), p, err))
			}
		}
		mtb = uint32(bigint.FromBytes(si).Int64())
	}
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

	if s.syncStage&headersSynced == 0 || s.syncStage&mptSynced == 0 || s.syncStage&blocksSynced != 0 {
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
	oldStage := s.syncStage
	s.lock.Lock()
	defer func() {
		if s.syncStage != oldStage {
			s.notifyStageChanged()
		}
	}()
	defer s.lock.Unlock()

	if s.syncStage&headersSynced == 0 || s.syncStage&mptSynced != 0 {
		return fmt.Errorf("MPT nodes were not requested: current state sync stage is %d", s.syncStage)
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
		s.blockHeight = s.getLatestSavedBlock(s.syncPoint)
	}
	return nil
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
			childrenPaths[h] = append(childrenPaths[h], paths...) // it's OK to have duplicates, they'll be handled by mempool
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

func (s *Module) dispose() {
	s.billet = nil
}

// BlockHeight returns index of the last stored block. It's a no-op to call this method until MPT is in sync
// since block height initialization requires access to the node state at state synchronisation point.
func (s *Module) BlockHeight() uint32 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if s.syncStage != inactive && s.syncStage&mptSynced == 0 {
		// It's a program bug.
		panic("block height is not yet initialized since MPT is not in sync")
	}

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

// NeedMPTNodes returns whether the module hasn't completed MPT synchronisation.
func (s *Module) NeedMPTNodes() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.syncStage&headersSynced != 0 && s.syncStage&mptSynced == 0
}

// NeedBlocks returns whether the module hasn't completed blocks synchronisation.
func (s *Module) NeedBlocks() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.syncStage&headersSynced != 0 && s.syncStage&mptSynced != 0 && s.syncStage&blocksSynced == 0
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
