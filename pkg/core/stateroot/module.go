package stateroot

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type (
	// VerifierFunc is a function that allows to check witness of account
	// for Hashable item with GAS limit.
	VerifierFunc func(util.Uint160, hash.Hashable, *transaction.Witness, int64) (int64, error)
	// Module represents module for local processing of state roots.
	Module struct {
		Store    *storage.MemCachedStore
		network  netmode.Magic
		srInHead bool
		mode     mpt.TrieMode
		mpt      *mpt.Trie
		verifier VerifierFunc
		log      *zap.Logger

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
func NewModule(cfg config.Blockchain, verif VerifierFunc, log *zap.Logger, s *storage.MemCachedStore) *Module {
	var mode mpt.TrieMode
	if cfg.Ledger.KeepOnlyLatestState {
		mode |= mpt.ModeLatest
	}
	if cfg.Ledger.RemoveUntraceableBlocks {
		mode |= mpt.ModeGC
	}
	return &Module{
		network:  cfg.Magic,
		srInHead: cfg.StateRootInHeader,
		mode:     mode,
		verifier: verif,
		log:      log,
		Store:    s,
	}
}

// GetState returns value at the specified key fom the MPT with the specified root.
func (s *Module) GetState(root util.Uint256, key []byte) ([]byte, error) {
	// Allow accessing old values, it's RO thing.
	tr := mpt.NewTrie(mpt.NewHashNode(root), s.mode&^mpt.ModeGCFlag, storage.NewMemCachedStore(s.Store))
	return tr.Get(key)
}

// FindStates returns set of key-value pairs with key matching the prefix starting
// from the `prefix`+`start` path from MPT trie with the specified root. `max` is
// the maximum number of elements to be returned. If nil `start` specified, then
// item with key equals to prefix is included into result; if empty `start` specified,
// then item with key equals to prefix is not included into result.
func (s *Module) FindStates(root util.Uint256, prefix, start []byte, max int) ([]storage.KeyValue, error) {
	// Allow accessing old values, it's RO thing.
	tr := mpt.NewTrie(mpt.NewHashNode(root), s.mode&^mpt.ModeGCFlag, storage.NewMemCachedStore(s.Store))
	return tr.Find(prefix, start, max)
}

// GetStateProof returns proof of having key in the MPT with the specified root.
func (s *Module) GetStateProof(root util.Uint256, key []byte) ([][]byte, error) {
	// Allow accessing old values, it's RO thing.
	tr := mpt.NewTrie(mpt.NewHashNode(root), s.mode&^mpt.ModeGCFlag, storage.NewMemCachedStore(s.Store))
	return tr.GetProof(key)
}

// GetStateRoot returns state root for a given height.
func (s *Module) GetStateRoot(height uint32) (*state.MPTRoot, error) {
	return s.getStateRoot(makeStateRootKey(height))
}

// GetLatestStateHeight returns the latest blockchain height by the given stateroot.
func (s *Module) GetLatestStateHeight(root util.Uint256) (uint32, error) {
	rootBytes := root.BytesBE()
	rootStartOffset := 1 + 4 // stateroot version (1 byte) + stateroot index (4 bytes)
	rootEndOffset := rootStartOffset + util.Uint256Size
	var (
		h       uint32
		found   bool
		rootKey = makeStateRootKey(s.localHeight.Load())
	)
	s.Store.Seek(storage.SeekRange{
		Prefix:    []byte{rootKey[0]}, // DataMPTAux
		Start:     rootKey[1:],        // Start is a value that should be appended to the Prefix
		Backwards: true,
	}, func(k, v []byte) bool {
		if len(k) == 5 && bytes.Equal(v[rootStartOffset:rootEndOffset], rootBytes) {
			h = binary.BigEndian.Uint32(k[1:]) // cut prefix DataMPTAux
			found = true
			return false
		}
		return true
	})
	if found {
		return h, nil
	}
	return h, storage.ErrKeyNotFound
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
func (s *Module) Init(height uint32) error {
	data, err := s.Store.Get([]byte{byte(storage.DataMPTAux), prefixValidated})
	if err == nil {
		s.validatedHeight.Store(binary.LittleEndian.Uint32(data))
	}

	if height == 0 {
		s.mpt = mpt.NewTrie(nil, s.mode, s.Store)
		s.currentLocal.Store(util.Uint256{})
		return nil
	}
	r, err := s.getStateRoot(makeStateRootKey(height))
	if err != nil {
		return err
	}
	s.currentLocal.Store(r.Root)
	s.localHeight.Store(r.Index)
	s.mpt = mpt.NewTrie(mpt.NewHashNode(r.Root), s.mode, s.Store)
	return nil
}

// CleanStorage removes all MPT-related data from the storage (MPT nodes, validated stateroots)
// except local stateroot for the current height and GC flag. This method is aimed to clean
// outdated MPT data before state sync process can be started.
// Note: this method is aimed to be called for genesis block only, an error is returned otherwise.
func (s *Module) CleanStorage() error {
	if s.localHeight.Load() != 0 {
		return fmt.Errorf("can't clean MPT data for non-genesis block: expected local stateroot height 0, got %d", s.localHeight.Load())
	}
	b := storage.NewMemCachedStore(s.Store)
	s.Store.Seek(storage.SeekRange{Prefix: []byte{byte(storage.DataMPT)}}, func(k, _ []byte) bool {
		// #1468, but don't need to copy here, because it is done by Store.
		b.Delete(k)
		return true
	})
	_, err := b.Persist()
	if err != nil {
		return fmt.Errorf("failed to remove outdated MPT-reated items: %w", err)
	}
	return nil
}

// JumpToState performs jump to the state specified by given stateroot index.
func (s *Module) JumpToState(sr *state.MPTRoot) {
	s.addLocalStateRoot(s.Store, sr)

	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, sr.Index)
	s.Store.Put([]byte{byte(storage.DataMPTAux), prefixValidated}, data)
	s.validatedHeight.Store(sr.Index)

	s.currentLocal.Store(sr.Root)
	s.localHeight.Store(sr.Index)
	s.mpt = mpt.NewTrie(mpt.NewHashNode(sr.Root), s.mode, s.Store)
}

// ResetState resets MPT state to the given height.
func (s *Module) ResetState(height uint32, cache *storage.MemCachedStore) error {
	// Update local stateroot.
	sr, err := s.GetStateRoot(height)
	if err != nil {
		return fmt.Errorf("failed to retrieve state root for height %d: %w", height, err)
	}
	s.addLocalStateRoot(cache, sr)

	// Remove all stateroots newer than the given height.
	srKey := makeStateRootKey(height)
	var srSeen bool
	cache.Seek(storage.SeekRange{
		Prefix:    srKey[0:1],
		Start:     srKey[1:5],
		Backwards: false,
	}, func(k, v []byte) bool {
		if len(k) == 5 {
			if srSeen {
				cache.Delete(k)
			} else if bytes.Equal(k, srKey) {
				srSeen = true
			}
		}
		return true
	})

	// Retrieve the most recent validated stateroot before the given height.
	witnessesLenOffset := 1 /* version */ + 4 /* index */ + smartcontract.Hash256Len /* root */
	var validated *uint32
	cache.Seek(storage.SeekRange{
		Prefix:    srKey[0:1],
		Start:     srKey[1:5],
		Backwards: true,
	}, func(k, v []byte) bool {
		if len(k) == 5 {
			if len(v) > witnessesLenOffset && v[witnessesLenOffset] != 0 {
				i := binary.BigEndian.Uint32(k[1:])
				validated = &i
				return false
			}
		}
		return true
	})
	if validated != nil {
		validatedBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(validatedBytes, *validated)
		cache.Put([]byte{byte(storage.DataMPTAux), prefixValidated}, validatedBytes)
		s.validatedHeight.Store(*validated)
	} else {
		cache.Delete([]byte{byte(storage.DataMPTAux), prefixValidated})
	}

	s.currentLocal.Store(sr.Root)
	s.localHeight.Store(sr.Index)
	s.mpt = mpt.NewTrie(mpt.NewHashNode(sr.Root), s.mode, s.Store)

	// Do not reset MPT nodes, leave the trie state itself as is.
	return nil
}

// GC performs garbage collection.
func (s *Module) GC(index uint32, store storage.Store) time.Duration {
	if !s.mode.GC() {
		panic("stateroot: GC invoked, but not enabled")
	}
	var removed int
	var stored int64
	s.log.Info("starting MPT garbage collection", zap.Uint32("index", index))
	start := time.Now()
	err := store.SeekGC(storage.SeekRange{
		Prefix: []byte{byte(storage.DataMPT)},
	}, func(k, v []byte) bool {
		stored++
		if !mpt.IsActiveValue(v) {
			h := binary.LittleEndian.Uint32(v[len(v)-4:])
			if h <= index {
				removed++
				stored--
				return false
			}
		}
		return true
	})
	dur := time.Since(start)
	if err != nil {
		s.log.Error("failed to flush MPT GC changeset", zap.Duration("time", dur), zap.Error(err))
	} else {
		s.log.Info("finished MPT garbage collection",
			zap.Int("removed", removed),
			zap.Int64("kept", stored),
			zap.Duration("time", dur))
	}
	return dur
}

// AddMPTBatch updates using provided batch.
func (s *Module) AddMPTBatch(index uint32, b mpt.Batch, cache *storage.MemCachedStore) (*mpt.Trie, *state.MPTRoot, error) {
	mpt := *s.mpt
	mpt.Store = cache
	if _, err := mpt.PutBatch(b); err != nil {
		return nil, nil, err
	}
	mpt.Flush(index)
	sr := &state.MPTRoot{
		Index: index,
		Root:  mpt.StateRoot(),
	}
	s.addLocalStateRoot(cache, sr)
	return &mpt, sr, nil
}

// UpdateCurrentLocal updates local caches using provided state root.
func (s *Module) UpdateCurrentLocal(mpt *mpt.Trie, sr *state.MPTRoot) {
	s.mpt = mpt
	s.currentLocal.Store(sr.Root)
	s.localHeight.Store(sr.Index)
	if s.srInHead {
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
	_, err := s.verifier(h, r, &r.Witness[0], maxVerificationGAS)
	return err
}
