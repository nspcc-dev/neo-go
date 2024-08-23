package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	json "github.com/nspcc-dev/go-ordered-json"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/statesync"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"go.uber.org/zap"
)

// Tuning parameters.
const (
	version = "0.2.12"

	// DefaultInitialGAS is the default amount of GAS emitted to the standby validators
	// multisignature account during native GAS contract initialization.
	DefaultInitialGAS                      = 52000000_00000000
	defaultGCPeriod                        = 10000
	defaultMemPoolSize                     = 50000
	defaultP2PNotaryRequestPayloadPoolSize = 1000
	defaultMaxBlockSize                    = 262144
	defaultMaxBlockSystemFee               = 900000000000
	defaultMaxTraceableBlocks              = 2102400 // 1 year of 15s blocks
	defaultMaxTransactionsPerBlock         = 512
	defaultTimePerBlock                    = 15 * time.Second
	// HeaderVerificationGasLimit is the maximum amount of GAS for block header verification.
	HeaderVerificationGasLimit = 3_00000000 // 3 GAS
	defaultStateSyncInterval   = 40000
)

// stateChangeStage denotes the stage of state modification process.
type stateChangeStage byte

// A set of stages used to split state jump / state reset into atomic operations.
const (
	// none means that no state jump or state reset process was initiated yet.
	none stateChangeStage = 1 << iota
	// stateJumpStarted means that state jump was just initiated, but outdated storage items
	// were not yet removed.
	stateJumpStarted
	// newStorageItemsAdded means that contract storage items are up-to-date with the current
	// state.
	newStorageItemsAdded
	// staleBlocksRemoved means that state corresponding to the stale blocks (genesis block in
	// in case of state jump) was removed from the storage.
	staleBlocksRemoved
	// headersReset denotes stale SYS-prefixed and IX-prefixed information was removed from
	// the storage (applicable to state reset only).
	headersReset
	// transfersReset denotes NEP transfers were successfully updated (applicable to state reset only).
	transfersReset
	// stateResetBit represents a bit identifier for state reset process. If this bit is not set, then
	// it's an unfinished state jump.
	stateResetBit byte = 1 << 7
)

var (
	// ErrAlreadyExists is returned when trying to add some transaction
	// that already exists on chain.
	ErrAlreadyExists = errors.New("already exists in blockchain")
	// ErrAlreadyInPool is returned when trying to add some already existing
	// transaction into the mempool.
	ErrAlreadyInPool = errors.New("already exists in mempool")
	// ErrOOM is returned when adding transaction to the memory pool because
	// it reached its full capacity.
	ErrOOM = errors.New("no space left in the memory pool")
	// ErrPolicy is returned on attempt to add transaction that doesn't
	// comply with node's configured policy into the mempool.
	ErrPolicy = errors.New("not allowed by policy")
	// ErrInvalidBlockIndex is returned when trying to add block with index
	// other than expected height of the blockchain.
	ErrInvalidBlockIndex = errors.New("invalid block index")
	// ErrHasConflicts is returned when trying to add some transaction which
	// conflicts with other transaction in the chain or pool according to
	// Conflicts attribute.
	ErrHasConflicts = errors.New("has conflicts")
)
var (
	persistInterval = 1 * time.Second
)

// Blockchain represents the blockchain. It maintans internal state representing
// the state of the ledger that can be accessed in various ways and changed by
// adding new blocks or headers.
type Blockchain struct {
	HeaderHashes

	config config.Blockchain

	// The only way chain state changes is by adding blocks, so we can't
	// allow concurrent block additions. It differs from the next lock in
	// that it's only for AddBlock method itself, the chain state is
	// protected by the lock below, but holding it during all of AddBlock
	// is too expensive (because the state only changes when persisting
	// change cache).
	addLock sync.Mutex

	// This lock ensures blockchain immutability for operations that need
	// that while performing their tasks. It's mostly used as a read lock
	// with the only writer being the block addition logic.
	lock sync.RWMutex

	// Data access object for CRUD operations around storage. It's write-cached.
	dao *dao.Simple

	// persistent is the same DB as dao, but we never write to it, so all reads
	// are directly from underlying persistent store.
	persistent *dao.Simple

	// Underlying persistent store.
	store storage.Store

	// Current index/height of the highest block.
	// Read access should always be called by BlockHeight().
	// Write access should only happen in storeBlock().
	blockHeight uint32

	// Current top Block wrapped in an atomic.Value for safe access.
	topBlock atomic.Value

	// Current persisted block count.
	persistedHeight uint32

	// Stop synchronization mechanisms.
	stopCh      chan struct{}
	runToExitCh chan struct{}
	// isRunning denotes whether blockchain routines are currently running.
	isRunning atomic.Value

	memPool *mempool.Pool

	// postBlock is a set of callback methods which should be run under the Blockchain lock after new block is persisted.
	// Block's transactions are passed via mempool.
	postBlock []func(func(*transaction.Transaction, *mempool.Pool, bool) bool, *mempool.Pool, *block.Block)

	log *zap.Logger

	lastBatch *storage.MemBatch

	contracts native.Contracts

	extensible atomic.Value

	// knownValidatorsCount is the latest known validators count used
	// for defaultBlockWitness.
	knownValidatorsCount atomic.Value
	// defaultBlockWitness stores transaction.Witness with m out of n multisig,
	// where n = knownValidatorsCount.
	defaultBlockWitness atomic.Value

	stateRoot *stateroot.Module

	// Notification subsystem.
	events  chan bcEvent
	subCh   chan any
	unsubCh chan any
}

// StateRoot represents local state root module.
type StateRoot interface {
	CurrentLocalHeight() uint32
	CurrentLocalStateRoot() util.Uint256
	CurrentValidatedHeight() uint32
	FindStates(root util.Uint256, prefix, start []byte, maxNum int) ([]storage.KeyValue, error)
	SeekStates(root util.Uint256, prefix []byte, f func(k, v []byte) bool)
	GetState(root util.Uint256, key []byte) ([]byte, error)
	GetStateProof(root util.Uint256, key []byte) ([][]byte, error)
	GetStateRoot(height uint32) (*state.MPTRoot, error)
	GetLatestStateHeight(root util.Uint256) (uint32, error)
}

// bcEvent is an internal event generated by the Blockchain and then
// broadcasted to other parties. It joins the new block and associated
// invocation logs, all the other events visible from outside can be produced
// from this combination.
type bcEvent struct {
	block          *block.Block
	appExecResults []*state.AppExecResult
}

// transferData is used for transfer caching during storeBlock.
type transferData struct {
	Info  state.TokenTransferInfo
	Log11 state.TokenTransferLog
	Log17 state.TokenTransferLog
}

// NewBlockchain returns a new blockchain object the will use the
// given Store as its underlying storage. For it to work correctly you need
// to spawn a goroutine for its Run method after this initialization.
func NewBlockchain(s storage.Store, cfg config.Blockchain, log *zap.Logger) (*Blockchain, error) {
	if log == nil {
		return nil, errors.New("empty logger")
	}

	// Protocol configuration fixups/checks.
	if cfg.InitialGASSupply <= 0 {
		cfg.InitialGASSupply = fixedn.Fixed8(DefaultInitialGAS)
		log.Info("initial gas supply is not set or wrong, setting default value", zap.Stringer("InitialGASSupply", cfg.InitialGASSupply))
	}
	if cfg.MemPoolSize <= 0 {
		cfg.MemPoolSize = defaultMemPoolSize
		log.Info("mempool size is not set or wrong, setting default value", zap.Int("MemPoolSize", cfg.MemPoolSize))
	}
	if cfg.P2PSigExtensions && cfg.P2PNotaryRequestPayloadPoolSize <= 0 {
		cfg.P2PNotaryRequestPayloadPoolSize = defaultP2PNotaryRequestPayloadPoolSize
		log.Info("P2PNotaryRequestPayloadPool size is not set or wrong, setting default value", zap.Int("P2PNotaryRequestPayloadPoolSize", cfg.P2PNotaryRequestPayloadPoolSize))
	}
	if cfg.MaxBlockSize == 0 {
		cfg.MaxBlockSize = defaultMaxBlockSize
		log.Info("MaxBlockSize is not set or wrong, setting default value", zap.Uint32("MaxBlockSize", cfg.MaxBlockSize))
	}
	if cfg.MaxBlockSystemFee <= 0 {
		cfg.MaxBlockSystemFee = defaultMaxBlockSystemFee
		log.Info("MaxBlockSystemFee is not set or wrong, setting default value", zap.Int64("MaxBlockSystemFee", cfg.MaxBlockSystemFee))
	}
	if cfg.MaxTraceableBlocks == 0 {
		cfg.MaxTraceableBlocks = defaultMaxTraceableBlocks
		log.Info("MaxTraceableBlocks is not set or wrong, using default value", zap.Uint32("MaxTraceableBlocks", cfg.MaxTraceableBlocks))
	}
	if cfg.MaxTransactionsPerBlock == 0 {
		cfg.MaxTransactionsPerBlock = defaultMaxTransactionsPerBlock
		log.Info("MaxTransactionsPerBlock is not set or wrong, using default value",
			zap.Uint16("MaxTransactionsPerBlock", cfg.MaxTransactionsPerBlock))
	}
	if cfg.TimePerBlock <= 0 {
		cfg.TimePerBlock = defaultTimePerBlock
		log.Info("TimePerBlock is not set or wrong, using default value",
			zap.Duration("TimePerBlock", cfg.TimePerBlock))
	}
	if cfg.MaxValidUntilBlockIncrement == 0 {
		const timePerDay = 24 * time.Hour

		cfg.MaxValidUntilBlockIncrement = uint32(timePerDay / cfg.TimePerBlock)
		log.Info("MaxValidUntilBlockIncrement is not set or wrong, using default value",
			zap.Uint32("MaxValidUntilBlockIncrement", cfg.MaxValidUntilBlockIncrement))
	}
	if cfg.P2PStateExchangeExtensions {
		if !cfg.StateRootInHeader {
			return nil, errors.New("P2PStatesExchangeExtensions are enabled, but StateRootInHeader is off")
		}
		if cfg.KeepOnlyLatestState && !cfg.RemoveUntraceableBlocks {
			return nil, errors.New("P2PStateExchangeExtensions can be enabled either on MPT-complete node (KeepOnlyLatestState=false) or on light GC-enabled node (RemoveUntraceableBlocks=true)")
		}
		if cfg.StateSyncInterval <= 0 {
			cfg.StateSyncInterval = defaultStateSyncInterval
			log.Info("StateSyncInterval is not set or wrong, using default value",
				zap.Int("StateSyncInterval", cfg.StateSyncInterval))
		}
	}
	if cfg.Hardforks == nil {
		cfg.Hardforks = map[string]uint32{}
		for _, hf := range config.Hardforks {
			cfg.Hardforks[hf.String()] = 0
		}
		log.Info("Hardforks are not set, using default value")
	} else if len(cfg.Hardforks) != 0 {
		// Explicitly set the height of all old omitted hardforks to 0 for proper
		// IsHardforkEnabled behaviour.
		for _, hf := range config.Hardforks {
			if _, ok := cfg.Hardforks[hf.String()]; !ok {
				cfg.Hardforks[hf.String()] = 0
				continue
			}
			break
		}
	}

	// Local config consistency checks.
	if cfg.Ledger.RemoveUntraceableBlocks && cfg.Ledger.GarbageCollectionPeriod == 0 {
		cfg.Ledger.GarbageCollectionPeriod = defaultGCPeriod
		log.Info("GarbageCollectionPeriod is not set or wrong, using default value", zap.Uint32("GarbageCollectionPeriod", cfg.Ledger.GarbageCollectionPeriod))
	}
	bc := &Blockchain{
		config:      cfg,
		dao:         dao.NewSimple(s, cfg.StateRootInHeader),
		persistent:  dao.NewSimple(s, cfg.StateRootInHeader),
		store:       s,
		stopCh:      make(chan struct{}),
		runToExitCh: make(chan struct{}),
		memPool:     mempool.New(cfg.MemPoolSize, 0, false, updateMempoolMetrics),
		log:         log,
		events:      make(chan bcEvent),
		subCh:       make(chan any),
		unsubCh:     make(chan any),
		contracts:   *native.NewContracts(cfg.ProtocolConfiguration),
	}

	bc.stateRoot = stateroot.NewModule(cfg, bc.VerifyWitness, bc.log, bc.dao.Store)
	bc.contracts.Designate.StateRootService = bc.stateRoot

	if err := bc.init(); err != nil {
		return nil, err
	}

	bc.isRunning.Store(false)
	return bc, nil
}

// GetDesignatedByRole returns a set of designated public keys for the given role
// relevant for the next block.
func (bc *Blockchain) GetDesignatedByRole(r noderoles.Role) (keys.PublicKeys, uint32, error) {
	// Retrieve designated nodes starting from the next block, because the current
	// block is already stored, thus, dependant services can't use PostPersist callback
	// to fetch relevant information at their start.
	res, h, err := bc.contracts.Designate.GetDesignatedByRole(bc.dao, r, bc.BlockHeight()+1)
	return res, h, err
}

// getCurrentHF returns the latest currently enabled hardfork. In case if no hardforks are enabled, the
// default config.Hardfork(0) value is returned.
func (bc *Blockchain) getCurrentHF() config.Hardfork {
	var (
		height  = bc.BlockHeight()
		current config.Hardfork
	)
	// Rely on the fact that hardforks list is continuous.
	for _, hf := range config.Hardforks {
		enableHeight, ok := bc.config.Hardforks[hf.String()]
		if !ok || height < enableHeight {
			break
		}
		current = hf
	}
	return current
}

// SetOracle sets oracle module. It can safely be called on the running blockchain.
// To unregister Oracle service use SetOracle(nil).
func (bc *Blockchain) SetOracle(mod native.OracleService) {
	orc := bc.contracts.Oracle
	currentHF := bc.getCurrentHF()
	if mod != nil {
		orcMd := orc.HFSpecificContractMD(&currentHF)
		md, ok := orcMd.GetMethod(manifest.MethodVerify, -1)
		if !ok {
			panic(fmt.Errorf("%s method not found", manifest.MethodVerify))
		}
		mod.UpdateNativeContract(orcMd.NEF.Script, orc.GetOracleResponseScript(),
			orc.Hash, md.MD.Offset)
		keys, _, err := bc.GetDesignatedByRole(noderoles.Oracle)
		if err != nil {
			bc.log.Error("failed to get oracle key list")
			return
		}
		mod.UpdateOracleNodes(keys)
		reqs, err := bc.contracts.Oracle.GetRequests(bc.dao)
		if err != nil {
			bc.log.Error("failed to get current oracle request list")
			return
		}
		mod.AddRequests(reqs)
	}
	orc.Module.Store(&mod)
	bc.contracts.Designate.OracleService.Store(&mod)
}

// SetNotary sets notary module. It may safely be called on the running blockchain.
// To unregister Notary service use SetNotary(nil).
func (bc *Blockchain) SetNotary(mod native.NotaryService) {
	if mod != nil {
		keys, _, err := bc.GetDesignatedByRole(noderoles.P2PNotary)
		if err != nil {
			bc.log.Error("failed to get notary key list")
			return
		}
		mod.UpdateNotaryNodes(keys)
	}
	bc.contracts.Designate.NotaryService.Store(&mod)
}

func (bc *Blockchain) init() error {
	// If we could not find the version in the Store, we know that there is nothing stored.
	ver, err := bc.dao.GetVersion()
	if err != nil {
		bc.log.Info("no storage version found! creating genesis block")
		ver = dao.Version{
			StoragePrefix:              storage.STStorage,
			StateRootInHeader:          bc.config.StateRootInHeader,
			P2PSigExtensions:           bc.config.P2PSigExtensions,
			P2PStateExchangeExtensions: bc.config.P2PStateExchangeExtensions,
			KeepOnlyLatestState:        bc.config.Ledger.KeepOnlyLatestState,
			Magic:                      uint32(bc.config.Magic),
			Value:                      version,
		}
		bc.dao.PutVersion(ver)
		bc.dao.Version = ver
		bc.persistent.Version = ver
		genesisBlock, err := CreateGenesisBlock(bc.config.ProtocolConfiguration)
		if err != nil {
			return err
		}
		bc.HeaderHashes.initGenesis(bc.dao, genesisBlock.Hash())
		if err := bc.stateRoot.Init(0); err != nil {
			return fmt.Errorf("can't init MPT: %w", err)
		}
		return bc.storeBlock(genesisBlock, nil)
	}
	if ver.Value != version {
		return fmt.Errorf("storage version mismatch (expected=%s, actual=%s)", version, ver.Value)
	}
	if ver.StateRootInHeader != bc.config.StateRootInHeader {
		return fmt.Errorf("StateRootInHeader setting mismatch (config=%t, db=%t)",
			bc.config.StateRootInHeader, ver.StateRootInHeader)
	}
	if ver.P2PSigExtensions != bc.config.P2PSigExtensions {
		return fmt.Errorf("P2PSigExtensions setting mismatch (old=%t, new=%t)",
			ver.P2PSigExtensions, bc.config.P2PSigExtensions)
	}
	if ver.P2PStateExchangeExtensions != bc.config.P2PStateExchangeExtensions {
		return fmt.Errorf("P2PStateExchangeExtensions setting mismatch (old=%t, new=%t)",
			ver.P2PStateExchangeExtensions, bc.config.P2PStateExchangeExtensions)
	}
	if ver.KeepOnlyLatestState != bc.config.Ledger.KeepOnlyLatestState {
		return fmt.Errorf("KeepOnlyLatestState setting mismatch (old=%v, new=%v)",
			ver.KeepOnlyLatestState, bc.config.Ledger.KeepOnlyLatestState)
	}
	if ver.Magic != uint32(bc.config.Magic) {
		return fmt.Errorf("protocol configuration Magic mismatch (old=%v, new=%v)",
			ver.Magic, bc.config.Magic)
	}
	bc.dao.Version = ver
	bc.persistent.Version = ver

	// At this point there was no version found in the storage which
	// implies a creating fresh storage with the version specified
	// and the genesis block as first block.
	bc.log.Info("restoring blockchain", zap.String("version", version))

	err = bc.HeaderHashes.init(bc.dao)
	if err != nil {
		return err
	}

	// Check whether StateChangeState stage is in the storage and continue interrupted state jump / state reset if so.
	stateChStage, err := bc.dao.Store.Get([]byte{byte(storage.SYSStateChangeStage)})
	if err == nil {
		if len(stateChStage) != 1 {
			return fmt.Errorf("invalid state jump stage format")
		}
		// State jump / state reset wasn't finished yet, thus continue it.
		stateSyncPoint, err := bc.dao.GetStateSyncPoint()
		if err != nil {
			return fmt.Errorf("failed to get state sync point from the storage")
		}
		if (stateChStage[0] & stateResetBit) != 0 {
			return bc.resetStateInternal(stateSyncPoint, stateChangeStage(stateChStage[0]&(^stateResetBit)))
		}
		if !(bc.config.P2PStateExchangeExtensions && bc.config.Ledger.RemoveUntraceableBlocks) {
			return errors.New("state jump was not completed, but P2PStateExchangeExtensions are disabled or archival node capability is on. " +
				"To start an archival node drop the database manually and restart the node")
		}
		return bc.jumpToStateInternal(stateSyncPoint, stateChangeStage(stateChStage[0]))
	}

	bHeight, err := bc.dao.GetCurrentBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to retrieve current block height: %w", err)
	}
	bc.blockHeight = bHeight
	bc.persistedHeight = bHeight

	bc.log.Debug("initializing caches", zap.Uint32("blockHeight", bHeight))
	if err = bc.stateRoot.Init(bHeight); err != nil {
		return fmt.Errorf("can't init MPT at height %d: %w", bHeight, err)
	}

	err = bc.initializeNativeCache(bc.blockHeight, bc.dao)
	if err != nil {
		return fmt.Errorf("can't init natives cache: %w", err)
	}

	// Check autogenerated native contracts' manifests and NEFs against the stored ones.
	// Need to be done after native Management cache initialization to be able to get
	// contract state from DAO via high-level bc API.
	var current = bc.getCurrentHF()
	for _, c := range bc.contracts.Contracts {
		md := c.Metadata()
		storedCS := bc.GetContractState(md.Hash)
		// Check that contract was deployed.
		if !bc.isHardforkEnabled(c.ActiveIn(), bHeight) {
			if storedCS != nil {
				return fmt.Errorf("native contract %s is already stored, but marked as inactive for height %d in config", md.Name, bHeight)
			}
			continue
		}
		if storedCS == nil {
			return fmt.Errorf("native contract %s is not stored, but should be active at height %d according to config", md.Name, bHeight)
		}
		storedCSBytes, err := stackitem.SerializeConvertible(storedCS)
		if err != nil {
			return fmt.Errorf("failed to check native %s state against autogenerated one: %w", md.Name, err)
		}
		hfMD := md.HFSpecificContractMD(&current)
		autogenCS := &state.Contract{
			ContractBase:  hfMD.ContractBase,
			UpdateCounter: storedCS.UpdateCounter, // it can be restored only from the DB, so use the stored value.
		}
		autogenCSBytes, err := stackitem.SerializeConvertible(autogenCS)
		if err != nil {
			return fmt.Errorf("failed to check native %s state against autogenerated one: %w", md.Name, err)
		}
		if !bytes.Equal(storedCSBytes, autogenCSBytes) {
			storedJ, _ := json.Marshal(storedCS)
			autogenJ, _ := json.Marshal(autogenCS)
			return fmt.Errorf("native %s: version mismatch for the latest hardfork %s (stored contract state differs from autogenerated one), "+
				"try to resynchronize the node from the genesis: %s vs %s", md.Name, current, string(storedJ), string(autogenJ))
		}
	}

	updateBlockHeightMetric(bHeight)
	updatePersistedHeightMetric(bHeight)
	updateHeaderHeightMetric(bc.HeaderHeight())

	return bc.updateExtensibleWhitelist(bHeight)
}

// jumpToState is an atomic operation that changes Blockchain state to the one
// specified by the state sync point p. All the data needed for the jump must be
// collected by the state sync module.
func (bc *Blockchain) jumpToState(p uint32) error {
	bc.addLock.Lock()
	bc.lock.Lock()
	defer bc.lock.Unlock()
	defer bc.addLock.Unlock()

	return bc.jumpToStateInternal(p, none)
}

// jumpToStateInternal is an internal representation of jumpToState callback that
// changes Blockchain state to the one specified by state sync point p and state
// jump stage. All the data needed for the jump must be in the DB, otherwise an
// error is returned. It is not protected by mutex.
func (bc *Blockchain) jumpToStateInternal(p uint32, stage stateChangeStage) error {
	if p >= bc.HeaderHeight() {
		return fmt.Errorf("invalid state sync point %d: headerHeignt is %d", p, bc.HeaderHeight())
	}

	bc.log.Info("jumping to state sync point", zap.Uint32("state sync point", p))

	jumpStageKey := []byte{byte(storage.SYSStateChangeStage)}
	switch stage {
	case none:
		bc.dao.Store.Put(jumpStageKey, []byte{byte(stateJumpStarted)})
		fallthrough
	case stateJumpStarted:
		newPrefix := statesync.TemporaryPrefix(bc.dao.Version.StoragePrefix)
		v, err := bc.dao.GetVersion()
		if err != nil {
			return fmt.Errorf("failed to get dao.Version: %w", err)
		}
		v.StoragePrefix = newPrefix
		bc.dao.PutVersion(v)
		bc.persistent.Version = v

		bc.dao.Store.Put(jumpStageKey, []byte{byte(newStorageItemsAdded)})

		fallthrough
	case newStorageItemsAdded:
		cache := bc.dao.GetPrivate()
		prefix := statesync.TemporaryPrefix(bc.dao.Version.StoragePrefix)
		bc.dao.Store.Seek(storage.SeekRange{Prefix: []byte{byte(prefix)}}, func(k, _ []byte) bool {
			// #1468, but don't need to copy here, because it is done by Store.
			cache.Store.Delete(k)
			return true
		})

		// After current state is updated, we need to remove outdated state-related data if so.
		// The only outdated data we might have is genesis-related data, so check it.
		if p-bc.config.MaxTraceableBlocks > 0 {
			err := cache.DeleteBlock(bc.GetHeaderHash(0))
			if err != nil {
				return fmt.Errorf("failed to remove outdated state data for the genesis block: %w", err)
			}
			prefixes := []byte{byte(storage.STNEP11Transfers), byte(storage.STNEP17Transfers), byte(storage.STTokenTransferInfo)}
			for i := range prefixes {
				cache.Store.Seek(storage.SeekRange{Prefix: prefixes[i : i+1]}, func(k, v []byte) bool {
					cache.Store.Delete(k)
					return true
				})
			}
		}
		// Update SYS-prefixed info.
		block, err := bc.dao.GetBlock(bc.GetHeaderHash(p))
		if err != nil {
			return fmt.Errorf("failed to get current block: %w", err)
		}
		cache.StoreAsCurrentBlock(block)
		cache.Store.Put(jumpStageKey, []byte{byte(staleBlocksRemoved)})
		_, err = cache.Persist()
		if err != nil {
			return fmt.Errorf("failed to persist old items removal: %w", err)
		}
	case staleBlocksRemoved:
		// there's nothing to do after that, so just continue with common operations
		// and remove state jump stage in the end.
	default:
		return fmt.Errorf("unknown state jump stage: %d", stage)
	}
	block, err := bc.dao.GetBlock(bc.GetHeaderHash(p + 1))
	if err != nil {
		return fmt.Errorf("failed to get block to init MPT: %w", err)
	}
	bc.stateRoot.JumpToState(&state.MPTRoot{
		Index: p,
		Root:  block.PrevStateRoot,
	})

	bc.dao.Store.Delete(jumpStageKey)

	err = bc.resetRAMState(p, false)
	if err != nil {
		return fmt.Errorf("failed to update in-memory blockchain data: %w", err)
	}
	return nil
}

// resetRAMState resets in-memory cached info.
func (bc *Blockchain) resetRAMState(height uint32, resetHeaders bool) error {
	if resetHeaders {
		err := bc.HeaderHashes.init(bc.dao)
		if err != nil {
			return err
		}
	}
	block, err := bc.dao.GetBlock(bc.GetHeaderHash(height))
	if err != nil {
		return fmt.Errorf("failed to get current block: %w", err)
	}
	bc.topBlock.Store(block)
	atomic.StoreUint32(&bc.blockHeight, height)
	atomic.StoreUint32(&bc.persistedHeight, height)

	err = bc.initializeNativeCache(block.Index, bc.dao)
	if err != nil {
		return fmt.Errorf("failed to initialize natives cache: %w", err)
	}

	if err := bc.updateExtensibleWhitelist(height); err != nil {
		return fmt.Errorf("failed to update extensible whitelist: %w", err)
	}

	updateBlockHeightMetric(height)
	updatePersistedHeightMetric(height)
	updateHeaderHeightMetric(bc.HeaderHeight())
	return nil
}

// Reset resets chain state to the specified height if possible. This method
// performs direct DB changes and can be called on non-running Blockchain only.
func (bc *Blockchain) Reset(height uint32) error {
	if bc.isRunning.Load().(bool) {
		return errors.New("can't reset state of the running blockchain")
	}
	bc.dao.PutStateSyncPoint(height)
	return bc.resetStateInternal(height, none)
}

func (bc *Blockchain) resetStateInternal(height uint32, stage stateChangeStage) error {
	// Cache isn't yet initialized, so retrieve block height right from DAO.
	currHeight, err := bc.dao.GetCurrentBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to retrieve current block height: %w", err)
	}
	// Headers are already initialized by this moment, thus may use chain's API.
	hHeight := bc.HeaderHeight()
	// State reset may already be started by this moment, so perform these checks only if it wasn't.
	if stage == none {
		if height > currHeight {
			return fmt.Errorf("current block height is %d, can't reset state to height %d", currHeight, height)
		}
		if height == currHeight && hHeight == currHeight {
			bc.log.Info("chain is at the proper state", zap.Uint32("height", height))
			return nil
		}
		if bc.config.Ledger.KeepOnlyLatestState {
			return fmt.Errorf("KeepOnlyLatestState is enabled, state for height %d is outdated and removed from the storage", height)
		}
		if bc.config.Ledger.RemoveUntraceableBlocks && currHeight >= bc.config.MaxTraceableBlocks {
			return fmt.Errorf("RemoveUntraceableBlocks is enabled, a necessary batch of traceable blocks has already been removed")
		}
	}

	// Retrieve necessary state before the DB modification.
	b, err := bc.GetBlock(bc.GetHeaderHash(height))
	if err != nil {
		return fmt.Errorf("failed to retrieve block %d: %w", height, err)
	}
	sr, err := bc.stateRoot.GetStateRoot(height)
	if err != nil {
		return fmt.Errorf("failed to retrieve stateroot for height %d: %w", height, err)
	}
	v := bc.dao.Version
	// dao is MemCachedStore over DB, we use dao directly to persist cached changes
	// right to the underlying DB.
	cache := bc.dao
	// upperCache is a private MemCachedStore over cache. During each of the state
	// sync stages we put the data inside the upperCache; in the end of each stage
	// we persist changes from upperCache to cache. Changes from cache are persisted
	// directly to the underlying persistent storage (boltDB, levelDB, etc.).
	// upperCache/cache segregation is needed to keep the DB state clean and to
	// persist data from different stages separately.
	upperCache := cache.GetPrivate()

	bc.log.Info("initializing state reset", zap.Uint32("target height", height))
	start := time.Now()
	p := start

	// Start batch persisting routine, it will be used for blocks/txs/AERs/storage items batches persist.
	type postPersist func(persistedKeys int, err error) error
	var (
		persistCh       = make(chan postPersist)
		persistToExitCh = make(chan struct{})
	)
	go func() {
		for {
			f, ok := <-persistCh
			if !ok {
				break
			}
			persistErr := f(cache.Persist())
			if persistErr != nil {
				bc.log.Fatal("persist failed", zap.Error(persistErr))
				panic(persistErr)
			}
		}
		close(persistToExitCh)
	}()
	defer func() {
		close(persistCh)
		<-persistToExitCh
		bc.log.Info("reset finished successfully", zap.Duration("took", time.Since(start)))
	}()

	resetStageKey := []byte{byte(storage.SYSStateChangeStage)}
	switch stage {
	case none:
		upperCache.Store.Put(resetStageKey, []byte{stateResetBit | byte(stateJumpStarted)})
		// Technically, there's no difference between Persist() and PersistSync() for the private
		// MemCached storage, but we'd better use the sync version in case of some further code changes.
		_, uerr := upperCache.PersistSync()
		if uerr != nil {
			panic(uerr)
		}
		upperCache = cache.GetPrivate()
		persistCh <- func(persistedKeys int, err error) error {
			if err != nil {
				return fmt.Errorf("failed to persist state reset start marker to the DB: %w", err)
			}
			return nil
		}
		fallthrough
	case stateJumpStarted:
		bc.log.Debug("trying to reset blocks, transactions and AERs")
		// Remove blocks/transactions/aers from currHeight down to height (not including height itself).
		// Keep headers for now, they'll be removed later. It's hard to handle the whole set of changes in
		// one stage, so persist periodically.
		const persistBatchSize = 100 * headerBatchCount // count blocks only, should be enough to avoid OOM killer even for large blocks
		var (
			pBlocksStart        = p
			blocksCnt, batchCnt int
			keysCnt             = new(int)
		)
		for i := height + 1; i <= currHeight; i++ {
			err := upperCache.DeleteBlock(bc.GetHeaderHash(i))
			if err != nil {
				return fmt.Errorf("error while removing block %d: %w", i, err)
			}
			blocksCnt++
			if blocksCnt == persistBatchSize {
				blocksCnt = 0
				batchCnt++
				bc.log.Info("intermediate batch of removed blocks, transactions and AERs is collected",
					zap.Int("batch", batchCnt),
					zap.Duration("took", time.Since(p)))

				persistStart := time.Now()
				persistBatch := batchCnt
				_, uerr := upperCache.PersistSync()
				if uerr != nil {
					panic(uerr)
				}
				upperCache = cache.GetPrivate()
				persistCh <- func(persistedKeys int, err error) error {
					if err != nil {
						return fmt.Errorf("failed to persist intermediate batch of removed blocks, transactions and AERs: %w", err)
					}
					*keysCnt += persistedKeys
					bc.log.Debug("intermediate batch of removed blocks, transactions and AERs is persisted",
						zap.Int("batch", persistBatch),
						zap.Duration("took", time.Since(persistStart)),
						zap.Int("keys", persistedKeys))
					return nil
				}
				p = time.Now()
			}
		}
		upperCache.Store.Put(resetStageKey, []byte{stateResetBit | byte(staleBlocksRemoved)})
		batchCnt++
		bc.log.Info("last batch of removed blocks, transactions and AERs is collected",
			zap.Int("batch", batchCnt),
			zap.Duration("took", time.Since(p)))
		bc.log.Info("blocks, transactions ans AERs are reset", zap.Duration("took", time.Since(pBlocksStart)))

		persistStart := time.Now()
		persistBatch := batchCnt
		_, uerr := upperCache.PersistSync()
		if uerr != nil {
			panic(uerr)
		}
		upperCache = cache.GetPrivate()
		persistCh <- func(persistedKeys int, err error) error {
			if err != nil {
				return fmt.Errorf("failed to persist last batch of removed blocks, transactions ans AERs: %w", err)
			}
			*keysCnt += persistedKeys
			bc.log.Debug("last batch of removed blocks, transactions and AERs is persisted",
				zap.Int("batch", persistBatch),
				zap.Duration("took", time.Since(persistStart)),
				zap.Int("keys", persistedKeys))
			return nil
		}
		p = time.Now()
		fallthrough
	case staleBlocksRemoved:
		// Completely remove contract IDs to update them later.
		bc.log.Debug("trying to reset contract storage items")
		pStorageStart := p

		p = time.Now()
		var mode = mpt.ModeAll
		if bc.config.Ledger.RemoveUntraceableBlocks {
			mode |= mpt.ModeGCFlag
		}
		trieStore := mpt.NewTrieStore(sr.Root, mode, upperCache.Store)
		oldStoragePrefix := v.StoragePrefix
		newStoragePrefix := statesync.TemporaryPrefix(oldStoragePrefix)

		const persistBatchSize = 200000
		var cnt, storageItmsCnt, batchCnt int
		trieStore.Seek(storage.SeekRange{Prefix: []byte{byte(oldStoragePrefix)}}, func(k, v []byte) bool {
			if cnt >= persistBatchSize {
				cnt = 0
				batchCnt++
				bc.log.Info("intermediate batch of contract storage items and IDs is collected",
					zap.Int("batch", batchCnt),
					zap.Duration("took", time.Since(p)))

				persistStart := time.Now()
				persistBatch := batchCnt
				_, uerr := upperCache.PersistSync()
				if uerr != nil {
					panic(uerr)
				}
				upperCache = cache.GetPrivate()
				persistCh <- func(persistedKeys int, err error) error {
					if err != nil {
						return fmt.Errorf("failed to persist intermediate batch of contract storage items: %w", err)
					}
					bc.log.Debug("intermediate batch of contract storage items is persisted",
						zap.Int("batch", persistBatch),
						zap.Duration("took", time.Since(persistStart)),
						zap.Int("keys", persistedKeys))
					return nil
				}
				p = time.Now()
			}
			// May safely omit KV copying.
			k[0] = byte(newStoragePrefix)
			upperCache.Store.Put(k, v)
			cnt++
			storageItmsCnt++

			return true
		})
		trieStore.Close()

		upperCache.Store.Put(resetStageKey, []byte{stateResetBit | byte(newStorageItemsAdded)})
		batchCnt++
		persistBatch := batchCnt
		bc.log.Info("last batch of contract storage items is collected", zap.Int("batch", batchCnt), zap.Duration("took", time.Since(p)))
		bc.log.Info("contract storage items are reset", zap.Duration("took", time.Since(pStorageStart)),
			zap.Int("keys", storageItmsCnt))

		lastStart := time.Now()
		_, uerr := upperCache.PersistSync()
		if uerr != nil {
			panic(uerr)
		}
		upperCache = cache.GetPrivate()
		persistCh <- func(persistedKeys int, err error) error {
			if err != nil {
				return fmt.Errorf("failed to persist contract storage items and IDs changes to the DB: %w", err)
			}
			bc.log.Debug("last batch of contract storage items and IDs is persisted", zap.Int("batch", persistBatch), zap.Duration("took", time.Since(lastStart)), zap.Int("keys", persistedKeys))
			return nil
		}
		p = time.Now()
		fallthrough
	case newStorageItemsAdded:
		// Reset SYS-prefixed and IX-prefixed information.
		bc.log.Debug("trying to reset headers information")
		for i := height + 1; i <= hHeight; i++ {
			upperCache.PurgeHeader(bc.GetHeaderHash(i))
		}
		upperCache.DeleteHeaderHashes(height+1, headerBatchCount)
		upperCache.StoreAsCurrentBlock(b)
		upperCache.PutCurrentHeader(b.Hash(), height)
		v.StoragePrefix = statesync.TemporaryPrefix(v.StoragePrefix)
		upperCache.PutVersion(v)
		// It's important to manually change the cache's Version at this stage, so that native cache
		// can be properly initialized (with the correct contract storage data prefix) at the final
		// stage of the state reset. At the same time, DB's SYSVersion-prefixed data will be persisted
		// from upperCache to cache in a standard way (several lines below).
		cache.Version = v
		bc.persistent.Version = v

		upperCache.Store.Put(resetStageKey, []byte{stateResetBit | byte(headersReset)})
		bc.log.Info("headers information is reset", zap.Duration("took", time.Since(p)))

		persistStart := time.Now()
		_, uerr := upperCache.PersistSync()
		if uerr != nil {
			panic(uerr)
		}
		upperCache = cache.GetPrivate()
		persistCh <- func(persistedKeys int, err error) error {
			if err != nil {
				return fmt.Errorf("failed to persist headers changes to the DB: %w", err)
			}
			bc.log.Debug("headers information is persisted", zap.Duration("took", time.Since(persistStart)), zap.Int("keys", persistedKeys))
			return nil
		}
		p = time.Now()
		fallthrough
	case headersReset:
		// Reset MPT.
		bc.log.Debug("trying to reset state root information and NEP transfers")
		err = bc.stateRoot.ResetState(height, upperCache.Store)
		if err != nil {
			return fmt.Errorf("failed to rollback MPT state: %w", err)
		}

		// Reset transfers.
		err = bc.resetTransfers(upperCache, height)
		if err != nil {
			return fmt.Errorf("failed to strip transfer log / transfer info: %w", err)
		}

		upperCache.Store.Put(resetStageKey, []byte{stateResetBit | byte(transfersReset)})
		bc.log.Info("state root information and NEP transfers are reset", zap.Duration("took", time.Since(p)))

		persistStart := time.Now()
		_, uerr := upperCache.PersistSync()
		if uerr != nil {
			panic(uerr)
		}
		upperCache = cache.GetPrivate()
		persistCh <- func(persistedKeys int, err error) error {
			if err != nil {
				return fmt.Errorf("failed to persist contract storage items changes to the DB: %w", err)
			}

			bc.log.Debug("state root information and NEP transfers are persisted", zap.Duration("took", time.Since(persistStart)), zap.Int("keys", persistedKeys))
			return nil
		}
		p = time.Now()
		fallthrough
	case transfersReset:
		// there's nothing to do after that, so just continue with common operations
		// and remove state reset stage in the end.
	default:
		return fmt.Errorf("unknown state reset stage: %d", stage)
	}

	// Direct (cache-less) DB operation:  remove stale storage items.
	bc.log.Debug("trying to remove stale storage items")
	keys := 0
	err = bc.store.SeekGC(storage.SeekRange{
		Prefix: []byte{byte(statesync.TemporaryPrefix(v.StoragePrefix))},
	}, func(_, _ []byte) bool {
		keys++
		return false
	})
	if err != nil {
		return fmt.Errorf("faield to remove stale storage items from DB: %w", err)
	}
	bc.log.Info("stale storage items are reset", zap.Duration("took", time.Since(p)), zap.Int("keys", keys))
	p = time.Now()

	bc.log.Debug("trying to remove state reset point")
	upperCache.Store.Delete(resetStageKey)
	// Unlike the state jump, state sync point must be removed as we have complete state for this height.
	upperCache.Store.Delete([]byte{byte(storage.SYSStateSyncPoint)})
	bc.log.Info("state reset point is removed", zap.Duration("took", time.Since(p)))

	persistStart := time.Now()
	_, uerr := upperCache.PersistSync()
	if uerr != nil {
		panic(uerr)
	}
	persistCh <- func(persistedKeys int, err error) error {
		if err != nil {
			return fmt.Errorf("failed to persist state reset stage to DAO: %w", err)
		}
		bc.log.Info("state reset point information is persisted", zap.Duration("took", time.Since(persistStart)), zap.Int("keys", persistedKeys))
		return nil
	}
	p = time.Now()

	err = bc.resetRAMState(height, true)
	if err != nil {
		return fmt.Errorf("failed to update in-memory blockchain data: %w", err)
	}
	return nil
}

func (bc *Blockchain) initializeNativeCache(blockHeight uint32, d *dao.Simple) error {
	for _, c := range bc.contracts.Contracts {
		// Check that contract was deployed.
		if !bc.isHardforkEnabled(c.ActiveIn(), blockHeight) {
			continue
		}
		err := c.InitializeCache(blockHeight, d)
		if err != nil {
			return fmt.Errorf("failed to initialize cache for %s: %w", c.Metadata().Name, err)
		}
	}
	return nil
}

// isHardforkEnabled returns true if the specified hardfork is enabled at the
// given height. nil hardfork is treated as always enabled.
func (bc *Blockchain) isHardforkEnabled(hf *config.Hardfork, blockHeight uint32) bool {
	hfs := bc.config.Hardforks
	if hf != nil {
		start, ok := hfs[hf.String()]
		if !ok || start < blockHeight {
			return false
		}
	}
	return true
}

// Run runs chain loop, it needs to be run as goroutine and executing it is
// critical for correct Blockchain operation.
func (bc *Blockchain) Run() {
	bc.isRunning.Store(true)
	persistTimer := time.NewTimer(persistInterval)
	defer func() {
		persistTimer.Stop()
		if _, err := bc.persist(true); err != nil {
			bc.log.Warn("failed to persist", zap.Error(err))
		}
		if err := bc.dao.Store.Close(); err != nil {
			bc.log.Warn("failed to close db", zap.Error(err))
		}
		bc.isRunning.Store(false)
		close(bc.runToExitCh)
	}()
	go bc.notificationDispatcher()
	var nextSync bool
	for {
		select {
		case <-bc.stopCh:
			return
		case <-persistTimer.C:
			var oldPersisted uint32
			var gcDur time.Duration

			if bc.config.Ledger.RemoveUntraceableBlocks {
				oldPersisted = atomic.LoadUint32(&bc.persistedHeight)
			}
			dur, err := bc.persist(nextSync)
			if err != nil {
				bc.log.Warn("failed to persist blockchain", zap.Error(err))
			}
			if bc.config.Ledger.RemoveUntraceableBlocks {
				gcDur = bc.tryRunGC(oldPersisted)
			}
			nextSync = dur > persistInterval*2
			interval := persistInterval - dur - gcDur
			interval = max(interval, time.Microsecond) // Reset doesn't work with zero or negative value.
			persistTimer.Reset(interval)
		}
	}
}

func (bc *Blockchain) tryRunGC(oldHeight uint32) time.Duration {
	var dur time.Duration

	newHeight := atomic.LoadUint32(&bc.persistedHeight)
	var tgtBlock = int64(newHeight)

	tgtBlock -= int64(bc.config.MaxTraceableBlocks)
	if bc.config.P2PStateExchangeExtensions {
		syncP := newHeight / uint32(bc.config.StateSyncInterval)
		syncP--
		syncP *= uint32(bc.config.StateSyncInterval)
		tgtBlock = min(tgtBlock, int64(syncP))
	}
	// Always round to the GCP.
	tgtBlock /= int64(bc.config.Ledger.GarbageCollectionPeriod)
	tgtBlock *= int64(bc.config.Ledger.GarbageCollectionPeriod)
	// Count periods.
	oldHeight /= bc.config.Ledger.GarbageCollectionPeriod
	newHeight /= bc.config.Ledger.GarbageCollectionPeriod
	if tgtBlock > int64(bc.config.Ledger.GarbageCollectionPeriod) && newHeight != oldHeight {
		tgtBlock /= int64(bc.config.Ledger.GarbageCollectionPeriod)
		tgtBlock *= int64(bc.config.Ledger.GarbageCollectionPeriod)
		dur = bc.stateRoot.GC(uint32(tgtBlock), bc.store)
		dur += bc.removeOldTransfers(uint32(tgtBlock))
	}
	return dur
}

// resetTransfers is a helper function that strips the top newest NEP17 and NEP11 transfer logs
// down to the given height (not including the height itself) and updates corresponding token
// transfer info.
func (bc *Blockchain) resetTransfers(cache *dao.Simple, height uint32) error {
	// Completely remove transfer info, updating it takes too much effort. We'll gather new
	// transfer info on-the-fly later.
	cache.Store.Seek(storage.SeekRange{
		Prefix: []byte{byte(storage.STTokenTransferInfo)},
	}, func(k, v []byte) bool {
		cache.Store.Delete(k)
		return true
	})

	// Look inside each transfer batch and iterate over the batch transfers, picking those that
	// not newer than the given height. Also, for each suitable transfer update transfer info
	// flushing changes after complete account's transfers processing.
	prefixes := []byte{byte(storage.STNEP11Transfers), byte(storage.STNEP17Transfers)}
	for i := range prefixes {
		var (
			acc             util.Uint160
			trInfo          *state.TokenTransferInfo
			removeFollowing bool
			seekErr         error
		)

		cache.Store.Seek(storage.SeekRange{
			Prefix:    prefixes[i : i+1],
			Backwards: false, // From oldest to newest batch.
		}, func(k, v []byte) bool {
			var batchAcc util.Uint160
			copy(batchAcc[:], k[1:])

			if batchAcc != acc { // Some new account we're iterating over.
				if trInfo != nil {
					seekErr = cache.PutTokenTransferInfo(acc, trInfo)
					if seekErr != nil {
						return false
					}
				}
				acc = batchAcc
				trInfo = nil
				removeFollowing = false
			} else if removeFollowing {
				cache.Store.Delete(bytes.Clone(k))
				return seekErr == nil
			}

			r := io.NewBinReaderFromBuf(v[1:])
			l := len(v)
			bytesRead := 1 // 1 is for batch size byte which is read by default.
			var (
				oldBatchSize = v[0]
				newBatchSize byte
			)
			for i := byte(0); i < v[0]; i++ { // From oldest to newest transfer of the batch.
				var t *state.NEP17Transfer
				if k[0] == byte(storage.STNEP11Transfers) {
					tr := new(state.NEP11Transfer)
					tr.DecodeBinary(r)
					t = &tr.NEP17Transfer
				} else {
					t = new(state.NEP17Transfer)
					t.DecodeBinary(r)
				}
				if r.Err != nil {
					seekErr = fmt.Errorf("failed to decode subsequent transfer: %w", r.Err)
					break
				}

				if t.Block > height {
					break
				}
				bytesRead = l - r.Len() // Including batch size byte.
				newBatchSize++
				if trInfo == nil {
					var err error
					trInfo, err = cache.GetTokenTransferInfo(batchAcc)
					if err != nil {
						seekErr = fmt.Errorf("failed to retrieve token transfer info for %s: %w", batchAcc.StringLE(), r.Err)
						return false
					}
				}
				appendTokenTransferInfo(trInfo, t.Asset, t.Block, t.Timestamp, k[0] == byte(storage.STNEP11Transfers), newBatchSize >= state.TokenTransferBatchSize)
			}
			if newBatchSize == oldBatchSize {
				// The batch is already in storage and doesn't need to be changed.
				return seekErr == nil
			}
			if newBatchSize > 0 {
				v[0] = newBatchSize
				cache.Store.Put(k, v[:bytesRead])
			} else {
				cache.Store.Delete(k)
				removeFollowing = true
			}
			return seekErr == nil
		})
		if seekErr != nil {
			return seekErr
		}
		if trInfo != nil {
			// Flush the last batch of transfer info changes.
			err := cache.PutTokenTransferInfo(acc, trInfo)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// appendTokenTransferInfo is a helper for resetTransfers that updates token transfer info
// wrt the given transfer that was added to the subsequent transfer batch.
func appendTokenTransferInfo(transferData *state.TokenTransferInfo,
	token int32, bIndex uint32, bTimestamp uint64, isNEP11 bool, lastTransferInBatch bool) {
	var (
		newBatch      *bool
		nextBatch     *uint32
		currTimestamp *uint64
	)
	if !isNEP11 {
		newBatch = &transferData.NewNEP17Batch
		nextBatch = &transferData.NextNEP17Batch
		currTimestamp = &transferData.NextNEP17NewestTimestamp
	} else {
		newBatch = &transferData.NewNEP11Batch
		nextBatch = &transferData.NextNEP11Batch
		currTimestamp = &transferData.NextNEP11NewestTimestamp
	}
	transferData.LastUpdated[token] = bIndex
	*newBatch = lastTransferInBatch
	if *newBatch {
		*nextBatch++
		*currTimestamp = bTimestamp
	}
}

func (bc *Blockchain) removeOldTransfers(index uint32) time.Duration {
	bc.log.Info("starting transfer data garbage collection", zap.Uint32("index", index))
	start := time.Now()
	h, err := bc.GetHeader(bc.GetHeaderHash(index))
	if err != nil {
		dur := time.Since(start)
		bc.log.Error("failed to find block header for transfer GC", zap.Duration("time", dur), zap.Error(err))
		return dur
	}
	var removed, kept int64
	var ts = h.Timestamp
	prefixes := []byte{byte(storage.STNEP11Transfers), byte(storage.STNEP17Transfers)}

	for i := range prefixes {
		var acc util.Uint160
		var canDrop bool

		err = bc.store.SeekGC(storage.SeekRange{
			Prefix:    prefixes[i : i+1],
			Backwards: true, // From new to old.
		}, func(k, v []byte) bool {
			// We don't look inside of the batches, it requires too much effort, instead
			// we drop batches that are confirmed to contain outdated entries.
			var batchAcc util.Uint160
			var batchTs = binary.BigEndian.Uint64(k[1+util.Uint160Size:])
			copy(batchAcc[:], k[1:])

			if batchAcc != acc { // Some new account we're iterating over.
				acc = batchAcc
			} else if canDrop { // We've seen this account and all entries in this batch are guaranteed to be outdated.
				removed++
				return false
			}
			// We don't know what's inside, so keep the current
			// batch anyway, but allow to drop older ones.
			canDrop = batchTs <= ts
			kept++
			return true
		})
		if err != nil {
			break
		}
	}
	dur := time.Since(start)
	if err != nil {
		bc.log.Error("failed to flush transfer data GC changeset", zap.Duration("time", dur), zap.Error(err))
	} else {
		bc.log.Info("finished transfer data garbage collection",
			zap.Int64("removed", removed),
			zap.Int64("kept", kept),
			zap.Duration("time", dur))
	}
	return dur
}

// notificationDispatcher manages subscription to events and broadcasts new events.
func (bc *Blockchain) notificationDispatcher() {
	var (
		// These are just sets of subscribers, though modelled as maps
		// for ease of management (not a lot of subscriptions is really
		// expected, but maps are convenient for adding/deleting elements).
		blockFeed        = make(map[chan *block.Block]bool)
		headerFeed       = make(map[chan *block.Header]bool)
		txFeed           = make(map[chan *transaction.Transaction]bool)
		notificationFeed = make(map[chan *state.ContainedNotificationEvent]bool)
		executionFeed    = make(map[chan *state.AppExecResult]bool)
	)
	for {
		select {
		case <-bc.stopCh:
			return
		case sub := <-bc.subCh:
			switch ch := sub.(type) {
			case chan *block.Header:
				headerFeed[ch] = true
			case chan *block.Block:
				blockFeed[ch] = true
			case chan *transaction.Transaction:
				txFeed[ch] = true
			case chan *state.ContainedNotificationEvent:
				notificationFeed[ch] = true
			case chan *state.AppExecResult:
				executionFeed[ch] = true
			default:
				panic(fmt.Sprintf("bad subscription: %T", sub))
			}
		case unsub := <-bc.unsubCh:
			switch ch := unsub.(type) {
			case chan *block.Header:
				delete(headerFeed, ch)
			case chan *block.Block:
				delete(blockFeed, ch)
			case chan *transaction.Transaction:
				delete(txFeed, ch)
			case chan *state.ContainedNotificationEvent:
				delete(notificationFeed, ch)
			case chan *state.AppExecResult:
				delete(executionFeed, ch)
			default:
				panic(fmt.Sprintf("bad unsubscription: %T", unsub))
			}
		case event := <-bc.events:
			// We don't want to waste time looping through transactions when there are no
			// subscribers.
			if len(txFeed) != 0 || len(notificationFeed) != 0 || len(executionFeed) != 0 {
				aer := event.appExecResults[0]
				if !aer.Container.Equals(event.block.Hash()) {
					panic("inconsistent application execution results")
				}
				for ch := range executionFeed {
					ch <- aer
				}
				for i := range aer.Events {
					for ch := range notificationFeed {
						ch <- &state.ContainedNotificationEvent{
							Container:         aer.Container,
							NotificationEvent: aer.Events[i],
						}
					}
				}

				aerIdx := 1
				for _, tx := range event.block.Transactions {
					aer := event.appExecResults[aerIdx]
					if !aer.Container.Equals(tx.Hash()) {
						panic("inconsistent application execution results")
					}
					aerIdx++
					for ch := range executionFeed {
						ch <- aer
					}
					if aer.VMState == vmstate.Halt {
						for i := range aer.Events {
							for ch := range notificationFeed {
								ch <- &state.ContainedNotificationEvent{
									Container:         aer.Container,
									NotificationEvent: aer.Events[i],
								}
							}
						}
					}
					for ch := range txFeed {
						ch <- tx
					}
				}

				aer = event.appExecResults[aerIdx]
				if !aer.Container.Equals(event.block.Hash()) {
					panic("inconsistent application execution results")
				}
				for ch := range executionFeed {
					ch <- aer
				}
				for i := range aer.Events {
					for ch := range notificationFeed {
						ch <- &state.ContainedNotificationEvent{
							Container:         aer.Container,
							NotificationEvent: aer.Events[i],
						}
					}
				}
			}
			for ch := range headerFeed {
				ch <- &event.block.Header
			}
			for ch := range blockFeed {
				ch <- event.block
			}
		}
	}
}

// Close stops Blockchain's internal loop, syncs changes to persistent storage
// and closes it. The Blockchain is no longer functional after the call to Close.
func (bc *Blockchain) Close() {
	// If there is a block addition in progress, wait for it to finish and
	// don't allow new ones.
	bc.addLock.Lock()
	close(bc.stopCh)
	<-bc.runToExitCh
	bc.addLock.Unlock()
	_ = bc.log.Sync()
}

// AddBlock accepts successive block for the Blockchain, verifies it and
// stores internally. Eventually it will be persisted to the backing storage.
func (bc *Blockchain) AddBlock(block *block.Block) error {
	bc.addLock.Lock()
	defer bc.addLock.Unlock()

	var mp *mempool.Pool
	expectedHeight := bc.BlockHeight() + 1
	if expectedHeight != block.Index {
		return fmt.Errorf("expected %d, got %d: %w", expectedHeight, block.Index, ErrInvalidBlockIndex)
	}
	if bc.config.StateRootInHeader != block.StateRootEnabled {
		return fmt.Errorf("%w: %v != %v",
			ErrHdrStateRootSetting, bc.config.StateRootInHeader, block.StateRootEnabled)
	}

	if block.Index == bc.HeaderHeight()+1 {
		err := bc.addHeaders(!bc.config.SkipBlockVerification, &block.Header)
		if err != nil {
			return err
		}
	}
	if !bc.config.SkipBlockVerification {
		merkle := block.ComputeMerkleRoot()
		if !block.MerkleRoot.Equals(merkle) {
			return errors.New("invalid block: MerkleRoot mismatch")
		}
		mp = mempool.New(len(block.Transactions), 0, false, nil)
		for _, tx := range block.Transactions {
			var err error
			// Transactions are verified before adding them
			// into the pool, so there is no point in doing
			// it again even if we're verifying in-block transactions.
			if bc.memPool.ContainsKey(tx.Hash()) {
				err = mp.Add(tx, bc)
				if err == nil {
					continue
				}
			} else {
				err = bc.verifyAndPoolTx(tx, mp, bc)
			}
			if err != nil {
				if bc.config.VerifyTransactions {
					return fmt.Errorf("transaction %s failed to verify: %w", tx.Hash().StringLE(), err)
				}
				bc.log.Warn(fmt.Sprintf("transaction %s failed to verify: %s", tx.Hash().StringLE(), err))
			}
		}
	}
	return bc.storeBlock(block, mp)
}

// AddHeaders processes the given headers and add them to the
// HeaderHashList. It expects headers to be sorted by index.
func (bc *Blockchain) AddHeaders(headers ...*block.Header) error {
	return bc.addHeaders(!bc.config.SkipBlockVerification, headers...)
}

// addHeaders is an internal implementation of AddHeaders (`verify` parameter
// tells it to verify or not verify given headers).
func (bc *Blockchain) addHeaders(verify bool, headers ...*block.Header) error {
	var (
		start = time.Now()
		err   error
	)

	if len(headers) > 0 {
		var i int
		curHeight := bc.HeaderHeight()
		for i = range headers {
			if headers[i].Index > curHeight {
				break
			}
		}
		headers = headers[i:]
	}

	if len(headers) == 0 {
		return nil
	} else if verify {
		// Verify that the chain of the headers is consistent.
		var lastHeader *block.Header
		if lastHeader, err = bc.GetHeader(headers[0].PrevHash); err != nil {
			return fmt.Errorf("previous header was not found: %w", err)
		}
		for _, h := range headers {
			if err = bc.verifyHeader(h, lastHeader); err != nil {
				return err
			}
			lastHeader = h
		}
	}
	res := bc.HeaderHashes.addHeaders(headers...)
	if res == nil {
		bc.log.Debug("done processing headers",
			zap.Uint32("headerIndex", bc.HeaderHeight()),
			zap.Uint32("blockHeight", bc.BlockHeight()),
			zap.Duration("took", time.Since(start)))
	}
	return res
}

// GetStateRoot returns state root for the given height.
func (bc *Blockchain) GetStateRoot(height uint32) (*state.MPTRoot, error) {
	return bc.stateRoot.GetStateRoot(height)
}

// GetStateModule returns state root service instance.
func (bc *Blockchain) GetStateModule() StateRoot {
	return bc.stateRoot
}

// GetStateSyncModule returns new state sync service instance.
func (bc *Blockchain) GetStateSyncModule() *statesync.Module {
	return statesync.NewModule(bc, bc.stateRoot, bc.log, bc.dao, bc.jumpToState)
}

// storeBlock performs chain update using the block given, it executes all
// transactions with all appropriate side-effects and updates Blockchain state.
// This is the only way to change Blockchain state.
func (bc *Blockchain) storeBlock(block *block.Block, txpool *mempool.Pool) error {
	var (
		cache          = bc.dao.GetPrivate()
		aerCache       = bc.dao.GetPrivate()
		appExecResults = make([]*state.AppExecResult, 0, 2+len(block.Transactions))
		aerchan        = make(chan *state.AppExecResult, len(block.Transactions)/8) // Tested 8 and 4 with no practical difference, but feel free to test more and tune.
		aerdone        = make(chan error)
	)
	go func() {
		var (
			kvcache      = aerCache
			err          error
			txCnt        int
			baer1, baer2 *state.AppExecResult
			transCache   = make(map[util.Uint160]transferData)
		)
		kvcache.StoreAsCurrentBlock(block)
		if bc.config.Ledger.RemoveUntraceableBlocks {
			var start, stop uint32
			if bc.config.P2PStateExchangeExtensions {
				// remove batch of old blocks starting from P2-MaxTraceableBlocks-StateSyncInterval up to P2-MaxTraceableBlocks
				if block.Index >= 2*uint32(bc.config.StateSyncInterval) &&
					block.Index >= uint32(bc.config.StateSyncInterval)+bc.config.MaxTraceableBlocks && // check this in case if MaxTraceableBlocks>StateSyncInterval
					int(block.Index)%bc.config.StateSyncInterval == 0 {
					stop = block.Index - uint32(bc.config.StateSyncInterval) - bc.config.MaxTraceableBlocks
					start = stop - min(stop, uint32(bc.config.StateSyncInterval))
				}
			} else if block.Index > bc.config.MaxTraceableBlocks {
				start = block.Index - bc.config.MaxTraceableBlocks // is at least 1
				stop = start + 1
			}
			for index := start; index < stop; index++ {
				err := kvcache.DeleteBlock(bc.GetHeaderHash(index))
				if err != nil {
					bc.log.Warn("error while removing old block",
						zap.Uint32("index", index),
						zap.Error(err))
				}
			}
		}
		for aer := range aerchan {
			if aer.Container == block.Hash() {
				if baer1 == nil {
					baer1 = aer
				} else {
					baer2 = aer
				}
			} else {
				err = kvcache.StoreAsTransaction(block.Transactions[txCnt], block.Index, aer)
				txCnt++
			}
			if err != nil {
				err = fmt.Errorf("failed to store exec result: %w", err)
				break
			}
			if aer.Execution.VMState == vmstate.Halt {
				for j := range aer.Execution.Events {
					bc.handleNotification(&aer.Execution.Events[j], kvcache, transCache, block, aer.Container)
				}
			}
		}
		if err != nil {
			aerdone <- err
			return
		}
		if err := kvcache.StoreAsBlock(block, baer1, baer2); err != nil {
			aerdone <- err
			return
		}
		for acc, trData := range transCache {
			err = kvcache.PutTokenTransferInfo(acc, &trData.Info)
			if err != nil {
				aerdone <- err
				return
			}
			if !trData.Info.NewNEP11Batch {
				kvcache.PutTokenTransferLog(acc, trData.Info.NextNEP11NewestTimestamp, trData.Info.NextNEP11Batch, true, &trData.Log11)
			}
			if !trData.Info.NewNEP17Batch {
				kvcache.PutTokenTransferLog(acc, trData.Info.NextNEP17NewestTimestamp, trData.Info.NextNEP17Batch, false, &trData.Log17)
			}
		}
		close(aerdone)
	}()
	_ = cache.GetItemCtx() // Prime serialization context cache (it'll be reused by upper layer DAOs).
	aer, v, err := bc.runPersist(bc.contracts.GetPersistScript(), block, cache, trigger.OnPersist, nil)
	if err != nil {
		// Release goroutines, don't care about errors, we already have one.
		close(aerchan)
		<-aerdone
		return fmt.Errorf("onPersist failed: %w", err)
	}
	appExecResults = append(appExecResults, aer)
	aerchan <- aer

	for _, tx := range block.Transactions {
		systemInterop := bc.newInteropContext(trigger.Application, cache, block, tx)
		systemInterop.ReuseVM(v)
		v.LoadScriptWithFlags(tx.Script, callflag.All)
		v.GasLimit = tx.SystemFee

		err := systemInterop.Exec()
		var faultException string
		if !v.HasFailed() {
			_, err := systemInterop.DAO.Persist()
			if err != nil {
				// Release goroutines, don't care about errors, we already have one.
				close(aerchan)
				<-aerdone
				return fmt.Errorf("failed to persist invocation results: %w", err)
			}
		} else {
			bc.log.Warn("contract invocation failed",
				zap.String("tx", tx.Hash().StringLE()),
				zap.Uint32("block", block.Index),
				zap.Error(err))
			faultException = err.Error()
		}
		aer := &state.AppExecResult{
			Container: tx.Hash(),
			Execution: state.Execution{
				Trigger:        trigger.Application,
				VMState:        v.State(),
				GasConsumed:    v.GasConsumed(),
				Stack:          v.Estack().ToArray(),
				Events:         systemInterop.Notifications,
				FaultException: faultException,
			},
		}
		appExecResults = append(appExecResults, aer)
		aerchan <- aer
	}

	aer, _, err = bc.runPersist(bc.contracts.GetPostPersistScript(), block, cache, trigger.PostPersist, v)
	if err != nil {
		// Release goroutines, don't care about errors, we already have one.
		close(aerchan)
		<-aerdone
		return fmt.Errorf("postPersist failed: %w", err)
	}
	appExecResults = append(appExecResults, aer)
	aerchan <- aer
	close(aerchan)
	b := mpt.MapToMPTBatch(cache.Store.GetStorageChanges())
	mpt, sr, err := bc.stateRoot.AddMPTBatch(block.Index, b, cache.Store)
	if err != nil {
		// Release goroutines, don't care about errors, we already have one.
		<-aerdone
		// Here MPT can be left in a half-applied state.
		// However if this error occurs, this is a bug somewhere in code
		// because changes applied are the ones from HALTed transactions.
		return fmt.Errorf("error while trying to apply MPT changes: %w", err)
	}
	if bc.config.StateRootInHeader && bc.HeaderHeight() > sr.Index {
		h, err := bc.GetHeader(bc.GetHeaderHash(sr.Index + 1))
		if err != nil {
			err = fmt.Errorf("failed to get next header: %w", err)
		} else if h.PrevStateRoot != sr.Root {
			err = fmt.Errorf("local stateroot and next header's PrevStateRoot mismatch: %s vs %s", sr.Root.StringBE(), h.PrevStateRoot.StringBE())
		}
		if err != nil {
			// Release goroutines, don't care about errors, we already have one.
			<-aerdone
			return err
		}
	}

	if bc.config.Ledger.SaveStorageBatch {
		bc.lastBatch = cache.GetBatch()
	}
	// Every persist cycle we also compact our in-memory MPT. It's flushed
	// already in AddMPTBatch, so collapsing it is safe.
	persistedHeight := atomic.LoadUint32(&bc.persistedHeight)
	if persistedHeight == block.Index-1 {
		// 10 is good and roughly estimated to fit remaining trie into 1M of memory.
		mpt.Collapse(10)
	}

	aererr := <-aerdone
	if aererr != nil {
		return aererr
	}

	bc.lock.Lock()
	_, err = aerCache.Persist()
	if err != nil {
		bc.lock.Unlock()
		return err
	}
	_, err = cache.Persist()
	if err != nil {
		bc.lock.Unlock()
		return err
	}

	mpt.Store = bc.dao.Store
	bc.stateRoot.UpdateCurrentLocal(mpt, sr)
	bc.topBlock.Store(block)
	atomic.StoreUint32(&bc.blockHeight, block.Index)
	bc.memPool.RemoveStale(func(tx *transaction.Transaction) bool { return bc.IsTxStillRelevant(tx, txpool, false) }, bc)
	for _, f := range bc.postBlock {
		f(bc.IsTxStillRelevant, txpool, block)
	}
	if err := bc.updateExtensibleWhitelist(block.Index); err != nil {
		bc.lock.Unlock()
		return err
	}
	bc.lock.Unlock()

	updateBlockHeightMetric(block.Index)
	// Genesis block is stored when Blockchain is not yet running, so there
	// is no one to read this event. And it doesn't make much sense as event
	// anyway.
	if block.Index != 0 {
		bc.events <- bcEvent{block, appExecResults}
	}
	return nil
}

func (bc *Blockchain) updateExtensibleWhitelist(height uint32) error {
	updateCommittee := bc.config.ShouldUpdateCommitteeAt(height)
	stateVals, sh, err := bc.contracts.Designate.GetDesignatedByRole(bc.dao, noderoles.StateValidator, height)
	if err != nil {
		return err
	}

	if bc.extensible.Load() != nil && !updateCommittee && sh != height {
		return nil
	}

	newList := []util.Uint160{bc.contracts.NEO.GetCommitteeAddress(bc.dao)}
	nextVals := bc.contracts.NEO.GetNextBlockValidatorsInternal(bc.dao)
	script, err := smartcontract.CreateDefaultMultiSigRedeemScript(nextVals)
	if err != nil {
		return err
	}
	newList = append(newList, hash.Hash160(script))
	bc.updateExtensibleList(&newList, nextVals)

	if len(stateVals) > 0 {
		h, err := bc.contracts.Designate.GetLastDesignatedHash(bc.dao, noderoles.StateValidator)
		if err != nil {
			return err
		}
		newList = append(newList, h)
		bc.updateExtensibleList(&newList, stateVals)
	}

	sort.Slice(newList, func(i, j int) bool {
		return newList[i].Less(newList[j])
	})
	bc.extensible.Store(newList)
	return nil
}

func (bc *Blockchain) updateExtensibleList(s *[]util.Uint160, pubs keys.PublicKeys) {
	for _, pub := range pubs {
		*s = append(*s, pub.GetScriptHash())
	}
}

// IsExtensibleAllowed determines if script hash is allowed to send extensible payloads.
func (bc *Blockchain) IsExtensibleAllowed(u util.Uint160) bool {
	us := bc.extensible.Load().([]util.Uint160)
	n := sort.Search(len(us), func(i int) bool { return !us[i].Less(u) })
	return n < len(us)
}

func (bc *Blockchain) runPersist(script []byte, block *block.Block, cache *dao.Simple, trig trigger.Type, v *vm.VM) (*state.AppExecResult, *vm.VM, error) {
	systemInterop := bc.newInteropContext(trig, cache, block, nil)
	if v == nil {
		v = systemInterop.SpawnVM()
	} else {
		systemInterop.ReuseVM(v)
	}
	v.LoadScriptWithFlags(script, callflag.All)
	if err := systemInterop.Exec(); err != nil {
		return nil, v, fmt.Errorf("VM has failed: %w", err)
	} else if _, err := systemInterop.DAO.Persist(); err != nil {
		return nil, v, fmt.Errorf("can't save changes: %w", err)
	}
	return &state.AppExecResult{
		Container: block.Hash(), // application logs can be retrieved by block hash
		Execution: state.Execution{
			Trigger:     trig,
			VMState:     v.State(),
			GasConsumed: v.GasConsumed(),
			Stack:       v.Estack().ToArray(),
			Events:      systemInterop.Notifications,
		},
	}, v, nil
}

func (bc *Blockchain) handleNotification(note *state.NotificationEvent, d *dao.Simple,
	transCache map[util.Uint160]transferData, b *block.Block, h util.Uint256) {
	if note.Name != "Transfer" {
		return
	}
	arr, ok := note.Item.Value().([]stackitem.Item)
	if !ok || !(len(arr) == 3 || len(arr) == 4) {
		return
	}
	from, err := parseUint160(arr[0])
	if err != nil {
		return
	}
	to, err := parseUint160(arr[1])
	if err != nil {
		return
	}
	amount, err := arr[2].TryInteger()
	if err != nil {
		return
	}
	var id []byte
	if len(arr) == 4 {
		id, err = arr[3].TryBytes()
		if err != nil || len(id) > limits.MaxStorageKeyLen {
			return
		}
	}
	bc.processTokenTransfer(d, transCache, h, b, note.ScriptHash, from, to, amount, id)
}

func parseUint160(itm stackitem.Item) (util.Uint160, error) {
	_, ok := itm.(stackitem.Null) // Minting or burning.
	if ok {
		return util.Uint160{}, nil
	}
	bytes, err := itm.TryBytes()
	if err != nil {
		return util.Uint160{}, err
	}
	return util.Uint160DecodeBytesBE(bytes)
}

func (bc *Blockchain) processTokenTransfer(cache *dao.Simple, transCache map[util.Uint160]transferData,
	h util.Uint256, b *block.Block, sc util.Uint160, from util.Uint160, to util.Uint160,
	amount *big.Int, tokenID []byte) {
	var id int32
	nativeContract := bc.contracts.ByHash(sc)
	if nativeContract != nil {
		id = nativeContract.Metadata().ID
	} else {
		assetContract, err := native.GetContract(cache, sc)
		if err != nil {
			return
		}
		id = assetContract.ID
	}
	var transfer io.Serializable
	var nep17xfer *state.NEP17Transfer
	var isNEP11 = (tokenID != nil)
	if !isNEP11 {
		nep17xfer = &state.NEP17Transfer{
			Asset:        id,
			Amount:       amount,
			Block:        b.Index,
			Counterparty: to,
			Timestamp:    b.Timestamp,
			Tx:           h,
		}
		transfer = nep17xfer
	} else {
		nep11xfer := &state.NEP11Transfer{
			NEP17Transfer: state.NEP17Transfer{
				Asset:        id,
				Amount:       amount,
				Block:        b.Index,
				Counterparty: to,
				Timestamp:    b.Timestamp,
				Tx:           h,
			},
			ID: tokenID,
		}
		transfer = nep11xfer
		nep17xfer = &nep11xfer.NEP17Transfer
	}
	if !from.Equals(util.Uint160{}) {
		_ = nep17xfer.Amount.Neg(nep17xfer.Amount)
		err := appendTokenTransfer(cache, transCache, from, transfer, id, b.Index, b.Timestamp, isNEP11)
		_ = nep17xfer.Amount.Neg(nep17xfer.Amount)
		if err != nil {
			return
		}
	}
	if !to.Equals(util.Uint160{}) {
		nep17xfer.Counterparty = from
		_ = appendTokenTransfer(cache, transCache, to, transfer, id, b.Index, b.Timestamp, isNEP11) // Nothing useful we can do.
	}
}

func appendTokenTransfer(cache *dao.Simple, transCache map[util.Uint160]transferData, addr util.Uint160, transfer io.Serializable,
	token int32, bIndex uint32, bTimestamp uint64, isNEP11 bool) error {
	transferData, ok := transCache[addr]
	if !ok {
		balances, err := cache.GetTokenTransferInfo(addr)
		if err != nil {
			return err
		}
		if !balances.NewNEP11Batch {
			trLog, err := cache.GetTokenTransferLog(addr, balances.NextNEP11NewestTimestamp, balances.NextNEP11Batch, true)
			if err != nil {
				return err
			}
			transferData.Log11 = *trLog
		}
		if !balances.NewNEP17Batch {
			trLog, err := cache.GetTokenTransferLog(addr, balances.NextNEP17NewestTimestamp, balances.NextNEP17Batch, false)
			if err != nil {
				return err
			}
			transferData.Log17 = *trLog
		}
		transferData.Info = *balances
	}
	var (
		log           *state.TokenTransferLog
		nextBatch     uint32
		currTimestamp uint64
	)
	if !isNEP11 {
		log = &transferData.Log17
		nextBatch = transferData.Info.NextNEP17Batch
		currTimestamp = transferData.Info.NextNEP17NewestTimestamp
	} else {
		log = &transferData.Log11
		nextBatch = transferData.Info.NextNEP11Batch
		currTimestamp = transferData.Info.NextNEP11NewestTimestamp
	}
	err := log.Append(transfer)
	if err != nil {
		return err
	}
	newBatch := log.Size() >= state.TokenTransferBatchSize
	if newBatch {
		cache.PutTokenTransferLog(addr, currTimestamp, nextBatch, isNEP11, log)
		// Put makes a copy of it anyway.
		log.Reset()
	}
	appendTokenTransferInfo(&transferData.Info, token, bIndex, bTimestamp, isNEP11, newBatch)
	transCache[addr] = transferData
	return nil
}

// ForEachNEP17Transfer executes f for each NEP-17 transfer in log starting from
// the transfer with the newest timestamp up to the oldest transfer. It continues
// iteration until false is returned from f. The last non-nil error is returned.
func (bc *Blockchain) ForEachNEP17Transfer(acc util.Uint160, newestTimestamp uint64, f func(*state.NEP17Transfer) (bool, error)) error {
	return bc.dao.SeekNEP17TransferLog(acc, newestTimestamp, f)
}

// ForEachNEP11Transfer executes f for each NEP-11 transfer in log starting from
// the transfer with the newest timestamp up to the oldest transfer. It continues
// iteration until false is returned from f. The last non-nil error is returned.
func (bc *Blockchain) ForEachNEP11Transfer(acc util.Uint160, newestTimestamp uint64, f func(*state.NEP11Transfer) (bool, error)) error {
	return bc.dao.SeekNEP11TransferLog(acc, newestTimestamp, f)
}

// GetNEP17Contracts returns the list of deployed NEP-17 contracts.
func (bc *Blockchain) GetNEP17Contracts() []util.Uint160 {
	return bc.contracts.Management.GetNEP17Contracts(bc.dao)
}

// GetNEP11Contracts returns the list of deployed NEP-11 contracts.
func (bc *Blockchain) GetNEP11Contracts() []util.Uint160 {
	return bc.contracts.Management.GetNEP11Contracts(bc.dao)
}

// GetTokenLastUpdated returns a set of contract ids with the corresponding last updated
// block indexes. In case of an empty account, latest stored state synchronisation point
// is returned under Math.MinInt32 key.
func (bc *Blockchain) GetTokenLastUpdated(acc util.Uint160) (map[int32]uint32, error) {
	info, err := bc.dao.GetTokenTransferInfo(acc)
	if err != nil {
		return nil, err
	}
	if bc.config.P2PStateExchangeExtensions && bc.config.Ledger.RemoveUntraceableBlocks {
		if _, ok := info.LastUpdated[bc.contracts.NEO.ID]; !ok {
			nBalance, lub := bc.contracts.NEO.BalanceOf(bc.dao, acc)
			if nBalance.Sign() != 0 {
				info.LastUpdated[bc.contracts.NEO.ID] = lub
			}
		}
	}
	stateSyncPoint, err := bc.dao.GetStateSyncPoint()
	if err == nil {
		info.LastUpdated[math.MinInt32] = stateSyncPoint
	}
	return info.LastUpdated, nil
}

// GetUtilityTokenBalance returns utility token (GAS) balance for the acc.
func (bc *Blockchain) GetUtilityTokenBalance(acc util.Uint160) *big.Int {
	bs := bc.contracts.GAS.BalanceOf(bc.dao, acc)
	if bs == nil {
		return big.NewInt(0)
	}
	return bs
}

// GetGoverningTokenBalance returns governing token (NEO) balance and the height
// of the last balance change for the account.
func (bc *Blockchain) GetGoverningTokenBalance(acc util.Uint160) (*big.Int, uint32) {
	return bc.contracts.NEO.BalanceOf(bc.dao, acc)
}

// GetNotaryBalance returns Notary deposit amount for the specified account.
func (bc *Blockchain) GetNotaryBalance(acc util.Uint160) *big.Int {
	return bc.contracts.Notary.BalanceOf(bc.dao, acc)
}

// GetNotaryServiceFeePerKey returns a NotaryAssisted transaction attribute fee
// per key which is a reward per notary request key for designated notary nodes.
func (bc *Blockchain) GetNotaryServiceFeePerKey() int64 {
	return bc.contracts.Policy.GetAttributeFeeInternal(bc.dao, transaction.NotaryAssistedT)
}

// GetNotaryContractScriptHash returns Notary native contract hash.
func (bc *Blockchain) GetNotaryContractScriptHash() util.Uint160 {
	if bc.P2PSigExtensionsEnabled() {
		return bc.contracts.Notary.Hash
	}
	return util.Uint160{}
}

// GetNotaryDepositExpiration returns Notary deposit expiration height for the specified account.
func (bc *Blockchain) GetNotaryDepositExpiration(acc util.Uint160) uint32 {
	return bc.contracts.Notary.ExpirationOf(bc.dao, acc)
}

// LastBatch returns last persisted storage batch.
func (bc *Blockchain) LastBatch() *storage.MemBatch {
	return bc.lastBatch
}

// persist flushes current in-memory Store contents to the persistent storage.
func (bc *Blockchain) persist(isSync bool) (time.Duration, error) {
	var (
		start     = time.Now()
		duration  time.Duration
		persisted int
		err       error
	)

	if isSync {
		persisted, err = bc.dao.PersistSync()
	} else {
		persisted, err = bc.dao.Persist()
	}
	if err != nil {
		return 0, err
	}
	if persisted > 0 {
		bHeight, err := bc.persistent.GetCurrentBlockHeight()
		if err != nil {
			return 0, err
		}
		oldHeight := atomic.SwapUint32(&bc.persistedHeight, bHeight)
		diff := bHeight - oldHeight

		storedHeaderHeight, _, err := bc.persistent.GetCurrentHeaderHeight()
		if err != nil {
			return 0, err
		}
		duration = time.Since(start)
		bc.log.Info("persisted to disk",
			zap.Uint32("blocks", diff),
			zap.Int("keys", persisted),
			zap.Uint32("headerHeight", storedHeaderHeight),
			zap.Uint32("blockHeight", bHeight),
			zap.Duration("took", duration))

		// update monitoring metrics.
		updatePersistedHeightMetric(bHeight)
	}

	return duration, nil
}

// GetTransaction returns a TX and its height by the given hash. The height is MaxUint32 if tx is in the mempool.
func (bc *Blockchain) GetTransaction(hash util.Uint256) (*transaction.Transaction, uint32, error) {
	if tx, ok := bc.memPool.TryGetValue(hash); ok {
		return tx, math.MaxUint32, nil // the height is not actually defined for memPool transaction.
	}
	return bc.dao.GetTransaction(hash)
}

// GetAppExecResults returns application execution results with the specified trigger by the given
// tx hash or block hash.
func (bc *Blockchain) GetAppExecResults(hash util.Uint256, trig trigger.Type) ([]state.AppExecResult, error) {
	return bc.dao.GetAppExecResults(hash, trig)
}

// GetStorageItem returns an item from storage.
func (bc *Blockchain) GetStorageItem(id int32, key []byte) state.StorageItem {
	return bc.dao.GetStorageItem(id, key)
}

// SeekStorage performs seek operation over contract storage. Prefix is trimmed in the resulting pair's key.
func (bc *Blockchain) SeekStorage(id int32, prefix []byte, cont func(k, v []byte) bool) {
	bc.dao.Seek(id, storage.SeekRange{Prefix: prefix}, cont)
}

// GetBlock returns a Block by the given hash.
func (bc *Blockchain) GetBlock(hash util.Uint256) (*block.Block, error) {
	topBlock := bc.topBlock.Load()
	if topBlock != nil {
		tb := topBlock.(*block.Block)
		if tb.Hash().Equals(hash) {
			return tb, nil
		}
	}

	block, err := bc.dao.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	if !block.MerkleRoot.Equals(util.Uint256{}) && len(block.Transactions) == 0 {
		return nil, errors.New("only header is found")
	}
	for _, tx := range block.Transactions {
		stx, _, err := bc.dao.GetTransaction(tx.Hash())
		if err != nil {
			return nil, err
		}
		*tx = *stx
	}
	return block, nil
}

// GetHeader returns data block header identified with the given hash value.
func (bc *Blockchain) GetHeader(hash util.Uint256) (*block.Header, error) {
	topBlock := bc.topBlock.Load()
	if topBlock != nil {
		tb := topBlock.(*block.Block)
		if tb.Hash().Equals(hash) {
			return &tb.Header, nil
		}
	}
	block, err := bc.dao.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	return &block.Header, nil
}

// HasBlock returns true if the blockchain contains the given
// block hash.
func (bc *Blockchain) HasBlock(hash util.Uint256) bool {
	if bc.HeaderHashes.haveRecentHash(hash, bc.BlockHeight()) {
		return true
	}

	if header, err := bc.GetHeader(hash); err == nil {
		return header.Index <= bc.BlockHeight()
	}
	return false
}

// CurrentBlockHash returns the highest processed block hash.
func (bc *Blockchain) CurrentBlockHash() util.Uint256 {
	topBlock := bc.topBlock.Load()
	if topBlock != nil {
		tb := topBlock.(*block.Block)
		return tb.Hash()
	}
	return bc.GetHeaderHash(bc.BlockHeight())
}

// BlockHeight returns the height/index of the highest block.
func (bc *Blockchain) BlockHeight() uint32 {
	return atomic.LoadUint32(&bc.blockHeight)
}

// GetContractState returns contract by its script hash.
func (bc *Blockchain) GetContractState(hash util.Uint160) *state.Contract {
	contract, err := native.GetContract(bc.dao, hash)
	if contract == nil && !errors.Is(err, storage.ErrKeyNotFound) {
		bc.log.Warn("failed to get contract state", zap.Error(err))
	}
	return contract
}

// GetContractScriptHash returns contract script hash by its ID.
func (bc *Blockchain) GetContractScriptHash(id int32) (util.Uint160, error) {
	return native.GetContractScriptHash(bc.dao, id)
}

// GetNativeContractScriptHash returns native contract script hash by its name.
func (bc *Blockchain) GetNativeContractScriptHash(name string) (util.Uint160, error) {
	c := bc.contracts.ByName(name)
	if c != nil {
		return c.Metadata().Hash, nil
	}
	return util.Uint160{}, errors.New("Unknown native contract")
}

// GetNatives returns list of native contracts.
func (bc *Blockchain) GetNatives() []state.Contract {
	res := make([]state.Contract, 0, len(bc.contracts.Contracts))
	current := bc.getCurrentHF()
	for _, c := range bc.contracts.Contracts {
		activeIn := c.ActiveIn()
		if !(activeIn == nil || activeIn.Cmp(current) <= 0) {
			continue
		}

		st := bc.GetContractState(c.Metadata().Hash)
		if st != nil { // Should never happen, but better safe than sorry.
			res = append(res, *st)
		}
	}
	return res
}

// GetConfig returns the config stored in the blockchain.
func (bc *Blockchain) GetConfig() config.Blockchain {
	return bc.config
}

// SubscribeForBlocks adds given channel to new block event broadcasting, so when
// there is a new block added to the chain you'll receive it via this channel.
// Make sure it's read from regularly as not reading these events might affect
// other Blockchain functions. Make sure you're not changing the received blocks,
// as it may affect the functionality of Blockchain and other subscribers.
func (bc *Blockchain) SubscribeForBlocks(ch chan *block.Block) {
	bc.subCh <- ch
}

// SubscribeForHeadersOfAddedBlocks adds given channel to new header event broadcasting, so
// when there is a new block added to the chain you'll receive its header via this
// channel. Make sure it's read from regularly as not reading these events might
// affect other Blockchain functions. Make sure you're not changing the received
// headers, as it may affect the functionality of Blockchain and other
// subscribers.
func (bc *Blockchain) SubscribeForHeadersOfAddedBlocks(ch chan *block.Header) {
	bc.subCh <- ch
}

// SubscribeForTransactions adds given channel to new transaction event
// broadcasting, so when there is a new transaction added to the chain (in a
// block) you'll receive it via this channel. Make sure it's read from regularly
// as not reading these events might affect other Blockchain functions. Make sure
// you're not changing the received transactions, as it may affect the
// functionality of Blockchain and other subscribers.
func (bc *Blockchain) SubscribeForTransactions(ch chan *transaction.Transaction) {
	bc.subCh <- ch
}

// SubscribeForNotifications adds given channel to new notifications event
// broadcasting, so when an in-block transaction execution generates a
// notification you'll receive it via this channel. Only notifications from
// successful transactions are broadcasted, if you're interested in failed
// transactions use SubscribeForExecutions instead. Make sure this channel is
// read from regularly as not reading these events might affect other Blockchain
// functions. Make sure you're not changing the received notification events, as
// it may affect the functionality of Blockchain and other subscribers.
func (bc *Blockchain) SubscribeForNotifications(ch chan *state.ContainedNotificationEvent) {
	bc.subCh <- ch
}

// SubscribeForExecutions adds given channel to new transaction execution event
// broadcasting, so when an in-block transaction execution happens you'll receive
// the result of it via this channel. Make sure it's read from regularly as not
// reading these events might affect other Blockchain functions. Make sure you're
// not changing the received execution results, as it may affect the
// functionality of Blockchain and other subscribers.
func (bc *Blockchain) SubscribeForExecutions(ch chan *state.AppExecResult) {
	bc.subCh <- ch
}

// UnsubscribeFromBlocks unsubscribes given channel from new block notifications,
// you can close it afterwards. Passing non-subscribed channel is a no-op, but
// the method can read from this channel (discarding any read data).
func (bc *Blockchain) UnsubscribeFromBlocks(ch chan *block.Block) {
unsubloop:
	for {
		select {
		case <-ch:
		case bc.unsubCh <- ch:
			break unsubloop
		}
	}
}

// UnsubscribeFromHeadersOfAddedBlocks unsubscribes given channel from new
// block's header notifications, you can close it afterwards. Passing
// non-subscribed channel is a no-op, but the method can read from this
// channel (discarding any read data).
func (bc *Blockchain) UnsubscribeFromHeadersOfAddedBlocks(ch chan *block.Header) {
unsubloop:
	for {
		select {
		case <-ch:
		case bc.unsubCh <- ch:
			break unsubloop
		}
	}
}

// UnsubscribeFromTransactions unsubscribes given channel from new transaction
// notifications, you can close it afterwards. Passing non-subscribed channel is
// a no-op, but the method can read from this channel (discarding any read data).
func (bc *Blockchain) UnsubscribeFromTransactions(ch chan *transaction.Transaction) {
unsubloop:
	for {
		select {
		case <-ch:
		case bc.unsubCh <- ch:
			break unsubloop
		}
	}
}

// UnsubscribeFromNotifications unsubscribes given channel from new
// execution-generated notifications, you can close it afterwards. Passing
// non-subscribed channel is a no-op, but the method can read from this channel
// (discarding any read data).
func (bc *Blockchain) UnsubscribeFromNotifications(ch chan *state.ContainedNotificationEvent) {
unsubloop:
	for {
		select {
		case <-ch:
		case bc.unsubCh <- ch:
			break unsubloop
		}
	}
}

// UnsubscribeFromExecutions unsubscribes given channel from new execution
// notifications, you can close it afterwards. Passing non-subscribed channel is
// a no-op, but the method can read from this channel (discarding any read data).
func (bc *Blockchain) UnsubscribeFromExecutions(ch chan *state.AppExecResult) {
unsubloop:
	for {
		select {
		case <-ch:
		case bc.unsubCh <- ch:
			break unsubloop
		}
	}
}

// CalculateClaimable calculates the amount of GAS generated by owning specified
// amount of NEO between specified blocks.
func (bc *Blockchain) CalculateClaimable(acc util.Uint160, endHeight uint32) (*big.Int, error) {
	nextBlock, err := bc.getFakeNextBlock(bc.BlockHeight() + 1)
	if err != nil {
		return nil, err
	}
	ic := bc.newInteropContext(trigger.Application, bc.dao, nextBlock, nil)
	return bc.contracts.NEO.CalculateBonus(ic, acc, endHeight)
}

// FeePerByte returns transaction network fee per byte.
func (bc *Blockchain) FeePerByte() int64 {
	return bc.contracts.Policy.GetFeePerByteInternal(bc.dao)
}

// GetMemPool returns the memory pool of the blockchain.
func (bc *Blockchain) GetMemPool() *mempool.Pool {
	return bc.memPool
}

// ApplyPolicyToTxSet applies configured policies to given transaction set. It
// expects slice to be ordered by fee and returns a subslice of it.
func (bc *Blockchain) ApplyPolicyToTxSet(txes []*transaction.Transaction) []*transaction.Transaction {
	maxTx := bc.config.MaxTransactionsPerBlock
	if maxTx != 0 && len(txes) > int(maxTx) {
		txes = txes[:maxTx]
	}
	maxBlockSize := bc.config.MaxBlockSize
	maxBlockSysFee := bc.config.MaxBlockSystemFee
	oldVC := bc.knownValidatorsCount.Load()
	defaultWitness := bc.defaultBlockWitness.Load()
	curVC := bc.config.GetNumOfCNs(bc.BlockHeight() + 1)
	if oldVC == nil || oldVC != curVC {
		m := smartcontract.GetDefaultHonestNodeCount(curVC)
		verification, _ := smartcontract.CreateDefaultMultiSigRedeemScript(bc.contracts.NEO.GetNextBlockValidatorsInternal(bc.dao))
		defaultWitness = transaction.Witness{
			InvocationScript:   make([]byte, 66*m),
			VerificationScript: verification,
		}
		bc.knownValidatorsCount.Store(curVC)
		bc.defaultBlockWitness.Store(defaultWitness)
	}
	var (
		b           = &block.Block{Header: block.Header{Script: defaultWitness.(transaction.Witness)}}
		blockSize   = uint32(b.GetExpectedBlockSizeWithoutTransactions(len(txes)))
		blockSysFee int64
	)
	for i, tx := range txes {
		blockSize += uint32(tx.Size())
		blockSysFee += tx.SystemFee
		if blockSize > maxBlockSize || blockSysFee > maxBlockSysFee {
			txes = txes[:i]
			break
		}
	}
	return txes
}

// Various errors that could be returns upon header verification.
var (
	ErrHdrHashMismatch     = errors.New("previous header hash doesn't match")
	ErrHdrIndexMismatch    = errors.New("previous header index doesn't match")
	ErrHdrInvalidTimestamp = errors.New("block is not newer than the previous one")
	ErrHdrStateRootSetting = errors.New("state root setting mismatch")
	ErrHdrInvalidStateRoot = errors.New("state root for previous block is invalid")
)

func (bc *Blockchain) verifyHeader(currHeader, prevHeader *block.Header) error {
	if bc.config.StateRootInHeader {
		if bc.stateRoot.CurrentLocalHeight() == prevHeader.Index {
			if sr := bc.stateRoot.CurrentLocalStateRoot(); currHeader.PrevStateRoot != sr {
				return fmt.Errorf("%w: %s != %s",
					ErrHdrInvalidStateRoot, currHeader.PrevStateRoot.StringLE(), sr.StringLE())
			}
		}
	}
	if prevHeader.Hash() != currHeader.PrevHash {
		return ErrHdrHashMismatch
	}
	if prevHeader.Index+1 != currHeader.Index {
		return ErrHdrIndexMismatch
	}
	if prevHeader.Timestamp >= currHeader.Timestamp {
		return ErrHdrInvalidTimestamp
	}
	return bc.verifyHeaderWitnesses(currHeader, prevHeader)
}

// Various errors that could be returned upon verification.
var (
	ErrTxExpired         = errors.New("transaction has expired")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrTxSmallNetworkFee = errors.New("too small network fee")
	ErrTxTooBig          = errors.New("too big transaction")
	ErrMemPoolConflict   = errors.New("invalid transaction due to conflicts with the memory pool")
	ErrInvalidScript     = errors.New("invalid script")
	ErrInvalidAttribute  = errors.New("invalid attribute")
)

// verifyAndPoolTx verifies whether a transaction is bonafide or not and tries
// to add it to the mempool given.
func (bc *Blockchain) verifyAndPoolTx(t *transaction.Transaction, pool *mempool.Pool, feer mempool.Feer, data ...any) error {
	// This code can technically be moved out of here, because it doesn't
	// really require a chain lock.
	err := vm.IsScriptCorrect(t.Script, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidScript, err)
	}

	height := bc.BlockHeight()
	isPartialTx := data != nil
	if t.ValidUntilBlock <= height || !isPartialTx && t.ValidUntilBlock > height+bc.config.MaxValidUntilBlockIncrement {
		return fmt.Errorf("%w: ValidUntilBlock = %d, current height = %d", ErrTxExpired, t.ValidUntilBlock, height)
	}
	// Policying.
	if err := bc.contracts.Policy.CheckPolicy(bc.dao, t); err != nil {
		// Only one %w can be used.
		return fmt.Errorf("%w: %w", ErrPolicy, err)
	}
	if t.SystemFee > bc.config.MaxBlockSystemFee {
		return fmt.Errorf("%w: too big system fee (%d > MaxBlockSystemFee %d)", ErrPolicy, t.SystemFee, bc.config.MaxBlockSystemFee)
	}
	size := t.Size()
	if size > transaction.MaxTransactionSize {
		return fmt.Errorf("%w: (%d > MaxTransactionSize %d)", ErrTxTooBig, size, transaction.MaxTransactionSize)
	}
	needNetworkFee := int64(size)*bc.FeePerByte() + bc.CalculateAttributesFee(t)
	netFee := t.NetworkFee - needNetworkFee
	if netFee < 0 {
		return fmt.Errorf("%w: net fee is %v, need %v", ErrTxSmallNetworkFee, t.NetworkFee, needNetworkFee)
	}
	// check that current tx wasn't included in the conflicts attributes of some other transaction which is already in the chain
	if err := bc.dao.HasTransaction(t.Hash(), t.Signers, height, bc.config.MaxTraceableBlocks); err != nil {
		switch {
		case errors.Is(err, dao.ErrAlreadyExists):
			return ErrAlreadyExists
		case errors.Is(err, dao.ErrHasConflicts):
			return fmt.Errorf("blockchain: %w", ErrHasConflicts)
		default:
			return err
		}
	}
	err = bc.verifyTxWitnesses(t, nil, isPartialTx, netFee)
	if err != nil {
		return err
	}
	if err := bc.verifyTxAttributes(bc.dao, t, isPartialTx); err != nil {
		return err
	}
	err = pool.Add(t, feer, data...)
	if err != nil {
		switch {
		case errors.Is(err, mempool.ErrConflict):
			return ErrMemPoolConflict
		case errors.Is(err, mempool.ErrDup):
			return ErrAlreadyInPool
		case errors.Is(err, mempool.ErrInsufficientFunds):
			return ErrInsufficientFunds
		case errors.Is(err, mempool.ErrOOM):
			return ErrOOM
		case errors.Is(err, mempool.ErrConflictsAttribute):
			return fmt.Errorf("mempool: %w: %w", ErrHasConflicts, err)
		default:
			return err
		}
	}

	return nil
}

// CalculateAttributesFee returns network fee for all transaction attributes that should be
// paid according to native Policy.
func (bc *Blockchain) CalculateAttributesFee(tx *transaction.Transaction) int64 {
	var feeSum int64
	for _, attr := range tx.Attributes {
		base := bc.contracts.Policy.GetAttributeFeeInternal(bc.dao, attr.Type)
		switch attr.Type {
		case transaction.ConflictsT:
			feeSum += base * int64(len(tx.Signers))
		case transaction.NotaryAssistedT:
			if bc.P2PSigExtensionsEnabled() {
				na := attr.Value.(*transaction.NotaryAssisted)
				feeSum += base * (int64(na.NKeys) + 1)
			}
		default:
			feeSum += base
		}
	}
	return feeSum
}

func (bc *Blockchain) verifyTxAttributes(d *dao.Simple, tx *transaction.Transaction, isPartialTx bool) error {
	for i := range tx.Attributes {
		switch attrType := tx.Attributes[i].Type; attrType {
		case transaction.HighPriority:
			h := bc.contracts.NEO.GetCommitteeAddress(d)
			if !tx.HasSigner(h) {
				return fmt.Errorf("%w: high priority tx is not signed by committee", ErrInvalidAttribute)
			}
		case transaction.OracleResponseT:
			h, err := bc.contracts.Oracle.GetScriptHash(bc.dao)
			if err != nil || h.Equals(util.Uint160{}) {
				return fmt.Errorf("%w: %w", ErrInvalidAttribute, err)
			}
			hasOracle := false
			for i := range tx.Signers {
				if tx.Signers[i].Scopes != transaction.None {
					return fmt.Errorf("%w: oracle tx has invalid signer scope", ErrInvalidAttribute)
				}
				if tx.Signers[i].Account.Equals(h) {
					hasOracle = true
				}
			}
			if !hasOracle {
				return fmt.Errorf("%w: oracle tx is not signed by oracle nodes", ErrInvalidAttribute)
			}
			if !bytes.Equal(tx.Script, bc.contracts.Oracle.GetOracleResponseScript()) {
				return fmt.Errorf("%w: oracle tx has invalid script", ErrInvalidAttribute)
			}
			resp := tx.Attributes[i].Value.(*transaction.OracleResponse)
			req, err := bc.contracts.Oracle.GetRequestInternal(bc.dao, resp.ID)
			if err != nil {
				return fmt.Errorf("%w: oracle tx points to invalid request: %w", ErrInvalidAttribute, err)
			}
			if uint64(tx.NetworkFee+tx.SystemFee) < req.GasForResponse {
				return fmt.Errorf("%w: oracle tx has insufficient gas", ErrInvalidAttribute)
			}
		case transaction.NotValidBeforeT:
			nvb := tx.Attributes[i].Value.(*transaction.NotValidBefore).Height
			curHeight := bc.BlockHeight()
			if isPartialTx {
				maxNVBDelta, err := bc.GetMaxNotValidBeforeDelta()
				if err != nil {
					return fmt.Errorf("%w: failed to retrieve MaxNotValidBeforeDelta value from native Notary contract: %w", ErrInvalidAttribute, err)
				}
				if curHeight+maxNVBDelta < nvb {
					return fmt.Errorf("%w: NotValidBefore (%d) bigger than MaxNVBDelta (%d) allows at height %d", ErrInvalidAttribute, nvb, maxNVBDelta, curHeight)
				}
				if nvb+maxNVBDelta < tx.ValidUntilBlock {
					return fmt.Errorf("%w: NotValidBefore (%d) set more than MaxNVBDelta (%d) away from VUB (%d)", ErrInvalidAttribute, nvb, maxNVBDelta, tx.ValidUntilBlock)
				}
			} else {
				if curHeight < nvb {
					return fmt.Errorf("%w: transaction is not yet valid: NotValidBefore = %d, current height = %d", ErrInvalidAttribute, nvb, curHeight)
				}
			}
		case transaction.ConflictsT:
			conflicts := tx.Attributes[i].Value.(*transaction.Conflicts)
			// Only fully-qualified dao.ErrAlreadyExists error bothers us here, thus, we
			// can safely omit the signers, current index and MTB arguments to HasTransaction call to improve performance a bit.
			if err := bc.dao.HasTransaction(conflicts.Hash, nil, 0, 0); errors.Is(err, dao.ErrAlreadyExists) {
				return fmt.Errorf("%w: conflicting transaction %s is already on chain", ErrInvalidAttribute, conflicts.Hash.StringLE())
			}
		case transaction.NotaryAssistedT:
			if !bc.config.P2PSigExtensions {
				return fmt.Errorf("%w: NotaryAssisted attribute was found, but P2PSigExtensions are disabled", ErrInvalidAttribute)
			}
			if !tx.HasSigner(bc.contracts.Notary.Hash) {
				return fmt.Errorf("%w: NotaryAssisted attribute was found, but transaction is not signed by the Notary native contract", ErrInvalidAttribute)
			}
		default:
			if !bc.config.ReservedAttributes && attrType >= transaction.ReservedLowerBound && attrType <= transaction.ReservedUpperBound {
				return fmt.Errorf("%w: attribute of reserved type was found, but ReservedAttributes are disabled", ErrInvalidAttribute)
			}
		}
	}
	return nil
}

// IsTxStillRelevant is a callback for mempool transaction filtering after the
// new block addition. It returns false for transactions added by the new block
// (passed via txpool) and does witness reverification for non-standard
// contracts. It operates under the assumption that full transaction verification
// was already done so we don't need to check basic things like size, input/output
// correctness, presence in blocks before the new one, etc.
func (bc *Blockchain) IsTxStillRelevant(t *transaction.Transaction, txpool *mempool.Pool, isPartialTx bool) bool {
	var (
		recheckWitness bool
		curheight      = bc.BlockHeight()
	)

	if t.ValidUntilBlock <= curheight {
		return false
	}
	if txpool == nil {
		if bc.dao.HasTransaction(t.Hash(), t.Signers, curheight, bc.config.MaxTraceableBlocks) != nil {
			return false
		}
	} else if txpool.HasConflicts(t, bc) {
		return false
	}
	if err := bc.verifyTxAttributes(bc.dao, t, isPartialTx); err != nil {
		return false
	}
	for i := range t.Scripts {
		if !vm.IsStandardContract(t.Scripts[i].VerificationScript) {
			recheckWitness = true
			break
		}
	}
	if recheckWitness {
		return bc.verifyTxWitnesses(t, nil, isPartialTx) == nil
	}
	return true
}

// VerifyTx verifies whether transaction is bonafide or not relative to the
// current blockchain state. Note that this verification is completely isolated
// from the main node's mempool.
func (bc *Blockchain) VerifyTx(t *transaction.Transaction) error {
	var mp = mempool.New(1, 0, false, nil)
	bc.lock.RLock()
	defer bc.lock.RUnlock()
	return bc.verifyAndPoolTx(t, mp, bc)
}

// PoolTx verifies and tries to add given transaction into the mempool. If not
// given, the default mempool is used. Passing multiple pools is not supported.
func (bc *Blockchain) PoolTx(t *transaction.Transaction, pools ...*mempool.Pool) error {
	var pool = bc.memPool

	bc.lock.RLock()
	defer bc.lock.RUnlock()
	// Programmer error.
	if len(pools) > 1 {
		panic("too many pools given")
	}
	if len(pools) == 1 {
		pool = pools[0]
	}
	return bc.verifyAndPoolTx(t, pool, bc)
}

// PoolTxWithData verifies and tries to add given transaction with additional data into the mempool.
func (bc *Blockchain) PoolTxWithData(t *transaction.Transaction, data any, mp *mempool.Pool, feer mempool.Feer, verificationFunction func(tx *transaction.Transaction, data any) error) error {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

	if verificationFunction != nil {
		err := verificationFunction(t, data)
		if err != nil {
			return err
		}
	}
	return bc.verifyAndPoolTx(t, mp, feer, data)
}

// GetCommittee returns the sorted list of public keys of nodes in committee.
func (bc *Blockchain) GetCommittee() (keys.PublicKeys, error) {
	pubs := bc.contracts.NEO.GetCommitteeMembers(bc.dao)
	sort.Sort(pubs)
	return pubs, nil
}

// ComputeNextBlockValidators returns current validators. Validators list
// returned from this method is updated once per CommitteeSize number of blocks.
// For the last block in the dBFT epoch this method returns the list of validators
// recalculated from the latest relevant information about NEO votes; in this case
// list of validators may differ from the one returned by GetNextBlockValidators.
// For the not-last block of dBFT epoch this method returns the same list as
// GetNextBlockValidators.
func (bc *Blockchain) ComputeNextBlockValidators() []*keys.PublicKey {
	return bc.contracts.NEO.ComputeNextBlockValidators(bc.dao)
}

// GetNextBlockValidators returns next block validators. Validators list returned
// from this method is the sorted top NumOfCNs number of public keys from the
// committee of the current dBFT round (that was calculated once for the
// CommitteeSize number of blocks), thus, validators list returned from this
// method is being updated once per (committee size) number of blocks, but not
// every block.
func (bc *Blockchain) GetNextBlockValidators() ([]*keys.PublicKey, error) {
	return bc.contracts.NEO.GetNextBlockValidatorsInternal(bc.dao), nil
}

// GetEnrollments returns all registered validators.
func (bc *Blockchain) GetEnrollments() ([]state.Validator, error) {
	return bc.contracts.NEO.GetCandidates(bc.dao)
}

// GetTestVM returns an interop context with VM set up for a test run.
func (bc *Blockchain) GetTestVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) (*interop.Context, error) {
	if b == nil {
		var err error
		h := bc.BlockHeight() + 1
		b, err = bc.getFakeNextBlock(h)
		if err != nil {
			return nil, fmt.Errorf("failed to create fake block for height %d: %w", h, err)
		}
	}
	systemInterop := bc.newInteropContext(t, bc.dao, b, tx)
	_ = systemInterop.SpawnVM() // All the other code suppose that the VM is ready.
	return systemInterop, nil
}

// GetTestHistoricVM returns an interop context with VM set up for a test run.
func (bc *Blockchain) GetTestHistoricVM(t trigger.Type, tx *transaction.Transaction, nextBlockHeight uint32) (*interop.Context, error) {
	if bc.config.Ledger.KeepOnlyLatestState {
		return nil, errors.New("only latest state is supported")
	}
	b, err := bc.getFakeNextBlock(nextBlockHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to create fake block for height %d: %w", nextBlockHeight, err)
	}
	var mode = mpt.ModeAll
	if bc.config.Ledger.RemoveUntraceableBlocks {
		if b.Index < bc.BlockHeight()-bc.config.MaxTraceableBlocks {
			return nil, fmt.Errorf("state for height %d is outdated and removed from the storage", b.Index)
		}
		mode |= mpt.ModeGCFlag
	}
	if b.Index < 1 || b.Index > bc.BlockHeight()+1 {
		return nil, fmt.Errorf("unsupported historic chain's height: requested state for %d, chain height %d", b.Index, bc.blockHeight)
	}
	// Assuming that block N-th is processing during historic call, the historic invocation should be based on the storage state of height N-1.
	sr, err := bc.stateRoot.GetStateRoot(b.Index - 1)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve stateroot for height %d: %w", b.Index, err)
	}
	s := mpt.NewTrieStore(sr.Root, mode, storage.NewPrivateMemCachedStore(bc.dao.Store))
	dTrie := dao.NewSimple(s, bc.config.StateRootInHeader)
	dTrie.Version = bc.dao.Version
	// Initialize native cache before passing DAO to interop context constructor, because
	// the constructor will call BaseExecFee/StoragePrice policy methods on the passed DAO.
	err = bc.initializeNativeCache(b.Index, dTrie)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize native cache backed by historic DAO: %w", err)
	}
	systemInterop := bc.newInteropContext(t, dTrie, b, tx)
	_ = systemInterop.SpawnVM() // All the other code suppose that the VM is ready.
	return systemInterop, nil
}

// getFakeNextBlock returns fake block with the specified index and pre-filled Timestamp field.
func (bc *Blockchain) getFakeNextBlock(nextBlockHeight uint32) (*block.Block, error) {
	b := block.New(bc.config.StateRootInHeader)
	b.Index = nextBlockHeight
	hdr, err := bc.GetHeader(bc.GetHeaderHash(nextBlockHeight - 1))
	if err != nil {
		return nil, err
	}
	b.Timestamp = hdr.Timestamp + uint64(bc.config.TimePerBlock/time.Millisecond)
	return b, nil
}

// Various witness verification errors.
var (
	ErrWitnessHashMismatch         = errors.New("witness hash mismatch")
	ErrNativeContractWitness       = errors.New("native contract witness must have empty verification script")
	ErrVerificationFailed          = errors.New("signature check failed")
	ErrInvalidInvocationScript     = errors.New("invalid invocation script")
	ErrInvalidSignature            = fmt.Errorf("%w: invalid signature", ErrVerificationFailed)
	ErrInvalidVerificationScript   = errors.New("invalid verification script")
	ErrUnknownVerificationContract = errors.New("unknown verification contract")
	ErrInvalidVerificationContract = errors.New("verification contract is missing `verify` method or `verify` method has unexpected return value")
)

// InitVerificationContext initializes context for witness check.
func (bc *Blockchain) InitVerificationContext(ic *interop.Context, hash util.Uint160, witness *transaction.Witness) error {
	if len(witness.VerificationScript) != 0 {
		if witness.ScriptHash() != hash {
			return fmt.Errorf("%w: expected %s, got %s", ErrWitnessHashMismatch, hash.StringLE(), witness.ScriptHash().StringLE())
		}
		if bc.contracts.ByHash(hash) != nil {
			return ErrNativeContractWitness
		}
		err := vm.IsScriptCorrect(witness.VerificationScript, nil)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidVerificationScript, err)
		}
		ic.VM.LoadScriptWithHash(witness.VerificationScript, hash, callflag.ReadOnly)
	} else {
		cs, err := ic.GetContract(hash)
		if err != nil {
			return ErrUnknownVerificationContract
		}
		md := cs.Manifest.ABI.GetMethod(manifest.MethodVerify, -1)
		if md == nil || md.ReturnType != smartcontract.BoolType {
			return ErrInvalidVerificationContract
		}
		verifyOffset := md.Offset
		initOffset := -1
		md = cs.Manifest.ABI.GetMethod(manifest.MethodInit, 0)
		if md != nil {
			initOffset = md.Offset
		}
		ic.Invocations[cs.Hash]++
		ic.VM.LoadNEFMethod(&cs.NEF, &cs.Manifest, util.Uint160{}, hash, callflag.ReadOnly,
			true, verifyOffset, initOffset, nil)
	}
	if len(witness.InvocationScript) != 0 {
		err := vm.IsScriptCorrect(witness.InvocationScript, nil)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrInvalidInvocationScript, err)
		}
		ic.VM.LoadScript(witness.InvocationScript)
	}
	return nil
}

// VerifyWitness checks that w is a correct witness for c signed by h. It returns
// the amount of GAS consumed during verification and an error.
func (bc *Blockchain) VerifyWitness(h util.Uint160, c hash.Hashable, w *transaction.Witness, gas int64) (int64, error) {
	ic := bc.newInteropContext(trigger.Verification, bc.dao, nil, nil)
	ic.Container = c
	if tx, ok := c.(*transaction.Transaction); ok {
		ic.Tx = tx
	}
	return bc.verifyHashAgainstScript(h, w, ic, gas)
}

// verifyHashAgainstScript verifies given hash against the given witness and returns the amount of GAS consumed.
func (bc *Blockchain) verifyHashAgainstScript(hash util.Uint160, witness *transaction.Witness, interopCtx *interop.Context, gas int64) (int64, error) {
	gas = min(gas, bc.contracts.Policy.GetMaxVerificationGas(interopCtx.DAO))

	vm := interopCtx.SpawnVM()
	vm.GasLimit = gas
	if err := bc.InitVerificationContext(interopCtx, hash, witness); err != nil {
		return 0, err
	}
	err := interopCtx.Exec()
	if vm.HasFailed() {
		return 0, fmt.Errorf("%w: vm execution has failed: %w", ErrVerificationFailed, err)
	}
	estack := vm.Estack()
	if estack.Len() > 0 {
		resEl := estack.Pop()
		res, err := resEl.Item().TryBool()
		if err != nil {
			return 0, fmt.Errorf("%w: invalid return value", ErrVerificationFailed)
		}
		if vm.Estack().Len() != 0 {
			return 0, fmt.Errorf("%w: expected exactly one returned value", ErrVerificationFailed)
		}
		if !res {
			return vm.GasConsumed(), ErrInvalidSignature
		}
	} else {
		return 0, fmt.Errorf("%w: no result returned from the script", ErrVerificationFailed)
	}
	return vm.GasConsumed(), nil
}

// verifyTxWitnesses verifies the scripts (witnesses) that come with a given
// transaction. It can reorder them by ScriptHash, because that's required to
// match a slice of script hashes from the Blockchain. Block parameter
// is used for easy interop access and can be omitted for transactions that are
// not yet added into any block. verificationFee argument can be provided to
// restrict the maximum amount of GAS allowed to spend on transaction
// verification.
// Golang implementation of VerifyWitnesses method in C# (https://github.com/neo-project/neo/blob/master/neo/SmartContract/Helper.cs#L87).
func (bc *Blockchain) verifyTxWitnesses(t *transaction.Transaction, block *block.Block, isPartialTx bool, verificationFee ...int64) error {
	interopCtx := bc.newInteropContext(trigger.Verification, bc.dao, block, t)
	var gasLimit int64
	if len(verificationFee) == 0 {
		gasLimit = t.NetworkFee - int64(t.Size())*bc.FeePerByte() - bc.CalculateAttributesFee(t)
	} else {
		gasLimit = verificationFee[0]
	}
	for i := range t.Signers {
		gasConsumed, err := bc.verifyHashAgainstScript(t.Signers[i].Account, &t.Scripts[i], interopCtx, gasLimit)
		if err != nil &&
			!(i == 0 && isPartialTx && errors.Is(err, ErrInvalidSignature)) { // it's OK for partially-filled transaction with dummy first witness.
			return fmt.Errorf("witness #%d: %w", i, err)
		}
		gasLimit -= gasConsumed
	}

	return nil
}

// verifyHeaderWitnesses is a block-specific implementation of VerifyWitnesses logic.
func (bc *Blockchain) verifyHeaderWitnesses(currHeader, prevHeader *block.Header) error {
	hash := prevHeader.NextConsensus
	_, err := bc.VerifyWitness(hash, currHeader, &currHeader.Script, HeaderVerificationGasLimit)
	return err
}

// GoverningTokenHash returns the governing token (NEO) native contract hash.
func (bc *Blockchain) GoverningTokenHash() util.Uint160 {
	return bc.contracts.NEO.Hash
}

// UtilityTokenHash returns the utility token (GAS) native contract hash.
func (bc *Blockchain) UtilityTokenHash() util.Uint160 {
	return bc.contracts.GAS.Hash
}

// ManagementContractHash returns management contract's hash.
func (bc *Blockchain) ManagementContractHash() util.Uint160 {
	return bc.contracts.Management.Hash
}

func (bc *Blockchain) newInteropContext(trigger trigger.Type, d *dao.Simple, block *block.Block, tx *transaction.Transaction) *interop.Context {
	baseExecFee := int64(interop.DefaultBaseExecFee)
	if block == nil || block.Index != 0 {
		// Use provided dao instead of Blockchain's one to fetch possible ExecFeeFactor
		// changes that were not yet persisted to Blockchain's dao.
		baseExecFee = bc.contracts.Policy.GetExecFeeFactorInternal(d)
	}
	baseStorageFee := int64(native.DefaultStoragePrice)
	if block == nil || block.Index != 0 {
		// Use provided dao instead of Blockchain's one to fetch possible StoragePrice
		// changes that were not yet persisted to Blockchain's dao.
		baseStorageFee = bc.contracts.Policy.GetStoragePriceInternal(d)
	}
	ic := interop.NewContext(trigger, bc, d, baseExecFee, baseStorageFee, native.GetContract, bc.contracts.Contracts, contract.LoadToken, block, tx, bc.log)
	ic.Functions = systemInterops
	switch {
	case tx != nil:
		ic.Container = tx
	case block != nil:
		ic.Container = block
	}
	ic.InitNonceData()
	return ic
}

// P2PSigExtensionsEnabled defines whether P2P signature extensions are enabled.
func (bc *Blockchain) P2PSigExtensionsEnabled() bool {
	return bc.config.P2PSigExtensions
}

// RegisterPostBlock appends provided function to the list of functions which should be run after new block
// is stored.
func (bc *Blockchain) RegisterPostBlock(f func(func(*transaction.Transaction, *mempool.Pool, bool) bool, *mempool.Pool, *block.Block)) {
	bc.postBlock = append(bc.postBlock, f)
}

// GetBaseExecFee return execution price for `NOP`.
func (bc *Blockchain) GetBaseExecFee() int64 {
	if bc.BlockHeight() == 0 {
		return interop.DefaultBaseExecFee
	}
	return bc.contracts.Policy.GetExecFeeFactorInternal(bc.dao)
}

// GetMaxVerificationGAS returns maximum verification GAS Policy limit.
func (bc *Blockchain) GetMaxVerificationGAS() int64 {
	return bc.contracts.Policy.GetMaxVerificationGas(bc.dao)
}

// GetMaxNotValidBeforeDelta returns maximum NotValidBeforeDelta Notary limit.
func (bc *Blockchain) GetMaxNotValidBeforeDelta() (uint32, error) {
	if !bc.config.P2PSigExtensions {
		panic("disallowed call to Notary") // critical error, thus panic.
	}
	if !bc.isHardforkEnabled(bc.contracts.Notary.ActiveIn(), bc.BlockHeight()) {
		return 0, fmt.Errorf("native Notary is active starting from %s", bc.contracts.Notary.ActiveIn().String())
	}
	return bc.contracts.Notary.GetMaxNotValidBeforeDelta(bc.dao), nil
}

// GetStoragePrice returns current storage price.
func (bc *Blockchain) GetStoragePrice() int64 {
	if bc.BlockHeight() == 0 {
		return native.DefaultStoragePrice
	}
	return bc.contracts.Policy.GetStoragePriceInternal(bc.dao)
}
