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
}

// NewModule returns new instance of statesync module.
func NewModule(bc blockchainer.Blockchainer, log *zap.Logger, s *dao.Simple) *Module {
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
	} else if s.bc.BlockHeight() > p-2*s.syncInterval {
		// chain has already been synchronised up to old state sync point and regular blocks processing was started
		s.syncStage = inactive
		return nil
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

	// check headers sync state first
	ltstHeaderHeight := s.bc.HeaderHeight()
	if ltstHeaderHeight > p {
		s.syncStage = headersSynced
		s.log.Info("headers are in sync",
			zap.Uint32("headerHeight", s.bc.HeaderHeight()))
	}

	// check blocks sync state
	s.blockHeight = s.getLatestSavedBlock(p)
	if s.blockHeight >= p {
		s.syncStage |= blocksSynced
		s.log.Info("blocks are in sync",
			zap.Uint32("blockHeight", s.blockHeight))
	}

	// check MPT sync state
	if s.blockHeight > p {
		s.syncStage |= mptSynced
		s.log.Info("MPT is in sync",
			zap.Uint32("stateroot height", s.bc.GetStateModule().CurrentLocalHeight()))
	} else if s.syncStage&headersSynced != 0 {
		header, err := s.bc.GetHeader(s.bc.GetHeaderHash(int(p + 1)))
		if err != nil {
			return fmt.Errorf("failed to get header to initialize MPT billet: %w", err)
		}
		s.billet = mpt.NewBillet(header.PrevStateRoot, s.bc.GetConfig().KeepOnlyLatestState, s.dao.Store)
		s.log.Info("MPT billet initialized",
			zap.Uint32("height", s.syncPoint),
			zap.String("state root", header.PrevStateRoot.StringBE()))
		pool := NewPool()
		pool.Add(header.PrevStateRoot, []byte{})
		err = s.billet.Traverse(func(n mpt.Node, _ []byte) bool {
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
			return fmt.Errorf("failed to traverse MPT while initialization: %w", err)
		}
		s.mptpool.Update(nil, pool.GetAll())
		if s.mptpool.Count() == 0 {
			s.syncStage |= mptSynced
			s.log.Info("MPT is in sync",
				zap.Uint32("stateroot height", p))
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
		s.syncStage = headersSynced
		s.log.Info("headers for state sync are fetched",
			zap.Uint32("header height", s.bc.HeaderHeight()))

		header, err := s.bc.GetHeader(s.bc.GetHeaderHash(int(s.syncPoint) + 1))
		if err != nil {
			s.log.Fatal("failed to get header to initialize MPT billet",
				zap.Uint32("height", s.syncPoint+1),
				zap.Error(err))
		}
		s.billet = mpt.NewBillet(header.PrevStateRoot, s.bc.GetConfig().KeepOnlyLatestState, s.dao.Store)
		s.mptpool.Add(header.PrevStateRoot, []byte{})
		s.log.Info("MPT billet initialized",
			zap.Uint32("height", s.syncPoint),
			zap.String("state root", header.PrevStateRoot.StringBE()))
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
		nPaths, ok := s.mptpool.TryGet(n.Hash())
		if !ok {
			// it can easily happen after receiving the same data from different peers.
			return nil
		}

		var childrenPaths = make(map[util.Uint256][][]byte)
		for _, path := range nPaths {
			err := s.billet.RestoreHashNode(path, n.Node)
			if err != nil {
				return fmt.Errorf("failed to add MPT node with hash %s and path %s: %w", n.Hash().StringBE(), hex.EncodeToString(path), err)
			}
			for h, paths := range mpt.GetChildrenPaths(path, n.Node) {
				childrenPaths[h] = append(childrenPaths[h], paths...) // it's OK to have duplicates, they'll be handled by mempool
			}
		}

		s.mptpool.Update(map[util.Uint256][][]byte{n.Hash(): nPaths}, childrenPaths)
	}
	if s.mptpool.Count() == 0 {
		s.syncStage |= mptSynced
		s.log.Info("MPT is in sync",
			zap.Uint32("height", s.syncPoint))
		s.checkSyncIsCompleted()
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
	err := s.bc.JumpToState(s)
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

	b := mpt.NewBillet(root, s.bc.GetConfig().KeepOnlyLatestState, storage.NewMemCachedStore(s.dao.Store))
	return b.Traverse(process, false)
}

// GetJumpHeight returns state sync point to jump to. It is not protected by mutex and should be called
// under the module lock.
func (s *Module) GetJumpHeight() (uint32, error) {
	if s.syncStage != headersSynced|mptSynced|blocksSynced {
		return 0, errors.New("state sync module has wong state to perform state jump")
	}
	return s.syncPoint, nil
}
