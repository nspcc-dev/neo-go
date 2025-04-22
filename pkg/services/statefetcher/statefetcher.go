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
	"github.com/nspcc-dev/neo-go/pkg/core/statesync"
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
	AddContractStorageData(kv []statesync.KeyValue, syncHeight uint32, expectedRoot util.Uint256) error
	GetLastStoredKey() []byte
}

// Service fetches contract storage state from NeoFS.
type Service struct {
	neofs.BasicService

	isActive   atomic.Bool
	isShutdown atomic.Bool
	lock       sync.RWMutex

	cfg               config.NeoFSStateFetcher
	stateSyncInterval uint32
	lastStateIndex    uint32
	lastStateOid      oid.ID

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
		sfs.isShutdown.Store(true)
		return nil, fmt.Errorf("failed to dial NeoFS pool: %w", err)
	}

	if err = containerID.DecodeString(sfs.cfg.ContainerID); err != nil {
		sfs.isShutdown.Store(true)
		return nil, fmt.Errorf("failed to decode container ID: %w", err)
	}

	err = sfs.Retry(func() error {
		containerObj, err = sfs.Pool.ContainerGet(sfs.Ctx, containerID, client.PrmContainerGet{})
		return err
	})
	if err != nil {
		sfs.isShutdown.Store(true)
		return nil, fmt.Errorf("failed to get container: %w", err)
	}

	containerMagic := containerObj.Attribute("Magic")
	if containerMagic != strconv.Itoa(int(sfs.chain.GetConfig().Magic)) {
		sfs.isShutdown.Store(true)
		return nil, fmt.Errorf("container magic mismatch: expected %d, got %s", sfs.chain.GetConfig().Magic, containerMagic)
	}

	return sfs, nil
}

func (sfs *Service) LatestStateObjectHeight() (uint32, error) {
	sfs.lock.RLock()
	if sfs.lastStateIndex != 0 {
		idx := sfs.lastStateIndex
		sfs.lock.RUnlock()
		return idx, nil
	}
	sfs.lock.RUnlock()
	var lastFoundIdx uint32
	var lastFoundOID oid.ID

searchLoop:
	for height := sfs.stateSyncInterval; ; height += sfs.stateSyncInterval {
		select {
		case <-sfs.Ctx.Done():
			return 0, sfs.Ctx.Err()
		default:
		}

		prm := client.PrmObjectSearch{}
		filters := object.NewSearchFilters()
		filters.AddFilter(sfs.cfg.StateAttribute, fmt.Sprintf("%d", height), object.MatchStringEqual)
		prm.SetFilters(filters)

		ctx, cancel := context.WithTimeout(sfs.Ctx, sfs.cfg.Timeout)
		var oids []oid.ID
		var err error
		err = sfs.Retry(func() error {
			oids, err = neofs.ObjectSearch(ctx, sfs.Pool, sfs.Account.PrivateKey(), sfs.cfg.ContainerID, prm)
			return err
		})
		cancel()
		if err != nil {
			sfs.isShutdown.Store(true)
			return 0, fmt.Errorf("failed to search state object at height %d: %w", height, err)
		}

		if len(oids) == 0 {
			break searchLoop
		}
		lastFoundIdx = height
		lastFoundOID = oids[0]
	}
	sfs.lock.Lock()
	sfs.lastStateIndex = lastFoundIdx
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

func (sfs *Service) exiter() {
	if !sfs.isActive.Load() {
		return
	}
	force := <-sfs.quit
	sfs.log.Info("shutting down NeoFS StateFetcher service", zap.Bool("force", force))

	sfs.isActive.Store(false)
	sfs.isShutdown.Store(true)

	if force {
		sfs.CtxCancel()
	}
	<-sfs.runToExiter

	_ = sfs.Pool.Close()
	_ = sfs.log.Sync()

	if sfs.shutdownCallback != nil {
		sfs.shutdownCallback()
	}
	close(sfs.exiterToShutdown)
}

func (sfs *Service) run() {
	defer close(sfs.runToExiter)

	var (
		syncHeight   uint32
		expectedRoot util.Uint256
	)

	reader, err := sfs.objectGet(sfs.Ctx, sfs.lastStateOid.String())
	if err != nil {
		sfs.log.Error("failed to get state object", zap.Error(err))
		sfs.stopService(true)
		return
	}
	defer func() {
		if err = reader.Close(); err != nil {
			sfs.log.Warn("failed to close reader", zap.Error(err))
		}
	}()
	batches := make(chan []statesync.KeyValue, 2)
	go func() {
		defer close(batches)

		br := gio.NewBinReaderFromIO(reader)
		version := br.ReadB()
		if version != 0 || br.Err != nil {
			sfs.log.Error("invalid state object version", zap.Uint8("version", version), zap.Error(br.Err))
			return
		}

		_ = br.ReadU32LE() // Skip network magic check.
		syncHeight = br.ReadU32LE()
		br.ReadBytes(expectedRoot[:])
		if br.Err != nil {
			sfs.log.Error("failed to read state root", zap.Error(br.Err))
			return
		}
		sfs.log.Info("state object", zap.String("root", expectedRoot.StringLE()), zap.Uint32("height", syncHeight))

		lastKey := sfs.chain.GetLastStoredKey()
		batch := make([]statesync.KeyValue, 0, sfs.cfg.KeyValueBatchSize)

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

			if len(lastKey) > 0 && bytes.Compare(key, lastKey) <= 0 {
				continue
			}

			batch = append(batch, statesync.KeyValue{Key: key, Value: value})
			if len(batch) >= sfs.cfg.KeyValueBatchSize {
				batches <- batch
				batch = make([]statesync.KeyValue, 0, sfs.cfg.KeyValueBatchSize)
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
