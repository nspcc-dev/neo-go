/*
Package statesync implements module for the P2P state synchronisation process. The
module manages state synchronisation for non-archival nodes which are joining the
network and don't have the ability to resync from the genesis block.

Given the currently available state synchronisation point P, sate sync process
includes the following stages:

1. Fetching headers starting from height 0 up to P+1.
2. Fetching MPT nodes for height P stating from the corresponding state root.
3. Fetching blocks starting from height P-MaxTraceableBlocks (or 0) up to P.

Steps 2 and 3 are being performed in parallel. Once all the data are collected
and stored in the db, an atomic state jump is occurred to the state sync point P.
Further node operation process is performed using standard sync mechanism until
the node reaches synchronised state.
*/
package statesync

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
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

	dao     *dao.Simple
	bc      blockchainer.Blockchainer
	mptpool *Pool

	billet *mpt.Billet

	jumpCallback func(p uint32) error
}

// NewModule returns new instance of statesync module.
func NewModule(bc blockchainer.Blockchainer, log *zap.Logger, s *dao.Simple, jumpCallback func(p uint32) error) *Module {
	if !(bc.GetConfig().P2PStateExchangeExtensions && bc.GetConfig().RemoveUntraceableBlocks) {
		return &Module{
			dao:       s,
			bc:        bc,
			syncStage: inactive,
		}
	}
	return &Module{
		dao:          s,
		bc:           bc,
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
	s.lock.Lock()
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
		err = s.bc.GetStateModule().CleanStorage()
		if err != nil {
			return fmt.Errorf("failed to remove outdated MPT data from storage: %w", err)
		}
	}

	s.syncPoint = p
	err = s.dao.PutStateSyncPoint(p)
	if err != nil {
		return fmt.Errorf("failed to store state synchronisation point %d: %w", p, err)
	}
	s.syncStage = initialized
	s.log.Info("try to sync state for the latest state synchronisation point",
		zap.Uint32("point", p),
		zap.Uint32("evaluated chain's blockHeight", currChainHeight))

	return s.defineSyncStage()
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
			zap.Uint32("stateroot height", s.bc.GetStateModule().CurrentLocalHeight()))
	} else if s.syncStage&headersSynced != 0 {
		header, err := s.bc.GetHeader(s.bc.GetHeaderHash(int(s.syncPoint + 1)))
		if err != nil {
			return fmt.Errorf("failed to get header to initialize MPT billet: %w", err)
		}
		s.billet = mpt.NewBillet(header.PrevStateRoot, s.bc.GetConfig().KeepOnlyLatestState,
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
	s.lock.Lock()
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
	s.lock.Lock()
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
	if s.bc.GetConfig().VerifyBlocks {
		merkle := block.ComputeMerkleRoot()
		if !block.MerkleRoot.Equals(merkle) {
			return errors.New("invalid block: MerkleRoot mismatch")
		}
	}
	cache := s.dao.GetWrapped()
	writeBuf := io.NewBufBinWriter()
	if err := cache.StoreAsBlock(block, writeBuf); err != nil {
		return err
	}
	writeBuf.Reset()

	err := cache.PutStateSyncCurrentBlockHeight(block.Index)
	if err != nil {
		return fmt.Errorf("failed to store current block height: %w", err)
	}

	for _, tx := range block.Transactions {
		if err := cache.StoreAsTransaction(tx, block.Index, writeBuf); err != nil {
			return err
		}
		writeBuf.Reset()
	}

	_, err = cache.Persist()
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
	s.lock.Lock()
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

// NeedMPTNodes returns whether the module hasn't completed MPT synchronisation.
func (s *Module) NeedMPTNodes() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.syncStage&headersSynced != 0 && s.syncStage&mptSynced == 0
}

// Traverse traverses local MPT nodes starting from the specified root down to its
// children calling `process` for each serialised node until stop condition is satisfied.
func (s *Module) Traverse(root util.Uint256, process func(node mpt.Node, nodeBytes []byte) bool) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	b := mpt.NewBillet(root, s.bc.GetConfig().KeepOnlyLatestState, 0, storage.NewMemCachedStore(s.dao.Store))
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
