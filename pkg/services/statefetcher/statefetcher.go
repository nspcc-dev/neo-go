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
	AddContractStorageItems(kv []storage.KeyValue, syncHeight uint32, expectedRoot util.Uint256) error
	GetLastStoredKey() []byte
}

// Service fetches contract storage state from NeoFS.
type Service struct {
	neofs.BasicService
	containerMagic int

	isActive   atomic.Bool
	isShutdown atomic.Bool

	cfg               config.NeoFSStateFetcher
	stateSyncInterval uint32

	lock                 sync.RWMutex
	lastStateObjectIndex uint32
	lastStateOID         oid.ID

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

	s := &Service{
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

	if s.stateSyncInterval == 0 {
		s.stateSyncInterval = config.DefaultStateSyncInterval
	}
	var (
		containerID  cid.ID
		containerObj container.Container
	)
	s.Ctx, s.CtxCancel = context.WithCancel(context.Background())
	if err = s.Pool.Dial(context.Background()); err != nil {
		s.isActive.CompareAndSwap(true, false)
		return nil, fmt.Errorf("failed to dial NeoFS pool: %w", err)
	}

	if err = containerID.DecodeString(s.cfg.ContainerID); err != nil {
		s.isActive.CompareAndSwap(true, false)
		return nil, fmt.Errorf("failed to decode container ID: %w", err)
	}

	err = s.Retry(func() error {
		containerObj, err = s.Pool.ContainerGet(s.Ctx, containerID, client.PrmContainerGet{})
		return err
	})
	if err != nil {
		s.isActive.CompareAndSwap(true, false)
		return nil, fmt.Errorf("failed to get container: %w", err)
	}

	containerMagic := containerObj.Attribute("Magic")
	if containerMagic != strconv.Itoa(int(s.chain.GetConfig().Magic)) {
		s.isActive.CompareAndSwap(true, false)
		return nil, fmt.Errorf("container magic mismatch: expected %d, got %s", s.chain.GetConfig().Magic, containerMagic)
	}
	s.containerMagic, err = strconv.Atoi(containerMagic)
	return s, nil
}

func (s *Service) LatestStateObjectHeight(h ...uint32) (uint32, error) {
	s.lock.RLock()
	if s.lastStateObjectIndex != 0 {
		idx := s.lastStateObjectIndex
		s.lock.RUnlock()
		return idx, nil
	}
	s.lock.RUnlock()
	var (
		lastFoundIdx uint32
		lastFoundOID oid.ID
	)

searchLoop:
	for height := s.stateSyncInterval; ; height += s.stateSyncInterval {
		select {
		case <-s.Ctx.Done():
			return 0, s.Ctx.Err()
		default:
		}
		if len(h) > 0 {
			height = h[0]
		}
		prm := client.PrmObjectSearch{}
		filters := object.NewSearchFilters()
		filters.AddFilter(s.cfg.StateAttribute, fmt.Sprintf("%d", height), object.MatchStringEqual)
		prm.SetFilters(filters)

		ctx, cancel := context.WithTimeout(s.Ctx, s.cfg.Timeout)
		var (
			oids []oid.ID
			err  error
		)
		err = s.Retry(func() error {
			oids, err = neofs.ObjectSearch(ctx, s.Pool, s.Account.PrivateKey(), s.cfg.ContainerID, prm)
			return err
		})
		cancel()
		if err != nil {
			s.isActive.CompareAndSwap(true, false)
			return 0, fmt.Errorf("failed to search state object at height %d: %w", height, err)
		}

		if len(oids) == 0 {
			break searchLoop
		}
		lastFoundIdx = height
		lastFoundOID = oids[0]
	}
	if lastFoundIdx == 0 || lastFoundOID.IsZero() {
		s.isActive.CompareAndSwap(true, false)
		return 0, fmt.Errorf("no state object found")
	}
	s.lock.Lock()
	s.lastStateObjectIndex = lastFoundIdx
	s.lastStateOID = lastFoundOID
	s.lock.Unlock()

	return lastFoundIdx, nil
}

// Start begins state fetching.
func (s *Service) Start() error {
	if s.IsShutdown() {
		return errors.New("service is already shut down")
	}
	if !s.isActive.CompareAndSwap(false, true) {
		return nil
	}
	s.log.Info("starting NeoFS StateFetcher service")
	go s.exiter()
	go s.run()
	return nil
}

func (s *Service) stopService(force bool) {
	s.quitOnce.Do(func() {
		s.quit <- force
		close(s.quit)
	})
}

// Shutdown requests graceful shutdown of the service.
func (s *Service) Shutdown() {
	if !s.IsActive() || s.IsShutdown() {
		return
	}
	s.stopService(true)
	<-s.exiterToShutdown
}

// exiter is a routine that is listening to a quitting signal and manages graceful
// Service shutdown process.
func (s *Service) exiter() {
	if !s.isActive.Load() {
		return
	}
	// Closing signal may come from anyone, but only once.
	force := <-s.quit
	s.log.Info("shutting down NeoFS StateFetcher service", zap.Bool("force", force))

	s.isActive.Store(false)
	s.isShutdown.Store(true)
	// Cansel all pending OIDs/blocks downloads in case if shutdown requested by user
	// or caused by downloading error.
	if force {
		s.CtxCancel()
	}
	// Wait for the run() to finish.
	<-s.runToExiter
	// Everything is done, release resources, turn off the activity marker and let
	// the server know about it.
	_ = s.Pool.Close()
	_ = s.log.Sync()

	if s.shutdownCallback != nil {
		s.shutdownCallback()
	}
	// Notify Shutdown routine in case if it's user-triggered shutdown.
	close(s.exiterToShutdown)
}

func (s *Service) run() {
	defer close(s.runToExiter)

	var (
		syncHeight   uint32
		expectedRoot util.Uint256
	)

	s.lock.RLock()
	isZero := s.lastStateOID.IsZero()
	s.lock.RUnlock()
	if isZero {
		_, err := s.LatestStateObjectHeight(s.chain.HeaderHeight() - 1)
		if err != nil {
			s.log.Error("failed to get state object", zap.Error(err))
			s.stopService(true)
			return
		}
	}
	s.lock.RLock()
	oidStr := s.lastStateOID.String()
	s.lock.RUnlock()
	reader, err := s.objectGet(s.Ctx, oidStr)
	if err != nil {
		s.log.Error("failed to get state object", zap.Error(err), zap.String("oid", s.lastStateOID.String()))
		s.stopService(true)
		return
	}
	defer func() {
		if err = reader.Close(); err != nil {
			s.log.Warn("failed to close reader", zap.Error(err))
		}
	}()
	batches := make(chan []storage.KeyValue, 2)
	go func() {
		defer close(batches)

		br := gio.NewBinReaderFromIO(reader)
		version := br.ReadB()
		if version != 0 || br.Err != nil {
			s.log.Error("invalid state object version", zap.Uint8("version", version), zap.Error(br.Err))
			return
		}
		magic := br.ReadU32LE()
		if magic != uint32(s.containerMagic) || br.Err != nil {
			s.log.Error("invalid state object magic", zap.Uint32("magic", magic))
			return
		}
		syncHeight = br.ReadU32LE()
		br.ReadBytes(expectedRoot[:])
		if br.Err != nil {
			s.log.Error("failed to read state root", zap.Error(br.Err))
			return
		}
		s.log.Info("contract storage state object found", zap.String("root", expectedRoot.StringLE()), zap.Uint32("height", syncHeight))

		var (
			lastKey = s.chain.GetLastStoredKey()
			skip    = len(lastKey) > 0
			batch   = make([]storage.KeyValue, 0, s.cfg.KeyValueBatchSize)
		)

		for {
			select {
			case <-s.Ctx.Done():
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
				s.log.Error("failed to read key", zap.Error(br.Err))
				return
			}

			value := br.ReadVarBytes()
			if br.Err != nil {
				s.log.Error("failed to read value", zap.Error(br.Err))
				return
			}

			if skip {
				if bytes.Equal(key, lastKey) {
					skip = false
				}
				continue
			}

			batch = append(batch, storage.KeyValue{Key: key, Value: value})
			if len(batch) >= s.cfg.KeyValueBatchSize {
				batches <- batch
				batch = make([]storage.KeyValue, 0, s.cfg.KeyValueBatchSize)
			}
		}
	}()

	for {
		select {
		case <-s.Ctx.Done():
			s.stopService(false)
			return
		case batch, ok := <-batches:
			if !ok {
				s.stopService(false)
				return
			}
			if len(batch) == 0 {
				continue
			}
			if err = s.chain.AddContractStorageItems(batch, syncHeight, expectedRoot); err != nil {
				s.log.Error("failed to add storage batch", zap.Error(err))
				s.stopService(true)
				return
			}
		}
	}
}

func (s *Service) objectGet(ctx context.Context, oid string) (io.ReadCloser, error) {
	u, err := url.Parse(fmt.Sprintf("%s:%s/%s", neofs.URIScheme, s.cfg.ContainerID, oid))
	if err != nil {
		return nil, err
	}
	var rc io.ReadCloser
	err = s.Retry(func() error {
		rc, err = neofs.GetWithClient(ctx, s.Pool, s.Account.PrivateKey(), u, false)
		return err
	})
	return rc, err
}

// IsActive checks if the service is running.
func (s *Service) IsActive() bool {
	return s.isActive.Load() && !s.isShutdown.Load()
}

// IsShutdown checks if the service is fully shut down.
func (s *Service) IsShutdown() bool {
	return s.isShutdown.Load()
}
