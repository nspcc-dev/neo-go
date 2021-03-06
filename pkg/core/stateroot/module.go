package stateroot

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type (
	// Module represents module for local processing of state roots.
	Module struct {
		Store   *storage.MemCachedStore
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
	tr := mpt.NewTrie(mpt.NewHashNode(root), false, storage.NewMemCachedStore(s.Store))
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
func (s *Module) Init(height uint32, enableRefCount bool) error {
	data, err := s.Store.Get([]byte{byte(storage.DataMPT), prefixValidated})
	if err == nil {
		s.validatedHeight.Store(binary.LittleEndian.Uint32(data))
	}

	var gcKey = []byte{byte(storage.DataMPT), prefixGC}
	if height == 0 {
		s.mpt = mpt.NewTrie(nil, enableRefCount, s.Store)
		var val byte
		if enableRefCount {
			val = 1
		}
		s.currentLocal.Store(util.Uint256{})
		return s.Store.Put(gcKey, []byte{val})
	}
	var hasRefCount bool
	if v, err := s.Store.Get(gcKey); err == nil {
		hasRefCount = v[0] != 0
	}
	if hasRefCount != enableRefCount {
		return fmt.Errorf("KeepOnlyLatestState setting mismatch: old=%v, new=%v", hasRefCount, enableRefCount)
	}
	r, err := s.getStateRoot(makeStateRootKey(height))
	if err != nil {
		return err
	}
	s.currentLocal.Store(r.Root)
	s.localHeight.Store(r.Index)
	s.mpt = mpt.NewTrie(mpt.NewHashNode(r.Root), enableRefCount, s.Store)
	return nil
}

// AddMPTBatch updates using provided batch.
func (s *Module) AddMPTBatch(index uint32, b mpt.Batch, cache *storage.MemCachedStore) (*mpt.Trie, *state.MPTRoot, error) {
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
