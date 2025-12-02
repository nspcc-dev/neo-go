package statefetcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	gio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"go.uber.org/zap"
)

// Ledger is the interface for statefetcher.
type Ledger interface {
	GetConfig() config.Blockchain
	HeaderHeight() uint32
	AddContractStorageItems(kv []storage.KeyValue) error
	InitContractStorageSync(r state.MPTRoot) error
	GetLastStoredKey() []byte
	VerifyWitness(h util.Uint160, c hash.Hashable, w *transaction.Witness, gas int64) (int64, error)
}

// Service fetches contract storage state from NeoFS.
type Service struct {
	neofs.BasicService

	isActive   atomic.Bool
	isShutdown atomic.Bool

	cfg               config.NeoFSStateFetcher
	stateSyncInterval uint32

	// lock protects stateRoot and stateObjectID from concurrent update by
	// network server.
	lock          sync.RWMutex
	stateRoot     state.MPTRoot
	stateObjectID oid.ID

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
	var containerObj container.Container
	s.Ctx, s.CtxCancel = context.WithCancel(context.Background())
	if err = s.Pool.Dial(context.Background()); err != nil {
		s.isActive.CompareAndSwap(true, false)
		return nil, fmt.Errorf("failed to dial NeoFS pool: %w", err)
	}

	err = s.Retry(func() error {
		containerObj, err = s.Pool.ContainerGet(s.Ctx, s.ContainerID, client.PrmContainerGet{})
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
	return s, nil
}

// Init initializes Service to fetch the most recent state from NeoFS within
// [minH, maxH] height range. If max is 0, the latest available state object is
// returned. If the cached state object doesn't match the search criteria, a new
// search will be initiated followed by the Service's cache update. The object
// height is returned.
func (s *Service) Init(minH uint32, maxH uint32) (uint32, error) {
	if maxH > 0 && maxH < minH {
		return 0, fmt.Errorf("max height must be greater than min height, got %d/%d", minH, maxH)
	}

	s.lock.RLock()
	if s.stateRoot.Index != 0 && s.stateRoot.Index >= minH && (maxH == 0 || s.stateRoot.Index <= maxH) {
		h := s.stateRoot.Index
		s.lock.RUnlock()
		return h, nil
	}
	s.lock.RUnlock()

	oid, r, err := s.findStateObject(minH, maxH)
	if err != nil {
		s.isActive.CompareAndSwap(true, false)
		return 0, fmt.Errorf("failed to find state object within [%d; %d] range: %w", minH, maxH, err)
	}

	s.log.Info("initializing NeoFS StateFetcher service",
		zap.Stringer("oid", oid),
		zap.Uint32("height", r.Index),
		zap.String("root", r.Root.StringLE()),
		zap.Bool("witnessed", len(r.Witness) > 0))

	s.lock.Lock()
	defer s.lock.Unlock()
	s.stateRoot = *r
	s.stateObjectID = oid

	return r.Index, nil
}

// findStateObject returns the ID of the most recent state object and the
// corresponding stateroot (with optional witness attached) within [minH, maxH]
// height range. If max is 0, the latest available object is returned.
func (s *Service) findStateObject(minH uint32, maxH uint32) (oid.ID, *state.MPTRoot, error) {
	var (
		obj *client.SearchResultItem
		r   = new(state.MPTRoot)
		err error
	)
	filters := object.NewSearchFilters()
	filters.AddFilter(s.cfg.StateAttribute, fmt.Sprintf("%d", minH), object.MatchNumGE)
	if maxH > 0 {
		filters.AddFilter(s.cfg.StateAttribute, fmt.Sprintf("%d", maxH), object.MatchNumLE)
	}
	ctx, cancel := context.WithTimeout(s.Ctx, s.cfg.Timeout)
	defer cancel()

	results, errs := neofs.ObjectSearch(ctx, s.Pool, s.Account.PrivateKey(), s.ContainerID, filters, []string{s.cfg.StateAttribute, neofs.DefaultStateRootAttribute, neofs.DefaultWitnessAttribute})
loop:
	for {
		select {
		case res, ok := <-results:
			if !ok {
				break loop
			}
			obj = &res

		case err = <-errs:
			return oid.ID{}, nil, fmt.Errorf("failed to search state object: %w", err)
		}
	}

	h, err := strconv.ParseUint(obj.Attributes[0], 10, 32)
	if err != nil || h == 0 {
		return oid.ID{}, nil, fmt.Errorf("invalid state object index: %w", err)
	}
	r.Index = uint32(h)

	r.Root, err = util.Uint256DecodeStringLE(obj.Attributes[1])
	if err != nil {
		return oid.ID{}, nil, fmt.Errorf("failed to decode state root from state object attribute: %w", err)
	}

	if len(obj.Attributes[2]) > 0 {
		b, err := base64.StdEncoding.DecodeString(obj.Attributes[2])
		if err != nil {
			return oid.ID{}, nil, fmt.Errorf("failed to decode state root witness from hex: %w", err)
		}
		w := new(transaction.Witness)
		err = w.FromBytes(b)
		if err != nil {
			return oid.ID{}, nil, fmt.Errorf("failed to decode state root witness: %w", err)
		}
		r.Witness = []transaction.Witness{*w}
		_, err = s.chain.VerifyWitness(hash.Hash160(w.VerificationScript), r, w, stateroot.MaxVerificationGAS)
		if err != nil {
			return oid.ID{}, nil, fmt.Errorf("invalid state root witness: %w", err)
		}
	}

	return obj.ID, r, nil
}

// Start starts state fetcher service. If syncPoint is 0, the latest available
// state object will be fetched.
func (s *Service) Start(syncPoint uint32) error {
	if s.IsShutdown() {
		return errors.New("service is already shut down")
	}
	if !s.isActive.CompareAndSwap(false, true) {
		return nil
	}
	s.log.Info("starting NeoFS StateFetcher service")
	go s.exiter()
	go s.run(syncPoint)
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

func (s *Service) run(syncPoint uint32) {
	defer close(s.runToExiter)

	// Ensure Service is initialized at the proper height before accessing state
	// object OID. The node may be recovering after previously interrupted state
	// sync at some older state sync point, hence the initial state object may
	// be too fresh for the StateSync module.
	_, err := s.Init(s.chain.HeaderHeight()-1, syncPoint)
	if err != nil {
		s.log.Error("failed to find the latest state object",
			zap.Uint32("minHeight", s.chain.HeaderHeight()-1),
			zap.Uint32("maxHeight", syncPoint),
			zap.Error(err))
		s.stopService(true)
		return
	}

	var (
		oid oid.ID
		r   state.MPTRoot
	)
	s.lock.RLock()
	r = s.stateRoot
	oid = s.stateObjectID
	s.lock.RUnlock()

	reader, err := s.objectGet(s.Ctx, oid)
	if err != nil {
		s.log.Error("failed to get state object",
			zap.Uint32("height", r.Index),
			zap.Stringer("oid", oid),
			zap.Error(err))
		s.stopService(true)
		return
	}
	defer func() {
		if err = reader.Close(); err != nil {
			s.log.Warn("failed to close state object reader", zap.Error(err))
		}
	}()

	br := gio.NewBinReaderFromIO(reader)
	version := br.ReadB()
	if version != 0 || br.Err != nil {
		s.log.Error("invalid state object version",
			zap.Uint32("expected", 0),
			zap.Uint8("actual", version),
			zap.Error(br.Err))
		return
	}
	magic := br.ReadU32LE()
	if magic != uint32(s.chain.GetConfig().Magic) || br.Err != nil {
		s.log.Error("invalid state object magic",
			zap.Uint32("expected", uint32(s.chain.GetConfig().Magic)),
			zap.Uint32("actual", magic),
			zap.Error(br.Err))
		return
	}
	h := br.ReadU32LE()
	if h != r.Index || br.Err != nil {
		s.log.Error("invalid state object height",
			zap.Uint32("expected", r.Index),
			zap.Uint32("actual", h),
			zap.Error(br.Err))
		return
	}
	root := util.Uint256{}
	br.ReadBytes(root[:])
	if !root.Equals(r.Root) || br.Err != nil {
		s.log.Error("invalid state object root hash",
			zap.String("expected", r.Root.StringLE()),
			zap.String("actual", root.StringLE()),
			zap.Error(br.Err))
		return
	}

	s.log.Info("initializing contract storage sync",
		zap.Uint32("height", s.stateRoot.Index),
		zap.String("root", s.stateRoot.Root.StringLE()),
		zap.Bool("witnessed", len(r.Witness) > 0))
	err = s.chain.InitContractStorageSync(r)
	if err != nil {
		s.log.Error("failed to initialize contract storage sync", zap.Error(err))
		return
	}

	batches := make(chan []storage.KeyValue, 2)
	go func() {
		defer close(batches)

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
			if err = s.chain.AddContractStorageItems(batch); err != nil {
				s.log.Error("failed to add storage batch", zap.Error(err))
				s.stopService(true)
				return
			}
		}
	}
}

func (s *Service) objectGet(ctx context.Context, oid oid.ID) (io.ReadCloser, error) {
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
