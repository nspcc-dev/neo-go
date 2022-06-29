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

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer/services"
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
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result/subscriptions"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"go.uber.org/zap"
)

// Tuning parameters.
const (
	headerBatchCount = 2000
	version          = "0.2.6"

	defaultInitialGAS                      = 52000000_00000000
	defaultGCPeriod                        = 10000
	defaultMemPoolSize                     = 50000
	defaultP2PNotaryRequestPayloadPoolSize = 1000
	defaultMaxBlockSize                    = 262144
	defaultMaxBlockSystemFee               = 900000000000
	defaultMaxTraceableBlocks              = 2102400 // 1 year of 15s blocks
	defaultMaxTransactionsPerBlock         = 512
	// HeaderVerificationGasLimit is the maximum amount of GAS for block header verification.
	HeaderVerificationGasLimit             = 3_00000000 // 3 GAS
	defaultStateSyncInterval               = 40000
	defaultDBFTWatchdogThresholdMultiplier = 60 // 60 blocks ~ 15min of reset threshold for 15-seconds blocks.
)

// stateJumpStage denotes the stage of state jump process.
type stateJumpStage byte

const (
	// none means that no state jump process was initiated yet.
	none stateJumpStage = 1 << iota
	// stateJumpStarted means that state jump was just initiated, but outdated storage items
	// were not yet removed.
	stateJumpStarted
	// newStorageItemsAdded means that contract storage items are up-to-date with the current
	// state.
	newStorageItemsAdded
	// genesisStateRemoved means that state corresponding to the genesis block was removed
	// from the storage.
	genesisStateRemoved
)

var (
	// ErrAlreadyExists is returned when trying to add some already existing
	// transaction into the pool (not specifying whether it exists in the
	// chain or mempool).
	ErrAlreadyExists = errors.New("already exists")
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
	config config.ProtocolConfiguration

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

	// Number of headers stored in the chain file.
	storedHeaderCount uint32

	// Header hashes list with associated lock.
	headerHashesLock sync.RWMutex
	headerHashes     []util.Uint256

	// Stop synchronization mechanisms.
	stopCh      chan struct{}
	runToExitCh chan struct{}

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
	subCh   chan interface{}
	unsubCh chan interface{}
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
func NewBlockchain(s storage.Store, cfg config.ProtocolConfiguration, log *zap.Logger) (*Blockchain, error) {
	if log == nil {
		return nil, errors.New("empty logger")
	}

	if cfg.InitialGASSupply <= 0 {
		cfg.InitialGASSupply = fixedn.Fixed8(defaultInitialGAS)
		log.Info("initial gas supply is not set or wrong, setting default value", zap.String("InitialGASSupply", cfg.InitialGASSupply.String()))
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
	if cfg.MaxValidUntilBlockIncrement == 0 {
		const secondsPerDay = int(24 * time.Hour / time.Second)

		cfg.MaxValidUntilBlockIncrement = uint32(secondsPerDay / cfg.SecondsPerBlock)
		log.Info("MaxValidUntilBlockIncrement is not set or wrong, using default value",
			zap.Uint32("MaxValidUntilBlockIncrement", cfg.MaxValidUntilBlockIncrement))
	}
	if cfg.P2PStateExchangeExtensions {
		if !cfg.StateRootInHeader {
			return nil, errors.New("P2PStatesExchangeExtensions are enabled, but StateRootInHeader is off")
		}
		if cfg.StateSyncInterval <= 0 {
			cfg.StateSyncInterval = defaultStateSyncInterval
			log.Info("StateSyncInterval is not set or wrong, using default value",
				zap.Int("StateSyncInterval", cfg.StateSyncInterval))
		}
	}
	if cfg.RemoveUntraceableBlocks && cfg.GarbageCollectionPeriod == 0 {
		cfg.GarbageCollectionPeriod = defaultGCPeriod
		log.Info("GarbageCollectionPeriod is not set or wrong, using default value", zap.Uint32("GarbageCollectionPeriod", cfg.GarbageCollectionPeriod))
	}
	if len(cfg.NativeUpdateHistories) == 0 {
		cfg.NativeUpdateHistories = map[string][]uint32{}
		log.Info("NativeActivations are not set, using default values")
	}
	if cfg.Hardforks == nil {
		cfg.Hardforks = map[string]uint32{}
		log.Info("Hardforks are not set, using default value")
	}
	if cfg.EnableDBFTWatchdog && cfg.DBFTWatchdogThresholdMultiplier == 0 {
		cfg.DBFTWatchdogThresholdMultiplier = defaultDBFTWatchdogThresholdMultiplier
		log.Info("DBFTWatchdogThresholdMultiplier is not set, using default value", zap.Int("DBFTWatchdogThresholdMultiplier", defaultDBFTWatchdogThresholdMultiplier))
	}
	bc := &Blockchain{
		config:      cfg,
		dao:         dao.NewSimple(s, cfg.StateRootInHeader, cfg.P2PSigExtensions),
		persistent:  dao.NewSimple(s, cfg.StateRootInHeader, cfg.P2PSigExtensions),
		store:       s,
		stopCh:      make(chan struct{}),
		runToExitCh: make(chan struct{}),
		memPool:     mempool.New(cfg.MemPoolSize, 0, false),
		log:         log,
		events:      make(chan bcEvent),
		subCh:       make(chan interface{}),
		unsubCh:     make(chan interface{}),
		contracts:   *native.NewContracts(cfg),
	}

	bc.stateRoot = stateroot.NewModule(bc.GetConfig(), bc.VerifyWitness, bc.log, bc.dao.Store)
	bc.contracts.Designate.StateRootService = bc.stateRoot

	if err := bc.init(); err != nil {
		return nil, err
	}

	return bc, nil
}

// SetOracle sets oracle module. It doesn't protected by mutex and
// must be called before `bc.Run()` to avoid data race.
func (bc *Blockchain) SetOracle(mod services.Oracle) {
	orc := bc.contracts.Oracle
	md, ok := orc.GetMethod(manifest.MethodVerify, -1)
	if !ok {
		panic(fmt.Errorf("%s method not found", manifest.MethodVerify))
	}
	mod.UpdateNativeContract(orc.NEF.Script, orc.GetOracleResponseScript(),
		orc.Hash, md.MD.Offset)
	orc.Module.Store(mod)
	bc.contracts.Designate.OracleService.Store(mod)
}

// SetNotary sets notary module. It doesn't protected by mutex and
// must be called before `bc.Run()` to avoid data race.
func (bc *Blockchain) SetNotary(mod services.Notary) {
	bc.contracts.Designate.NotaryService.Store(mod)
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
			KeepOnlyLatestState:        bc.config.KeepOnlyLatestState,
			Value:                      version,
		}
		bc.dao.PutVersion(ver)
		bc.dao.Version = ver
		bc.persistent.Version = ver
		genesisBlock, err := CreateGenesisBlock(bc.config)
		if err != nil {
			return err
		}
		bc.headerHashes = []util.Uint256{genesisBlock.Hash()}
		bc.dao.PutCurrentHeader(genesisBlock.Hash(), genesisBlock.Index)
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
	if ver.KeepOnlyLatestState != bc.config.KeepOnlyLatestState {
		return fmt.Errorf("KeepOnlyLatestState setting mismatch (old=%v, new=%v)",
			ver.KeepOnlyLatestState, bc.config.KeepOnlyLatestState)
	}
	bc.dao.Version = ver
	bc.persistent.Version = ver

	// At this point there was no version found in the storage which
	// implies a creating fresh storage with the version specified
	// and the genesis block as first block.
	bc.log.Info("restoring blockchain", zap.String("version", version))

	bc.headerHashes, err = bc.dao.GetHeaderHashes()
	if err != nil {
		return err
	}

	bc.storedHeaderCount = uint32(len(bc.headerHashes))

	currHeaderHeight, currHeaderHash, err := bc.dao.GetCurrentHeaderHeight()
	if err != nil {
		return fmt.Errorf("failed to retrieve current header info: %w", err)
	}
	if bc.storedHeaderCount == 0 && currHeaderHeight == 0 {
		bc.headerHashes = append(bc.headerHashes, currHeaderHash)
	}

	// There is a high chance that the Node is stopped before the next
	// batch of 2000 headers was stored. Via the currentHeaders stored we can sync
	// that with stored blocks.
	if currHeaderHeight >= bc.storedHeaderCount {
		hash := currHeaderHash
		var targetHash util.Uint256
		if len(bc.headerHashes) > 0 {
			targetHash = bc.headerHashes[len(bc.headerHashes)-1]
		} else {
			genesisBlock, err := CreateGenesisBlock(bc.config)
			if err != nil {
				return err
			}
			targetHash = genesisBlock.Hash()
			bc.headerHashes = append(bc.headerHashes, targetHash)
		}
		headers := make([]*block.Header, 0)

		for hash != targetHash {
			header, err := bc.GetHeader(hash)
			if err != nil {
				return fmt.Errorf("could not get header %s: %w", hash, err)
			}
			headers = append(headers, header)
			hash = header.PrevHash
		}
		headerSliceReverse(headers)
		for _, h := range headers {
			bc.headerHashes = append(bc.headerHashes, h.Hash())
		}
	}

	// Check whether StateJump stage is in the storage and continue interrupted state jump if so.
	jumpStage, err := bc.dao.Store.Get([]byte{byte(storage.SYSStateJumpStage)})
	if err == nil {
		if !(bc.GetConfig().P2PStateExchangeExtensions && bc.GetConfig().RemoveUntraceableBlocks) {
			return errors.New("state jump was not completed, but P2PStateExchangeExtensions are disabled or archival node capability is on. " +
				"To start an archival node drop the database manually and restart the node")
		}
		if len(jumpStage) != 1 {
			return fmt.Errorf("invalid state jump stage format")
		}
		// State jump wasn't finished yet, thus continue it.
		stateSyncPoint, err := bc.dao.GetStateSyncPoint()
		if err != nil {
			return fmt.Errorf("failed to get state sync point from the storage")
		}
		return bc.jumpToStateInternal(stateSyncPoint, stateJumpStage(jumpStage[0]))
	}

	bHeight, err := bc.dao.GetCurrentBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to retrieve current block height: %w", err)
	}
	bc.blockHeight = bHeight
	bc.persistedHeight = bHeight
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
	for _, c := range bc.contracts.Contracts {
		md := c.Metadata()
		storedCS := bc.GetContractState(md.Hash)
		history := md.UpdateHistory
		if len(history) == 0 || history[0] > bHeight {
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
		autogenCS := &state.Contract{
			ContractBase:  md.ContractBase,
			UpdateCounter: storedCS.UpdateCounter, // it can be restored only from the DB, so use the stored value.
		}
		autogenCSBytes, err := stackitem.SerializeConvertible(autogenCS)
		if err != nil {
			return fmt.Errorf("failed to check native %s state against autogenerated one: %w", md.Name, err)
		}
		if !bytes.Equal(storedCSBytes, autogenCSBytes) {
			return fmt.Errorf("native %s: version mismatch (stored contract state differs from autogenerated one), "+
				"try to resynchronize the node from the genesis", md.Name)
		}
	}

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
func (bc *Blockchain) jumpToStateInternal(p uint32, stage stateJumpStage) error {
	if p+1 >= uint32(len(bc.headerHashes)) {
		return fmt.Errorf("invalid state sync point %d: headerHeignt is %d", p, len(bc.headerHashes))
	}

	bc.log.Info("jumping to state sync point", zap.Uint32("state sync point", p))

	jumpStageKey := []byte{byte(storage.SYSStateJumpStage)}
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
			err := cache.DeleteBlock(bc.headerHashes[0])
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
		cache.Store.Put(jumpStageKey, []byte{byte(genesisStateRemoved)})
		_, err := cache.Persist()
		if err != nil {
			return fmt.Errorf("failed to persist old items removal: %w", err)
		}
	case genesisStateRemoved:
		// there's nothing to do after that, so just continue with common operations
		// and remove state jump stage in the end.
	default:
		return errors.New("unknown state jump stage")
	}

	block, err := bc.dao.GetBlock(bc.headerHashes[p])
	if err != nil {
		return fmt.Errorf("failed to get current block: %w", err)
	}
	bc.dao.StoreAsCurrentBlock(block)
	bc.topBlock.Store(block)
	atomic.StoreUint32(&bc.blockHeight, p)
	atomic.StoreUint32(&bc.persistedHeight, p)

	block, err = bc.dao.GetBlock(bc.headerHashes[p+1])
	if err != nil {
		return fmt.Errorf("failed to get block to init MPT: %w", err)
	}
	bc.stateRoot.JumpToState(&state.MPTRoot{
		Index: p,
		Root:  block.PrevStateRoot,
	})

	err = bc.initializeNativeCache(block.Index, bc.dao)
	if err != nil {
		return fmt.Errorf("failed to initialize natives cache: %w", err)
	}

	if err := bc.updateExtensibleWhitelist(p); err != nil {
		return fmt.Errorf("failed to update extensible whitelist: %w", err)
	}

	updateBlockHeightMetric(p)

	bc.dao.Store.Delete(jumpStageKey)
	return nil
}

func (bc *Blockchain) initializeNativeCache(blockHeight uint32, d *dao.Simple) error {
	err := bc.contracts.NEO.InitializeCache(blockHeight, d)
	if err != nil {
		return fmt.Errorf("can't init cache for NEO native contract: %w", err)
	}
	err = bc.contracts.Management.InitializeCache(d)
	if err != nil {
		return fmt.Errorf("can't init cache for Management native contract: %w", err)
	}
	err = bc.contracts.Designate.InitializeCache(d)
	if err != nil {
		return fmt.Errorf("can't init cache for Designation native contract: %w", err)
	}
	bc.contracts.Oracle.InitializeCache(d)
	if bc.P2PSigExtensionsEnabled() {
		err = bc.contracts.Notary.InitializeCache(d)
		if err != nil {
			return fmt.Errorf("can't init cache for Notary native contract: %w", err)
		}
	}
	err = bc.contracts.Policy.InitializeCache(d)
	if err != nil {
		return fmt.Errorf("can't init cache for Policy native contract: %w", err)
	}
	return nil
}

// Run runs chain loop, it needs to be run as goroutine and executing it is
// critical for correct Blockchain operation.
func (bc *Blockchain) Run() {
	persistTimer := time.NewTimer(persistInterval)
	defer func() {
		persistTimer.Stop()
		if _, err := bc.persist(true); err != nil {
			bc.log.Warn("failed to persist", zap.Error(err))
		}
		if err := bc.dao.Store.Close(); err != nil {
			bc.log.Warn("failed to close db", zap.Error(err))
		}
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

			if bc.config.RemoveUntraceableBlocks {
				oldPersisted = atomic.LoadUint32(&bc.persistedHeight)
			}
			dur, err := bc.persist(nextSync)
			if err != nil {
				bc.log.Warn("failed to persist blockchain", zap.Error(err))
			}
			if bc.config.RemoveUntraceableBlocks {
				gcDur = bc.tryRunGC(oldPersisted)
			}
			nextSync = dur > persistInterval*2
			interval := persistInterval - dur - gcDur
			if interval <= 0 {
				interval = time.Microsecond // Reset doesn't work with zero value
			}
			persistTimer.Reset(interval)
		}
	}
}

func (bc *Blockchain) tryRunGC(old uint32) time.Duration {
	var dur time.Duration

	new := atomic.LoadUint32(&bc.persistedHeight)
	var tgtBlock = int64(new)

	tgtBlock -= int64(bc.config.MaxTraceableBlocks)
	if bc.config.P2PStateExchangeExtensions {
		syncP := new / uint32(bc.config.StateSyncInterval)
		syncP--
		syncP *= uint32(bc.config.StateSyncInterval)
		if tgtBlock > int64(syncP) {
			tgtBlock = int64(syncP)
		}
	}
	// Always round to the GCP.
	tgtBlock /= int64(bc.config.GarbageCollectionPeriod)
	tgtBlock *= int64(bc.config.GarbageCollectionPeriod)
	// Count periods.
	old /= bc.config.GarbageCollectionPeriod
	new /= bc.config.GarbageCollectionPeriod
	if tgtBlock > int64(bc.config.GarbageCollectionPeriod) && new != old {
		tgtBlock /= int64(bc.config.GarbageCollectionPeriod)
		tgtBlock *= int64(bc.config.GarbageCollectionPeriod)
		dur = bc.stateRoot.GC(uint32(tgtBlock), bc.store)
		dur += bc.removeOldTransfers(uint32(tgtBlock))
	}
	return dur
}

func (bc *Blockchain) removeOldTransfers(index uint32) time.Duration {
	bc.log.Info("starting transfer data garbage collection", zap.Uint32("index", index))
	start := time.Now()
	h, err := bc.GetHeader(bc.GetHeaderHash(int(index)))
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
		blockFeed        = make(map[chan<- *block.Block]bool)
		txFeed           = make(map[chan<- *transaction.Transaction]bool)
		notificationFeed = make(map[chan<- *subscriptions.NotificationEvent]bool)
		executionFeed    = make(map[chan<- *state.AppExecResult]bool)
	)
	for {
		select {
		case <-bc.stopCh:
			return
		case sub := <-bc.subCh:
			switch ch := sub.(type) {
			case chan<- *block.Block:
				blockFeed[ch] = true
			case chan<- *transaction.Transaction:
				txFeed[ch] = true
			case chan<- *subscriptions.NotificationEvent:
				notificationFeed[ch] = true
			case chan<- *state.AppExecResult:
				executionFeed[ch] = true
			default:
				panic(fmt.Sprintf("bad subscription: %T", sub))
			}
		case unsub := <-bc.unsubCh:
			switch ch := unsub.(type) {
			case chan<- *block.Block:
				delete(blockFeed, ch)
			case chan<- *transaction.Transaction:
				delete(txFeed, ch)
			case chan<- *subscriptions.NotificationEvent:
				delete(notificationFeed, ch)
			case chan<- *state.AppExecResult:
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
						ch <- &subscriptions.NotificationEvent{
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
					if aer.VMState == vm.HaltState {
						for i := range aer.Events {
							for ch := range notificationFeed {
								ch <- &subscriptions.NotificationEvent{
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
						ch <- &subscriptions.NotificationEvent{
							Container:         aer.Container,
							NotificationEvent: aer.Events[i],
						}
					}
				}
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
		err := bc.addHeaders(bc.config.VerifyBlocks, &block.Header)
		if err != nil {
			return err
		}
	}
	if bc.config.VerifyBlocks {
		merkle := block.ComputeMerkleRoot()
		if !block.MerkleRoot.Equals(merkle) {
			return errors.New("invalid block: MerkleRoot mismatch")
		}
		mp = mempool.New(len(block.Transactions), 0, false)
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
			if err != nil && bc.config.VerifyTransactions {
				return fmt.Errorf("transaction %s failed to verify: %w", tx.Hash().StringLE(), err)
			}
		}
	}
	return bc.storeBlock(block, mp)
}

// AddHeaders processes the given headers and add them to the
// HeaderHashList. It expects headers to be sorted by index.
func (bc *Blockchain) AddHeaders(headers ...*block.Header) error {
	return bc.addHeaders(bc.config.VerifyBlocks, headers...)
}

// addHeaders is an internal implementation of AddHeaders (`verify` parameter
// tells it to verify or not verify given headers).
func (bc *Blockchain) addHeaders(verify bool, headers ...*block.Header) error {
	var (
		start = time.Now()
		batch = bc.dao.GetPrivate()
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

	bc.headerHashesLock.Lock()
	defer bc.headerHashesLock.Unlock()
	oldlen := len(bc.headerHashes)
	var lastHeader *block.Header
	for _, h := range headers {
		if int(h.Index) != len(bc.headerHashes) {
			continue
		}
		err = batch.StoreHeader(h)
		if err != nil {
			return err
		}
		bc.headerHashes = append(bc.headerHashes, h.Hash())
		lastHeader = h
	}

	if oldlen != len(bc.headerHashes) {
		for int(lastHeader.Index)-headerBatchCount >= int(bc.storedHeaderCount) {
			err = batch.StoreHeaderHashes(bc.headerHashes[bc.storedHeaderCount:bc.storedHeaderCount+headerBatchCount],
				bc.storedHeaderCount)
			if err != nil {
				return err
			}
			bc.storedHeaderCount += headerBatchCount
		}

		batch.PutCurrentHeader(lastHeader.Hash(), lastHeader.Index)
		updateHeaderHeightMetric(len(bc.headerHashes) - 1)
		if _, err = batch.Persist(); err != nil {
			return err
		}
		bc.log.Debug("done processing headers",
			zap.Int("headerIndex", len(bc.headerHashes)-1),
			zap.Uint32("blockHeight", bc.BlockHeight()),
			zap.Duration("took", time.Since(start)))
	}
	return nil
}

// GetStateModule returns state root service instance.
func (bc *Blockchain) GetStateModule() blockchainer.StateRoot {
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
		if bc.config.RemoveUntraceableBlocks {
			var start, stop uint32
			if bc.config.P2PStateExchangeExtensions {
				// remove batch of old blocks starting from P2-MaxTraceableBlocks-StateSyncInterval up to P2-MaxTraceableBlocks
				if block.Index >= 2*uint32(bc.config.StateSyncInterval) &&
					block.Index >= uint32(bc.config.StateSyncInterval)+bc.config.MaxTraceableBlocks && // check this in case if MaxTraceableBlocks>StateSyncInterval
					int(block.Index)%bc.config.StateSyncInterval == 0 {
					stop = block.Index - uint32(bc.config.StateSyncInterval) - bc.config.MaxTraceableBlocks
					if stop > uint32(bc.config.StateSyncInterval) {
						start = stop - uint32(bc.config.StateSyncInterval)
					}
				}
			} else if block.Index > bc.config.MaxTraceableBlocks {
				start = block.Index - bc.config.MaxTraceableBlocks // is at least 1
				stop = start + 1
			}
			for index := start; index < stop; index++ {
				err := kvcache.DeleteBlock(bc.headerHashes[index])
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
			if aer.Execution.VMState == vm.HaltState {
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
		h, err := bc.GetHeader(bc.GetHeaderHash(int(sr.Index) + 1))
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

	if bc.config.SaveStorageBatch {
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
	bc.updateExtensibleList(&newList, bc.contracts.NEO.GetNextBlockValidatorsInternal(bc.dao))

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
		if err != nil || len(id) > storage.MaxStorageKeyLen {
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
		assetContract, err := bc.contracts.Management.GetContract(cache, sc)
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
			Asset:     id,
			Amount:    *amount,
			From:      from,
			To:        to,
			Block:     b.Index,
			Timestamp: b.Timestamp,
			Tx:        h,
		}
		transfer = nep17xfer
	} else {
		nep11xfer := &state.NEP11Transfer{
			NEP17Transfer: state.NEP17Transfer{
				Asset:     id,
				Amount:    *amount,
				From:      from,
				To:        to,
				Block:     b.Index,
				Timestamp: b.Timestamp,
				Tx:        h,
			},
			ID: tokenID,
		}
		transfer = nep11xfer
		nep17xfer = &nep11xfer.NEP17Transfer
	}
	if !from.Equals(util.Uint160{}) {
		_ = nep17xfer.Amount.Neg(&nep17xfer.Amount)
		err := appendTokenTransfer(cache, transCache, from, transfer, id, b.Index, b.Timestamp, isNEP11)
		_ = nep17xfer.Amount.Neg(&nep17xfer.Amount)
		if err != nil {
			return
		}
	}
	if !to.Equals(util.Uint160{}) {
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
		newBatch      *bool
		nextBatch     *uint32
		currTimestamp *uint64
	)
	if !isNEP11 {
		log = &transferData.Log17
		newBatch = &transferData.Info.NewNEP17Batch
		nextBatch = &transferData.Info.NextNEP17Batch
		currTimestamp = &transferData.Info.NextNEP17NewestTimestamp
	} else {
		log = &transferData.Log11
		newBatch = &transferData.Info.NewNEP11Batch
		nextBatch = &transferData.Info.NextNEP11Batch
		currTimestamp = &transferData.Info.NextNEP11NewestTimestamp
	}
	err := log.Append(transfer)
	if err != nil {
		return err
	}
	transferData.Info.LastUpdated[token] = bIndex
	*newBatch = log.Size() >= state.TokenTransferBatchSize
	if *newBatch {
		cache.PutTokenTransferLog(addr, *currTimestamp, *nextBatch, isNEP11, log)
		*nextBatch++
		*currTimestamp = bTimestamp
		// Put makes a copy of it anyway.
		log.Reset()
	}
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
	if bc.config.P2PStateExchangeExtensions && bc.config.RemoveUntraceableBlocks {
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

// GetNotaryServiceFeePerKey returns NotaryServiceFeePerKey which is a reward per
// notary request key for designated notary nodes.
func (bc *Blockchain) GetNotaryServiceFeePerKey() int64 {
	return bc.contracts.Notary.GetNotaryServiceFeePerKey(bc.dao)
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

// HasTransaction returns true if the blockchain contains he given
// transaction hash.
func (bc *Blockchain) HasTransaction(hash util.Uint256) bool {
	if bc.memPool.ContainsKey(hash) {
		return true
	}
	return bc.dao.HasTransaction(hash) == dao.ErrAlreadyExists
}

// HasBlock returns true if the blockchain contains the given
// block hash.
func (bc *Blockchain) HasBlock(hash util.Uint256) bool {
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
	return bc.GetHeaderHash(int(bc.BlockHeight()))
}

// CurrentHeaderHash returns the hash of the latest known header.
func (bc *Blockchain) CurrentHeaderHash() util.Uint256 {
	bc.headerHashesLock.RLock()
	hash := bc.headerHashes[len(bc.headerHashes)-1]
	bc.headerHashesLock.RUnlock()
	return hash
}

// GetHeaderHash returns hash of the header/block with specified index, if
// Blockchain doesn't have a hash for this height, zero Uint256 value is returned.
func (bc *Blockchain) GetHeaderHash(i int) util.Uint256 {
	bc.headerHashesLock.RLock()
	defer bc.headerHashesLock.RUnlock()

	hashesLen := len(bc.headerHashes)
	if hashesLen <= i {
		return util.Uint256{}
	}
	return bc.headerHashes[i]
}

// BlockHeight returns the height/index of the highest block.
func (bc *Blockchain) BlockHeight() uint32 {
	return atomic.LoadUint32(&bc.blockHeight)
}

// HeaderHeight returns the index/height of the highest header.
func (bc *Blockchain) HeaderHeight() uint32 {
	bc.headerHashesLock.RLock()
	n := len(bc.headerHashes)
	bc.headerHashesLock.RUnlock()
	return uint32(n - 1)
}

// GetContractState returns contract by its script hash.
func (bc *Blockchain) GetContractState(hash util.Uint160) *state.Contract {
	contract, err := bc.contracts.Management.GetContract(bc.dao, hash)
	if contract == nil && err != storage.ErrKeyNotFound {
		bc.log.Warn("failed to get contract state", zap.Error(err))
	}
	return contract
}

// GetContractScriptHash returns contract script hash by its ID.
func (bc *Blockchain) GetContractScriptHash(id int32) (util.Uint160, error) {
	return bc.dao.GetContractScriptHash(id)
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
func (bc *Blockchain) GetNatives() []state.NativeContract {
	res := make([]state.NativeContract, 0, len(bc.contracts.Contracts))
	for _, c := range bc.contracts.Contracts {
		res = append(res, c.Metadata().NativeContract)
	}
	return res
}

// GetConfig returns the config stored in the blockchain.
func (bc *Blockchain) GetConfig() config.ProtocolConfiguration {
	return bc.config
}

// SubscribeForBlocks adds given channel to new block event broadcasting, so when
// there is a new block added to the chain you'll receive it via this channel.
// Make sure it's read from regularly as not reading these events might affect
// other Blockchain functions.
func (bc *Blockchain) SubscribeForBlocks(ch chan<- *block.Block) {
	bc.subCh <- ch
}

// SubscribeForTransactions adds given channel to new transaction event
// broadcasting, so when there is a new transaction added to the chain (in a
// block) you'll receive it via this channel. Make sure it's read from regularly
// as not reading these events might affect other Blockchain functions.
func (bc *Blockchain) SubscribeForTransactions(ch chan<- *transaction.Transaction) {
	bc.subCh <- ch
}

// SubscribeForNotifications adds given channel to new notifications event
// broadcasting, so when an in-block transaction execution generates a
// notification you'll receive it via this channel. Only notifications from
// successful transactions are broadcasted, if you're interested in failed
// transactions use SubscribeForExecutions instead. Make sure this channel is
// read from regularly as not reading these events might affect other Blockchain
// functions.
func (bc *Blockchain) SubscribeForNotifications(ch chan<- *subscriptions.NotificationEvent) {
	bc.subCh <- ch
}

// SubscribeForExecutions adds given channel to new transaction execution event
// broadcasting, so when an in-block transaction execution happens you'll receive
// the result of it via this channel. Make sure it's read from regularly as not
// reading these events might affect other Blockchain functions.
func (bc *Blockchain) SubscribeForExecutions(ch chan<- *state.AppExecResult) {
	bc.subCh <- ch
}

// UnsubscribeFromBlocks unsubscribes given channel from new block notifications,
// you can close it afterwards. Passing non-subscribed channel is a no-op.
func (bc *Blockchain) UnsubscribeFromBlocks(ch chan<- *block.Block) {
	bc.unsubCh <- ch
}

// UnsubscribeFromTransactions unsubscribes given channel from new transaction
// notifications, you can close it afterwards. Passing non-subscribed channel is
// a no-op.
func (bc *Blockchain) UnsubscribeFromTransactions(ch chan<- *transaction.Transaction) {
	bc.unsubCh <- ch
}

// UnsubscribeFromNotifications unsubscribes given channel from new
// execution-generated notifications, you can close it afterwards. Passing
// non-subscribed channel is a no-op.
func (bc *Blockchain) UnsubscribeFromNotifications(ch chan<- *subscriptions.NotificationEvent) {
	bc.unsubCh <- ch
}

// UnsubscribeFromExecutions unsubscribes given channel from new execution
// notifications, you can close it afterwards. Passing non-subscribed channel is
// a no-op.
func (bc *Blockchain) UnsubscribeFromExecutions(ch chan<- *state.AppExecResult) {
	bc.unsubCh <- ch
}

// CalculateClaimable calculates the amount of GAS generated by owning specified
// amount of NEO between specified blocks.
func (bc *Blockchain) CalculateClaimable(acc util.Uint160, endHeight uint32) (*big.Int, error) {
	return bc.contracts.NEO.CalculateBonus(bc.dao, acc, endHeight)
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
func (bc *Blockchain) verifyAndPoolTx(t *transaction.Transaction, pool *mempool.Pool, feer mempool.Feer, data ...interface{}) error {
	// This code can technically be moved out of here, because it doesn't
	// really require a chain lock.
	err := vm.IsScriptCorrect(t.Script, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidScript, err)
	}

	height := bc.BlockHeight()
	isPartialTx := data != nil
	if t.ValidUntilBlock <= height || !isPartialTx && t.ValidUntilBlock > height+bc.config.MaxValidUntilBlockIncrement {
		return fmt.Errorf("%w: ValidUntilBlock = %d, current height = %d", ErrTxExpired, t.ValidUntilBlock, height)
	}
	// Policying.
	if err := bc.contracts.Policy.CheckPolicy(bc.dao, t); err != nil {
		// Only one %w can be used.
		return fmt.Errorf("%w: %v", ErrPolicy, err)
	}
	size := t.Size()
	if size > transaction.MaxTransactionSize {
		return fmt.Errorf("%w: (%d > MaxTransactionSize %d)", ErrTxTooBig, size, transaction.MaxTransactionSize)
	}
	needNetworkFee := int64(size) * bc.FeePerByte()
	if bc.P2PSigExtensionsEnabled() {
		attrs := t.GetAttributes(transaction.NotaryAssistedT)
		if len(attrs) != 0 {
			na := attrs[0].Value.(*transaction.NotaryAssisted)
			needNetworkFee += (int64(na.NKeys) + 1) * bc.contracts.Notary.GetNotaryServiceFeePerKey(bc.dao)
		}
	}
	netFee := t.NetworkFee - needNetworkFee
	if netFee < 0 {
		return fmt.Errorf("%w: net fee is %v, need %v", ErrTxSmallNetworkFee, t.NetworkFee, needNetworkFee)
	}
	// check that current tx wasn't included in the conflicts attributes of some other transaction which is already in the chain
	if err := bc.dao.HasTransaction(t.Hash()); err != nil {
		switch {
		case errors.Is(err, dao.ErrAlreadyExists):
			return fmt.Errorf("blockchain: %w", ErrAlreadyExists)
		case errors.Is(err, dao.ErrHasConflicts):
			return fmt.Errorf("blockchain: %w", ErrHasConflicts)
		default:
			return err
		}
	}
	err = bc.verifyTxWitnesses(t, nil, isPartialTx)
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
			return fmt.Errorf("mempool: %w", ErrAlreadyExists)
		case errors.Is(err, mempool.ErrInsufficientFunds):
			return ErrInsufficientFunds
		case errors.Is(err, mempool.ErrOOM):
			return ErrOOM
		case errors.Is(err, mempool.ErrConflictsAttribute):
			return fmt.Errorf("mempool: %w: %s", ErrHasConflicts, err)
		default:
			return err
		}
	}

	return nil
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
				return fmt.Errorf("%w: %v", ErrInvalidAttribute, err)
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
				return fmt.Errorf("%w: oracle tx points to invalid request: %v", ErrInvalidAttribute, err)
			}
			if uint64(tx.NetworkFee+tx.SystemFee) < req.GasForResponse {
				return fmt.Errorf("%w: oracle tx has insufficient gas", ErrInvalidAttribute)
			}
		case transaction.NotValidBeforeT:
			if !bc.config.P2PSigExtensions {
				return fmt.Errorf("%w: NotValidBefore attribute was found, but P2PSigExtensions are disabled", ErrInvalidAttribute)
			}
			nvb := tx.Attributes[i].Value.(*transaction.NotValidBefore).Height
			if isPartialTx {
				maxNVBDelta := bc.contracts.Notary.GetMaxNotValidBeforeDelta(bc.dao)
				if bc.BlockHeight()+maxNVBDelta < nvb {
					return fmt.Errorf("%w: partially-filled transaction should become valid not less then %d blocks after current chain's height %d", ErrInvalidAttribute, maxNVBDelta, bc.BlockHeight())
				}
				if nvb+maxNVBDelta < tx.ValidUntilBlock {
					return fmt.Errorf("%w: partially-filled transaction should be valid during less than %d blocks", ErrInvalidAttribute, maxNVBDelta)
				}
			} else {
				if height := bc.BlockHeight(); height < nvb {
					return fmt.Errorf("%w: transaction is not yet valid: NotValidBefore = %d, current height = %d", ErrInvalidAttribute, nvb, height)
				}
			}
		case transaction.ConflictsT:
			if !bc.config.P2PSigExtensions {
				return fmt.Errorf("%w: Conflicts attribute was found, but P2PSigExtensions are disabled", ErrInvalidAttribute)
			}
			conflicts := tx.Attributes[i].Value.(*transaction.Conflicts)
			if err := bc.dao.HasTransaction(conflicts.Hash); errors.Is(err, dao.ErrAlreadyExists) {
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
	var recheckWitness bool
	var curheight = bc.BlockHeight()

	if t.ValidUntilBlock <= curheight {
		return false
	}
	if txpool == nil {
		if bc.dao.HasTransaction(t.Hash()) != nil {
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
	var mp = mempool.New(1, 0, false)
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
func (bc *Blockchain) PoolTxWithData(t *transaction.Transaction, data interface{}, mp *mempool.Pool, feer mempool.Feer, verificationFunction func(tx *transaction.Transaction, data interface{}) error) error {
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

// GetValidators returns current validators.
func (bc *Blockchain) GetValidators() ([]*keys.PublicKey, error) {
	return bc.contracts.NEO.ComputeNextBlockValidators(bc.blockHeight, bc.dao)
}

// GetNextBlockValidators returns next block validators.
func (bc *Blockchain) GetNextBlockValidators() ([]*keys.PublicKey, error) {
	return bc.contracts.NEO.GetNextBlockValidatorsInternal(bc.dao), nil
}

// GetEnrollments returns all registered validators.
func (bc *Blockchain) GetEnrollments() ([]state.Validator, error) {
	return bc.contracts.NEO.GetCandidates(bc.dao)
}

// GetTestVM returns an interop context with VM set up for a test run.
func (bc *Blockchain) GetTestVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) *interop.Context {
	systemInterop := bc.newInteropContext(t, bc.dao, b, tx)
	_ = systemInterop.SpawnVM() // All the other code suppose that the VM is ready.
	return systemInterop
}

// GetTestHistoricVM returns an interop context with VM set up for a test run.
func (bc *Blockchain) GetTestHistoricVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) (*interop.Context, error) {
	if bc.config.KeepOnlyLatestState {
		return nil, errors.New("only latest state is supported")
	}
	if b == nil {
		return nil, errors.New("block is mandatory to produce test historic VM")
	}
	var mode = mpt.ModeAll
	if bc.config.RemoveUntraceableBlocks {
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
	dTrie := dao.NewSimple(s, bc.config.StateRootInHeader, bc.config.P2PSigExtensions)
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

// Various witness verification errors.
var (
	ErrWitnessHashMismatch         = errors.New("witness hash mismatch")
	ErrNativeContractWitness       = errors.New("native contract witness must have empty verification script")
	ErrVerificationFailed          = errors.New("signature check failed")
	ErrInvalidInvocation           = errors.New("invalid invocation script")
	ErrInvalidSignature            = fmt.Errorf("%w: invalid signature", ErrVerificationFailed)
	ErrInvalidVerification         = errors.New("invalid verification script")
	ErrUnknownVerificationContract = errors.New("unknown verification contract")
	ErrInvalidVerificationContract = errors.New("verification contract is missing `verify` method")
)

// InitVerificationContext initializes context for witness check.
func (bc *Blockchain) InitVerificationContext(ic *interop.Context, hash util.Uint160, witness *transaction.Witness) error {
	if len(witness.VerificationScript) != 0 {
		if witness.ScriptHash() != hash {
			return ErrWitnessHashMismatch
		}
		if bc.contracts.ByHash(hash) != nil {
			return ErrNativeContractWitness
		}
		err := vm.IsScriptCorrect(witness.VerificationScript, nil)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidVerification, err)
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
		ic.VM.LoadNEFMethod(&cs.NEF, util.Uint160{}, hash, callflag.ReadOnly,
			true, verifyOffset, initOffset, nil)
	}
	if len(witness.InvocationScript) != 0 {
		err := vm.IsScriptCorrect(witness.InvocationScript, nil)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidInvocation, err)
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
	return bc.verifyHashAgainstScript(h, w, ic, gas)
}

// verifyHashAgainstScript verifies given hash against the given witness and returns the amount of GAS consumed.
func (bc *Blockchain) verifyHashAgainstScript(hash util.Uint160, witness *transaction.Witness, interopCtx *interop.Context, gas int64) (int64, error) {
	gasPolicy := bc.contracts.Policy.GetMaxVerificationGas(interopCtx.DAO)
	if gas > gasPolicy {
		gas = gasPolicy
	}

	vm := interopCtx.SpawnVM()
	vm.GasLimit = gas
	if err := bc.InitVerificationContext(interopCtx, hash, witness); err != nil {
		return 0, err
	}
	err := interopCtx.Exec()
	if vm.HasFailed() {
		return 0, fmt.Errorf("%w: vm execution has failed: %v", ErrVerificationFailed, err)
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
// not yet added into any block.
// Golang implementation of VerifyWitnesses method in C# (https://github.com/neo-project/neo/blob/master/neo/SmartContract/Helper.cs#L87).
func (bc *Blockchain) verifyTxWitnesses(t *transaction.Transaction, block *block.Block, isPartialTx bool) error {
	interopCtx := bc.newInteropContext(trigger.Verification, bc.dao, block, t)
	gasLimit := t.NetworkFee - int64(t.Size())*bc.FeePerByte()
	if bc.P2PSigExtensionsEnabled() {
		attrs := t.GetAttributes(transaction.NotaryAssistedT)
		if len(attrs) != 0 {
			na := attrs[0].Value.(*transaction.NotaryAssisted)
			gasLimit -= (int64(na.NKeys) + 1) * bc.contracts.Notary.GetNotaryServiceFeePerKey(bc.dao)
		}
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
	var hash util.Uint160
	if prevHeader == nil && currHeader.PrevHash.Equals(util.Uint256{}) {
		hash = currHeader.Script.ScriptHash()
	} else {
		hash = prevHeader.NextConsensus
	}
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
	ic := interop.NewContext(trigger, bc, d, baseExecFee, baseStorageFee, bc.contracts.Management.GetContract, bc.contracts.Contracts, contract.LoadToken, block, tx, bc.log)
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
func (bc *Blockchain) GetMaxNotValidBeforeDelta() uint32 {
	if !bc.config.P2PSigExtensions {
		panic("disallowed call to Notary")
	}
	return bc.contracts.Notary.GetMaxNotValidBeforeDelta(bc.dao)
}

// GetStoragePrice returns current storage price.
func (bc *Blockchain) GetStoragePrice() int64 {
	if bc.BlockHeight() == 0 {
		return native.DefaultStoragePrice
	}
	return bc.contracts.Policy.GetStoragePriceInternal(bc.dao)
}
