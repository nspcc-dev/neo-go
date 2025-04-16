package statefetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/config"
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
	AddContractStorageData(key string, value []byte, syncHeight uint32, expectedRoot util.Uint256) error
}

type kvPair struct {
	key   []byte
	value []byte
	err   error
}

// Service fetches contract storage state from NeoFS.
type Service struct {
	basic          neofs.BasicService
	isActive       atomic.Bool
	isShutdown     atomic.Bool
	cfg            config.NeoFSStateFetcher
	lastStateIndex uint32

	chain Ledger
	log   *zap.Logger
	wg    sync.WaitGroup

	shutdownCallback func()
}

// New creates a new Service instance.
func New(chain Ledger, cfg config.NeoFSStateFetcher, logger *zap.Logger, shutdownCallback func()) (*Service, error) {
	if !cfg.Enabled {
		return &Service{}, nil
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = neofs.DefaultTimeout
	}
	if cfg.StateAttribute == "" {
		cfg.StateAttribute = neofs.DefaultStateAttribute
	}
	if cfg.StateInterval <= 0 {
		cfg.StateInterval = neofs.DefaultStateInterval
	}
	basic, err := neofs.New(cfg.NeoFSService)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}
	sfs := Service{
		basic:            basic,
		log:              logger,
		cfg:              cfg,
		chain:            chain,
		shutdownCallback: shutdownCallback,
	}
	var (
		containerID  cid.ID
		containerObj container.Container
	)
	sfs.basic.Ctx, sfs.basic.CtxCancel = context.WithCancel(context.Background())
	if err = sfs.basic.Pool.Dial(context.Background()); err != nil {
		sfs.isShutdown.CompareAndSwap(false, true)
		return nil, fmt.Errorf("failed to dial NeoFS pool: %w", err)
	}

	err = containerID.DecodeString(sfs.cfg.ContainerID)
	if err != nil {
		sfs.isShutdown.CompareAndSwap(false, true)
		return nil, fmt.Errorf("failed to decode container ID: %w", err)
	}

	err = sfs.basic.Retry(func() error {
		containerObj, err = sfs.basic.Pool.ContainerGet(sfs.basic.Ctx, containerID, client.PrmContainerGet{})
		return err
	})
	if err != nil {
		sfs.isShutdown.CompareAndSwap(false, true)
		return nil, fmt.Errorf("failed to get container: %w", err)
	}
	containerMagic := containerObj.Attribute("Magic")
	if containerMagic != strconv.Itoa(int(sfs.chain.GetConfig().Magic)) {
		sfs.isShutdown.CompareAndSwap(false, true)
		return nil, fmt.Errorf("container magic mismatch: expected %d, got %s", sfs.chain.GetConfig().Magic, containerMagic)
	}
	sfs.isShutdown.CompareAndSwap(true, false)
	return &sfs, nil
}

// LastStateIndex returns the last state index from NeoFS.
func (sfs *Service) LastStateIndex() (uint32, error) {
	if sfs.lastStateIndex != 0 {
		return sfs.lastStateIndex, nil
	}
	startIndex := uint32(1)
	for {
		prm := client.PrmObjectSearch{}
		filters := object.NewSearchFilters()
		filters.AddFilter(sfs.cfg.StateAttribute, fmt.Sprintf("%d", startIndex*neofs.DefaultStateInterval), object.MatchStringEqual)
		prm.SetFilters(filters)

		ctx, cancel := context.WithTimeout(sfs.basic.Ctx, sfs.cfg.Timeout)
		var (
			blockOidsObject []oid.ID
			err             error
		)
		err = sfs.basic.Retry(func() error {
			blockOidsObject, err = neofs.ObjectSearch(ctx, sfs.basic.Pool, sfs.basic.Account.PrivateKey(), sfs.cfg.ContainerID, prm)
			return err
		})
		cancel()
		if err != nil {
			sfs.isShutdown.CompareAndSwap(false, true)
			return 0, fmt.Errorf("failed to find '%s' object with index %d: %w", sfs.cfg.StateAttribute, startIndex, err)
		}
		if len(blockOidsObject) == 0 {
			sfs.lastStateIndex = (startIndex - 1) * neofs.DefaultStateInterval
			return sfs.lastStateIndex, nil
		}
		startIndex++
	}
}

// Start begins state fetching.
func (sfs *Service) Start() error {
	if sfs.IsShutdown() {
		return errors.New("service shut down")
	}
	if !sfs.isActive.CompareAndSwap(false, true) {
		return nil
	}
	sfs.log.Info("state fetcher started")
	sfs.wg.Add(1)
	go sfs.run()
	return nil
}

// Shutdown stops the service.
func (sfs *Service) Shutdown() {
	if !sfs.isActive.CompareAndSwap(true, false) {
		return
	}
	sfs.basic.CtxCancel()
	sfs.wg.Wait()

	sfs.isShutdown.Store(true)
	if sfs.shutdownCallback != nil {
		sfs.shutdownCallback()
	}
	sfs.log.Info("state fetcher shutdown")
}

func (sfs *Service) run() {
	var (
		currentHeaderHeight = sfs.chain.HeaderHeight()
		syncHeight          uint32
		expectedRoot        util.Uint256
	)

	defer func() {
		sfs.isActive.Store(false)
		sfs.isShutdown.Store(true)
		sfs.basic.CtxCancel()
		sfs.wg.Done()
		if sfs.shutdownCallback != nil {
			sfs.shutdownCallback()
		}
		sfs.log.Info("state fetcher stopped")
	}()

	sfs.log.Info("checking state to fetch",
		zap.Uint32("headerHeight", currentHeaderHeight))

	reader, err := sfs.getStateObject()
	if err != nil {
		sfs.log.Error("failed to get state object", zap.Error(err))
		return
	}
	defer func() {
		if err := reader.Close(); err != nil {
			sfs.log.Warn("failed to close reader", zap.Error(err))
		}
	}()

	kvChan := make(chan kvPair, 1)
	go func() {
		defer close(kvChan)
		br := gio.NewBinReaderFromIO(reader)

		version := br.ReadB()
		if version != 0 || br.Err != nil {
			kvChan <- kvPair{err: fmt.Errorf("invalid state object version: %w", br.Err)}
			return
		}
		_ = br.ReadU32LE()
		syncHeight = br.ReadU32LE()
		br.ReadBytes(expectedRoot[:])
		if br.Err != nil {
			kvChan <- kvPair{err: fmt.Errorf("failed to read state root: %w", br.Err)}
			return
		}
		if syncHeight > currentHeaderHeight {
			kvChan <- kvPair{err: fmt.Errorf("sync height %d exceeds header height %d", syncHeight, currentHeaderHeight)}
			return
		}
		sfs.log.Info("state object",
			zap.String("root", expectedRoot.String()),
			zap.Uint32("height", syncHeight))
		for {
			select {
			case <-sfs.basic.Ctx.Done():
				kvChan <- kvPair{err: sfs.basic.Ctx.Err()}
				return
			default:
				key := br.ReadVarBytes()
				if br.Err != nil {
					kvChan <- kvPair{err: br.Err}
					return
				}
				value := br.ReadVarBytes()
				if br.Err != nil {
					kvChan <- kvPair{err: br.Err}
					return
				}
				kvChan <- kvPair{key: key, value: value}
			}
		}
	}()

	for {
		select {
		case <-sfs.basic.Ctx.Done():
			sfs.log.Debug("shutdown requested")
			return
		case pair, ok := <-kvChan:
			if !ok || pair.err != nil {
				if pair.err != nil && !errors.Is(pair.err, io.EOF) {
					sfs.log.Error("failed to read state stream", zap.Error(pair.err))
					return
				}
				sfs.log.Info("state fetch completed",
					zap.Uint32("height", syncHeight))
				return
			}

			if err := sfs.chain.AddContractStorageData(string(pair.key), pair.value, syncHeight, expectedRoot); err != nil {
				sfs.log.Error("failed to add storage pair",
					zap.String("key", string(pair.key)),
					zap.Error(err))
				return
			}
		}
	}
}

func (sfs *Service) getStateObject() (io.ReadCloser, error) {
	headerHeight := sfs.chain.HeaderHeight()
	var stateOidsObject []oid.ID
	var err error
	ctx, cancel := context.WithTimeout(sfs.basic.Ctx, sfs.cfg.Timeout)
	defer cancel()
	for i := headerHeight; true; i-- {
		sfs.log.Debug("searching state object", zap.Uint32("height", i))
		prm := client.PrmObjectSearch{}
		filters := object.NewSearchFilters()
		filters.AddFilter(sfs.cfg.StateAttribute, fmt.Sprintf("%d", i), object.MatchStringEqual)
		prm.SetFilters(filters)

		err = sfs.basic.Retry(func() error {
			stateOidsObject, err = neofs.ObjectSearch(ctx, sfs.basic.Pool, sfs.basic.Account.PrivateKey(), sfs.cfg.ContainerID, prm)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("failed to find '%s' object at index %d: %w", sfs.cfg.StateAttribute, i, err)
		}
		sfs.log.Debug("found objects", zap.Uint32("height", i), zap.Int("count", len(stateOidsObject)))
		if len(stateOidsObject) != 0 {
			break
		}
	}
	if len(stateOidsObject) == 0 {
		return nil, fmt.Errorf("no '%s' objects found", sfs.cfg.StateAttribute)
	}
	sfs.log.Debug("fetching object", zap.String("oid", stateOidsObject[0].String()))
	rc, err := sfs.objectGet(sfs.basic.Ctx, stateOidsObject[0].String())
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	return rc, nil
}

func (sfs *Service) objectGet(ctx context.Context, oid string) (io.ReadCloser, error) {
	u, err := url.Parse(fmt.Sprintf("%s:%s/%s", neofs.URIScheme, sfs.cfg.ContainerID, oid))
	if err != nil {
		return nil, err
	}
	var rc io.ReadCloser
	err = sfs.basic.Retry(func() error {
		rc, err = neofs.GetWithClient(ctx, sfs.basic.Pool, sfs.basic.Account.PrivateKey(), u, false)
		return err
	})
	return rc, err
}

// IsActive checks if the service is running.
func (sfs *Service) IsActive() bool {
	return sfs.isActive.Load() && !sfs.isShutdown.Load()
}

// IsShutdown checks if the service is shut down.
func (sfs *Service) IsShutdown() bool {
	return sfs.isShutdown.Load()
}
