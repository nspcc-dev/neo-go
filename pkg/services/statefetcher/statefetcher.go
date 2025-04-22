package statefetcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	gio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"go.uber.org/zap"
)

// Ledger is the interface for statefetcher.
type Ledger interface {
	GetConfig() config.Blockchain
	HeaderHeight() uint32
	AddContractStorageData(kv []storage.KeyValue, syncHeight uint32, expectedRoot util.Uint256) error
	GetLastStoredKey() []byte
}

// Service fetches contract storage state from NeoFS.
type Service struct {
	neofs.BasicService
	containerMagic int

	isActive   atomic.Bool
	isShutdown atomic.Bool
	lock       sync.RWMutex

	cfg                  config.NeoFSStateFetcher
	stateSyncInterval    uint32
	lastStateObjectIndex uint32
	lastStateOid         oid.ID

	chain Ledger
	log   *zap.Logger

	quit             chan bool
	quitOnce         sync.Once
	runToExiter      chan struct{}
	exiterToShutdown chan struct{}

	shutdownCallback func()
}

// New creates a new Service instance.
func New(chain Ledger, cfg config.NeoFSStateFetcher, stateSyncInterval int, logger *zap.Logger, shutdownCallback func()) (*Service, error) {
	if !cfg.Enabled {
		return &Service{}, nil
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = neofs.DefaultTimeout
	}
	if cfg.StateAttribute == "" {
		cfg.StateAttribute = neofs.DefaultStateAttribute
	}
	if cfg.KeyValueBatchSize <= 0 {
		cfg.KeyValueBatchSize = neofs.DefaultKVBatchSize
	}

	basic, err := neofs.NewBasicService(cfg.NeoFSService)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	sfs := &Service{
		BasicService:      basic,
		log:               logger,
		cfg:               cfg,
		chain:             chain,
		shutdownCallback:  shutdownCallback,
		stateSyncInterval: uint32(stateSyncInterval),
		quit:              make(chan bool),
		runToExiter:       make(chan struct{}),
		exiterToShutdown:  make(chan struct{}),
	}

	if sfs.stateSyncInterval == 0 {
		sfs.stateSyncInterval = config.DefaultStateSyncInterval
	}
	var (
		containerID  cid.ID
		containerObj container.Container
	)
	sfs.Ctx, sfs.CtxCancel = context.WithCancel(context.Background())
	if err = sfs.Pool.Dial(context.Background()); err != nil {
		sfs.isActive.CompareAndSwap(true, false)
		return nil, fmt.Errorf("failed to dial NeoFS pool: %w", err)
	}

	if err = containerID.DecodeString(sfs.cfg.ContainerID); err != nil {
		sfs.isActive.CompareAndSwap(true, false)
		return nil, fmt.Errorf("failed to decode container ID: %w", err)
	}

	err = sfs.Retry(func() error {
		containerObj, err = sfs.Pool.ContainerGet(sfs.Ctx, containerID, client.PrmContainerGet{})
		return err
	})
	if err != nil {
		sfs.isActive.CompareAndSwap(true, false)
		return nil, fmt.Errorf("failed to get container: %w", err)
	}

	containerMagic := containerObj.Attribute("Magic")
	if containerMagic != strconv.Itoa(int(sfs.chain.GetConfig().Magic)) {
		sfs.isActive.CompareAndSwap(true, false)
		return nil, fmt.Errorf("container magic mismatch: expected %d, got %s", sfs.chain.GetConfig().Magic, containerMagic)
	}
	sfs.containerMagic, err = strconv.Atoi(containerMagic)
	return sfs, nil
}

func (sfs *Service) LatestStateObjectHeight(h ...uint32) (uint32, error) {
	sfs.lock.RLock()
	if sfs.lastStateObjectIndex != 0 {
		idx := sfs.lastStateObjectIndex
		sfs.lock.RUnlock()
		return idx, nil
	}
	sfs.lock.RUnlock()
	var (
		lastFoundIdx uint32
		lastFoundOID oid.ID
	)

searchLoop:
	for height := sfs.stateSyncInterval; ; height += sfs.stateSyncInterval {
		select {
		case <-sfs.Ctx.Done():
			return 0, sfs.Ctx.Err()
		default:
		}
		if len(h) > 0 {
			height = h[0]
		}
		prm := client.PrmObjectSearch{}
		filters := object.NewSearchFilters()
		filters.AddFilter(sfs.cfg.StateAttribute, fmt.Sprintf("%d", height), object.MatchStringEqual)
		prm.SetFilters(filters)

		ctx, cancel := context.WithTimeout(sfs.Ctx, sfs.cfg.Timeout)
		var (
			oids []oid.ID
			err  error
		)
		err = sfs.Retry(func() error {
			oids, err = neofs.ObjectSearch(ctx, sfs.Pool, sfs.Account.PrivateKey(), sfs.cfg.ContainerID, prm)
			return err
		})
		cancel()
		if err != nil {
			sfs.isActive.CompareAndSwap(true, false)
			return 0, fmt.Errorf("failed to search state object at height %d: %w", height, err)
		}

		if len(oids) == 0 {
			break searchLoop
		}
		lastFoundIdx = height
		lastFoundOID = oids[0]
	}
	sfs.lock.Lock()
	sfs.lastStateObjectIndex = lastFoundIdx
	sfs.lastStateOid = lastFoundOID
	sfs.lock.Unlock()

	return lastFoundIdx, nil
}

// Start begins state fetching.
func (sfs *Service) Start() error {
	if sfs.IsShutdown() {
		return errors.New("service is already shut down")
	}
	if !sfs.isActive.CompareAndSwap(false, true) {
		return nil
	}
	sfs.log.Info("starting NeoFS StateFetcher service")
	go sfs.exiter()
	go sfs.run()
	return nil
}

func (sfs *Service) stopService(force bool) {
	sfs.quitOnce.Do(func() {
		sfs.quit <- force
		close(sfs.quit)
	})
}

// Shutdown requests graceful shutdown of the service.
func (sfs *Service) Shutdown() {
	if !sfs.IsActive() || sfs.IsShutdown() {
		return
	}
	sfs.stopService(true)
	<-sfs.exiterToShutdown
}

// exiter is a routine that is listening to a quitting signal and manages graceful
// Service shutdown process.
func (sfs *Service) exiter() {
	if !sfs.isActive.Load() {
		return
	}
	// Closing signal may come from anyone, but only once.
	force := <-sfs.quit
	sfs.log.Info("shutting down NeoFS StateFetcher service", zap.Bool("force", force))

	sfs.isActive.Store(false)
	sfs.isShutdown.Store(true)
	// Cansel all pending OIDs/blocks downloads in case if shutdown requested by user
	// or caused by downloading error.
	if force {
		sfs.CtxCancel()
	}
	// Wait for the run() to finish.
	<-sfs.runToExiter
	// Everything is done, release resources, turn off the activity marker and let
	// the server know about it.
	_ = sfs.Pool.Close()
	_ = sfs.log.Sync()

	if sfs.shutdownCallback != nil {
		sfs.shutdownCallback()
	}
	// Notify Shutdown routine in case if it's user-triggered shutdown.
	close(sfs.exiterToShutdown)
}

func (sfs *Service) run() {
	defer close(sfs.runToExiter)

	var (
		syncHeight   uint32
		expectedRoot util.Uint256
	)
	if sfs.lastStateOid == (oid.ID{}) {
		_, err := sfs.LatestStateObjectHeight(sfs.chain.HeaderHeight() - 1)
		if err != nil {
			sfs.log.Error("failed to get state object", zap.Error(err))
			sfs.stopService(true)
			return
		}
	}
	reader, err := sfs.objectGet(sfs.Ctx, sfs.lastStateOid.String())
	if err != nil {
		sfs.log.Error("failed to get state object", zap.Error(err), zap.String("oid", sfs.lastStateOid.String()))
		sfs.stopService(true)
		return
	}
	defer func() {
		if err = reader.Close(); err != nil {
			sfs.log.Warn("failed to close reader", zap.Error(err))
		}
	}()
	batches := make(chan []storage.KeyValue, 2)
	go func() {
		defer close(batches)

		br := gio.NewBinReaderFromIO(reader)
		version := br.ReadB()
		if version != 0 || br.Err != nil {
			sfs.log.Error("invalid state object version", zap.Uint8("version", version), zap.Error(br.Err))
			return
		}
		magic := br.ReadU32LE()
		if magic != uint32(sfs.containerMagic) || br.Err != nil {
			sfs.log.Error("invalid state object magic", zap.Uint32("magic", magic))
			return
		}
		syncHeight = br.ReadU32LE()
		br.ReadBytes(expectedRoot[:])
		if br.Err != nil {
			sfs.log.Error("failed to read state root", zap.Error(br.Err))
			return
		}
		sfs.log.Info("contract storage state object found", zap.String("root", expectedRoot.StringLE()), zap.Uint32("height", syncHeight))

		var (
			lastKey       = sfs.chain.GetLastStoredKey()
			skipUntilNext = len(lastKey) > 0
			batch         = make([]storage.KeyValue, 0, sfs.cfg.KeyValueBatchSize)
		)

		for {
			select {
			case <-sfs.Ctx.Done():
				return
			default:
			}

			key := br.ReadVarBytes()
			if errors.Is(br.Err, io.EOF) {
				// Flush remainder.
				if len(batch) > 0 {
					batches <- batch
				}
				return
			}
			if br.Err != nil {
				sfs.log.Error("failed to read key", zap.Error(br.Err))
				return
			}

			value := br.ReadVarBytes()
			if br.Err != nil {
				sfs.log.Error("failed to read value", zap.Error(br.Err))
				return
			}

			if skipUntilNext {
				if !bytes.Equal(key, lastKey) {
					continue
				}
				skipUntilNext = false
			}

			batch = append(batch, storage.KeyValue{Key: key, Value: value})
			if len(batch) >= sfs.cfg.KeyValueBatchSize {
				batches <- batch
				batch = make([]storage.KeyValue, 0, sfs.cfg.KeyValueBatchSize)
			}
		}
	}()

	for {
		select {
		case <-sfs.Ctx.Done():
			sfs.stopService(false)
			return
		case batch, ok := <-batches:
			if !ok {
				sfs.stopService(false)
				return
			}
			if len(batch) == 0 {
				continue
			}
			if err = sfs.chain.AddContractStorageData(batch, syncHeight, expectedRoot); err != nil {
				sfs.log.Error("failed to add storage batch", zap.Error(err))
				sfs.stopService(true)
				return
			}
		}
	}
}

func (sfs *Service) objectGet(ctx context.Context, oid string) (io.ReadCloser, error) {
	u, err := url.Parse(fmt.Sprintf("%s:%s/%s", neofs.URIScheme, sfs.cfg.ContainerID, oid))
	if err != nil {
		return nil, err
	}
	var rc io.ReadCloser
	err = sfs.Retry(func() error {
		rc, err = neofs.GetWithClient(ctx, sfs.Pool, sfs.Account.PrivateKey(), u, false)
		return err
	})
	return rc, err
}

// IsActive checks if the service is running.
func (sfs *Service) IsActive() bool {
	return sfs.isActive.Load() && !sfs.isShutdown.Load()
}

// IsShutdown checks if the service is fully shut down.
func (sfs *Service) IsShutdown() bool {
	return sfs.isShutdown.Load()
}
