package stateroot

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type (
	// Module represents module for local processing of state roots.
	Module struct {
		Store   *storage.MemCachedStore
		PS      storage.Store
		network netmode.Magic
		mpt     *mpt.Trie
		bc      blockchainer.Blockchainer
		log     *zap.Logger

		currentLocal    atomic.Value
		localHeight     atomic.Uint32
		validatedHeight atomic.Uint32

		mtx  sync.RWMutex
		keys []keyCache

		updateValidatorsCb func(height uint32, publicKeys keys.PublicKeys)
	}

	keyCache struct {
		height           uint32
		validatorsKeys   keys.PublicKeys
		validatorsHash   util.Uint160
		validatorsScript []byte
	}
)

// NewModule returns new instance of stateroot module.
func NewModule(bc blockchainer.Blockchainer, log *zap.Logger, s *storage.MemCachedStore) *Module {
	return &Module{
		network: bc.GetConfig().Magic,
		bc:      bc,
		log:     log,
		Store:   s,
	}
}

// GetStateProof returns proof of having key in the MPT with the specified root.
func (s *Module) GetStateProof(root util.Uint256, key []byte) ([][]byte, error) {
	tr := mpt.NewTrie(mpt.NewHashNode(root), mpt.Config{Store: storage.NewMemCachedStore(s.Store)})
	return tr.GetProof(key)
}

// GetStateRoot returns state root for a given height.
func (s *Module) GetStateRoot(height uint32) (*state.MPTRoot, error) {
	return s.getStateRoot(makeStateRootKey(height))
}

// CurrentLocalStateRoot returns hash of the local state root.
func (s *Module) CurrentLocalStateRoot() util.Uint256 {
	return s.currentLocal.Load().(util.Uint256)
}

// CurrentLocalHeight returns height of the local state root.
func (s *Module) CurrentLocalHeight() uint32 {
	return s.localHeight.Load()
}

// CurrentValidatedHeight returns current state root validated height.
func (s *Module) CurrentValidatedHeight() uint32 {
	return s.validatedHeight.Load()
}

// Init initializes state root module at the given height.
func (s *Module) Init(height uint32, span uint32, enableRefCount, removeUntraceable bool) error {
	data, err := s.Store.Get([]byte{byte(storage.DataMPT), prefixValidated})
	if err == nil {
		s.validatedHeight.Store(binary.LittleEndian.Uint32(data))
	}

	// Removing untraceable blocks needs reference counters.
	var gcKey = []byte{byte(storage.DataMPT), prefixGC}
	if height == 0 {
		s.mpt = mpt.NewTrie(nil, mpt.Config{
			Store:           s.Store,
			RefCountEnabled: enableRefCount,
			GCEnabled:       removeUntraceable,
			GenerationSpan:  span,
		})
		val := make([]byte, 5)
		if enableRefCount {
			val[0] = 1
		}
		if removeUntraceable {
			val[0] |= 2
		}
		binary.LittleEndian.PutUint32(val[1:], span)
		s.currentLocal.Store(util.Uint256{})
		return s.Store.Put(gcKey, val)
	}

	var hasRefCount, hasRemoveUntraceable bool
	var oldSpan uint32
	if v, err := s.Store.Get(gcKey); err == nil {
		hasRefCount = v[0]&1 != 0
		hasRemoveUntraceable = v[0]&2 != 0
		oldSpan = binary.LittleEndian.Uint32(v[1:])
	}
	if hasRefCount != enableRefCount {
		return fmt.Errorf("KeepOnlyLatestState setting mismatch: old=%v, new=%v", hasRefCount, enableRefCount)
	}
	if hasRemoveUntraceable != removeUntraceable {
		return fmt.Errorf("RemoveUntraceableBlocks setting mismatch: old=%v, new=%v",
			hasRemoveUntraceable, removeUntraceable)
	}
	if oldSpan != span {
		return fmt.Errorf("mismatched generation spans: old=%v, new=%v (wrong StateSyncInterval?)", oldSpan, span)
	}
	r, err := s.getStateRoot(makeStateRootKey(height))
	if err != nil {
		return err
	}
	s.currentLocal.Store(r.Root)
	s.localHeight.Store(r.Index)
	s.mpt = mpt.NewTrie(mpt.NewHashNode(r.Root), mpt.Config{
		Generation:      r.Index / span,
		GenerationSpan:  span,
		RefCountEnabled: enableRefCount,
		GCEnabled:       removeUntraceable,
		Store:           s.Store,
	})
	return nil
}

// CleanStorage removes all MPT-related data from the storage (MPT nodes, validated stateroots)
// except local stateroot for the current height and GC flag. This method is aimed to clean
// outdated MPT data before state sync process can be started.
// Note: this method is aimed to be called for genesis block only, an error is returned otherwice.
func (s *Module) CleanStorage() error {
	if s.localHeight.Load() != 0 {
		return fmt.Errorf("can't clean MPT data for non-genesis block: expected local stateroot height 0, got %d", s.localHeight.Load())
	}
	gcKey := []byte{byte(storage.DataMPT), prefixGC}
	gcVal, err := s.Store.Get(gcKey)
	if err != nil {
		return fmt.Errorf("failed to get GC flag: %w", err)
	}
	//
	b := s.Store.Batch()
	s.Store.Seek([]byte{byte(storage.DataMPT)}, func(k, _ []byte) {
		// Must copy here, #1468.
		key := slice.Copy(k)
		b.Delete(key)
	})
	err = s.Store.PutBatch(b)
	if err != nil {
		return fmt.Errorf("failed to remove outdated MPT-reated items: %w", err)
	}
	err = s.Store.Put(gcKey, gcVal)
	if err != nil {
		return fmt.Errorf("failed to store GC flag: %w", err)
	}
	currentLocal := s.currentLocal.Load().(util.Uint256)
	if !currentLocal.Equals(util.Uint256{}) {
		err := s.addLocalStateRoot(s.Store, &state.MPTRoot{
			Index: s.localHeight.Load(),
			Root:  currentLocal,
		})
		if err != nil {
			return fmt.Errorf("failed to store current local stateroot: %w", err)
		}
	}
	return nil
}

// JumpToState performs jump to the state specified by given stateroot index.
func (s *Module) JumpToState(sr *state.MPTRoot, enableRefCount, removeUntraceable bool) error {
	if err := s.addLocalStateRoot(s.Store, sr); err != nil {
		return fmt.Errorf("failed to store local state root: %w", err)
	}

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, sr.Index)
	if err := s.Store.Put([]byte{byte(storage.DataMPT), prefixValidated}, data); err != nil {
		return fmt.Errorf("failed to store validated height: %w", err)
	}
	s.validatedHeight.Store(sr.Index)

	s.currentLocal.Store(sr.Root)
	s.localHeight.Store(sr.Index)

	gcKey := []byte{byte(storage.DataMPT), prefixGC}
	gcVal, err := s.Store.Get(gcKey)
	if err != nil {
		return fmt.Errorf("failed to get GC flag: %w", err)
	}
	span := binary.LittleEndian.Uint32(gcVal[1:])
	s.mpt = mpt.NewTrie(mpt.NewHashNode(sr.Root), mpt.Config{
		Generation:      sr.Index / span,
		GenerationSpan:  span,
		Store:           s.Store,
		RefCountEnabled: enableRefCount,
		GCEnabled:       removeUntraceable,
	})
	return nil
}

// RemoveMPTAtHeight removes all MPT data for height. It should be executed
// only when reference counting is enabled.
func (s *Module) RemoveMPTAtHeight(height uint32) error {
	sr, err := s.GetStateRoot(height)
	if err != nil {
		return fmt.Errorf("can't get old state root: %w", err)
	}

	return mpt.RemoveRoot(sr.Root, mpt.Config{
		Generation:      height / s.mpt.GenerationSpan,
		GenerationSpan:  s.mpt.GenerationSpan,
		Store:           s.Store,
		RefCountEnabled: true,
	})
}

// Shutdown stops all MPT-related running processes if any.
func (s *Module) Shutdown() {
	s.mpt.ShutdownGC()
}

// AddMPTBatch updates using provided batch.
func (s *Module) AddMPTBatch(index uint32, b mpt.Batch, cache *storage.MemCachedStore) (*mpt.Trie, *state.MPTRoot, error) {
	s.mpt.Generation = index / s.mpt.GenerationSpan
	mpt := *s.mpt
	mpt.Store = cache
	if _, err := mpt.PutBatch(b); err != nil {
		return nil, nil, err
	}
	mpt.Flush()

	sr := &state.MPTRoot{
		Index: index,
		Root:  mpt.StateRoot(),
	}
	err := s.addLocalStateRoot(cache, sr)
	if err != nil {
		return nil, nil, err
	}
	return &mpt, sr, err
}

func (s *Module) RunGC(index uint32, st storage.Store) {
	mptGen := index / s.mpt.GenerationSpan
	if index%s.mpt.GenerationSpan == 0 && mptGen > 1 {
		mp := *s.mpt
		root := mp.StateRoot()
		s.log.Info("start GC", zap.Uint32("generation", mptGen),
			zap.String("root", root.StringLE()))
		start := time.Now()
		go func() {
			n, err := mp.PerformGC(root, st)
			if err != nil {
				s.log.Error("error during MPT GC", zap.Error(err))
			}
			s.log.Info("finish GC",
				zap.Int("removed", n),
				zap.Uint32("generation", mptGen),
				zap.Duration("time", time.Since(start)))
		}()

	}
}

// UpdateCurrentLocal updates local caches using provided state root.
func (s *Module) UpdateCurrentLocal(mpt *mpt.Trie, sr *state.MPTRoot) {
	s.mpt = mpt
	s.currentLocal.Store(sr.Root)
	s.localHeight.Store(sr.Index)
	if s.bc.GetConfig().StateRootInHeader {
		s.validatedHeight.Store(sr.Index)
		updateStateHeightMetric(sr.Index)
	}
}

// VerifyStateRoot checks if state root is valid.
func (s *Module) VerifyStateRoot(r *state.MPTRoot) error {
	_, err := s.getStateRoot(makeStateRootKey(r.Index - 1))
	if err != nil {
		return errors.New("can't get previous state root")
	}
	if len(r.Witness) != 1 {
		return errors.New("no witness")
	}
	return s.verifyWitness(r)
}

const maxVerificationGAS = 2_00000000

// verifyWitness verifies state root witness.
func (s *Module) verifyWitness(r *state.MPTRoot) error {
	s.mtx.Lock()
	h := s.getKeyCacheForHeight(r.Index).validatorsHash
	s.mtx.Unlock()
	return s.bc.VerifyWitness(h, r, &r.Witness[0], maxVerificationGAS)
}
