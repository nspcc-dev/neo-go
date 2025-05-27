package blockfetcher

//go:generate stringer -type=OperationMode

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
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	gio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/bqueue"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"go.uber.org/zap"
)

// OperationMode is an enum that denotes the operation mode of the Fetcher.
// It can be either Blocks or Headers.
type OperationMode byte

const (
	// Blocks denotes that the Fetcher is working with blocks.
	Blocks OperationMode = iota
	// Headers denotes that the Fetcher is working with headers.
	Headers
)

const (
	// DefaultQueueCacheSize is the default size of the queue cache.
	DefaultQueueCacheSize = 16000
)

// Ledger is an interface to Blockchain sufficient for Service.
type Ledger interface {
	GetConfig() config.Blockchain
	BlockHeight() uint32
	HeaderHeight() uint32
}

type indexedOID struct {
	Index uint32
	OID   oid.ID
}

// Service is a service that fetches blocks from NeoFS.
type Service struct {
	neofs.BasicService
	// isActive denotes whether the service is working or in the process of shutdown.
	isActive      atomic.Bool
	isShutdown    atomic.Bool
	log           *zap.Logger
	cfg           config.NeoFSBlockFetcher
	operationMode OperationMode

	stateRootInHeader bool
	// headerSizeMap is a map of height to expected header size.
	headerSizeMap map[int]int

	chain   Ledger
	enqueue func(obj bqueue.Indexable) error

	oidsCh chan indexedOID
	// wg is a wait group for block downloaders.
	wg sync.WaitGroup

	// A set of routines managing graceful Service shutdown.
	quit                  chan bool
	quitOnce              sync.Once
	exiterToOIDDownloader chan struct{}
	exiterToShutdown      chan struct{}
	oidDownloaderToExiter chan struct{}

	shutdownCallback func()

	// Depends on the OperationMode, the following functions are set to the appropriate functions.
	getFunc    func(ctx context.Context, oid string, index int) (io.ReadCloser, error)
	readFunc   func(rc io.ReadCloser) (any, error)
	heightFunc func() uint32

	// stopAt is the height at which the service will stop fetching objects.
	stopAt uint32
}

// New creates a new BlockFetcher Service.
func New(chain Ledger, cfg config.NeoFSBlockFetcher, logger *zap.Logger, put func(item bqueue.Indexable) error, shutdownCallback func(), opt OperationMode) (*Service, error) {
	if !cfg.Enabled {
		return &Service{}, nil
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = neofs.DefaultTimeout
	}
	if cfg.OIDBatchSize <= 0 {
		cfg.OIDBatchSize = cfg.BQueueSize / 2
	}
	if cfg.DownloaderWorkersCount <= 0 {
		cfg.DownloaderWorkersCount = neofs.DefaultDownloaderWorkersCount
	}
	if cfg.IndexFileSize <= 0 {
		cfg.IndexFileSize = neofs.DefaultIndexFileSize
	}
	if cfg.BlockAttribute == "" {
		cfg.BlockAttribute = neofs.DefaultBlockAttribute
	}
	if cfg.IndexFileAttribute == "" {
		cfg.IndexFileAttribute = neofs.DefaultIndexFileAttribute
	}

	basic, err := neofs.NewBasicService(cfg.NeoFSService)
	if err != nil {
		return nil, fmt.Errorf("failed to create NeoFS service: %w", err)
	}
	return &Service{
		BasicService:  basic,
		chain:         chain,
		log:           logger,
		cfg:           cfg,
		operationMode: opt,
		headerSizeMap: getHeaderSizeMap(chain.GetConfig()),

		enqueue:           put,
		stateRootInHeader: chain.GetConfig().StateRootInHeader,
		shutdownCallback:  shutdownCallback,

		quit:                  make(chan bool),
		exiterToOIDDownloader: make(chan struct{}),
		exiterToShutdown:      make(chan struct{}),
		oidDownloaderToExiter: make(chan struct{}),

		// Use buffer of two batch sizes to load OIDs in advance:
		//  * first full block of OIDs is processing by Downloader
		//  * second full block of OIDs is available to be fetched by Downloader immediately
		//  * third half-filled block of OIDs is being collected by OIDsFetcher.
		oidsCh: make(chan indexedOID, 2*cfg.OIDBatchSize),
	}, nil
}

func getHeaderSizeMap(chain config.Blockchain) map[int]int {
	headerSizeMap := make(map[int]int)
	headerSizeMap[0] = block.GetExpectedHeaderSize(chain.StateRootInHeader, 0) // genesis header size.
	headerSizeMap[1] = block.GetExpectedHeaderSize(chain.StateRootInHeader, chain.GetNumOfCNs(0))
	for height := range chain.CommitteeHistory {
		headerSizeMap[int(height)] = block.GetExpectedHeaderSize(chain.StateRootInHeader, chain.GetNumOfCNs(height))
	}
	return headerSizeMap
}

// Start runs the NeoFS BlockFetcher service.
func (bfs *Service) Start(stopAt ...uint32) error {
	if bfs.IsShutdown() {
		return errors.New("service is already shut down")
	}
	if !bfs.isActive.CompareAndSwap(false, true) {
		return nil
	}
	bfs.log.Info("starting NeoFS BlockFetcher service", zap.String("mode", bfs.operationMode.String()))
	var (
		containerObj container.Container
		err          error
	)
	bfs.Ctx, bfs.CtxCancel = context.WithCancel(context.Background())
	if err = bfs.Pool.Dial(context.Background()); err != nil {
		bfs.isActive.CompareAndSwap(true, false)
		return fmt.Errorf("failed to dial NeoFS pool: %w", err)
	}

	err = bfs.Retry(func() error {
		containerObj, err = bfs.Pool.ContainerGet(bfs.Ctx, bfs.ContainerID, client.PrmContainerGet{})
		return err
	})
	if err != nil {
		bfs.isActive.CompareAndSwap(true, false)
		return fmt.Errorf("failed to get container: %w", err)
	}
	containerMagic := containerObj.Attribute("Magic")
	if containerMagic != strconv.Itoa(int(bfs.chain.GetConfig().Magic)) {
		bfs.isActive.CompareAndSwap(true, false)
		return fmt.Errorf("container magic mismatch: expected %d, got %s", bfs.chain.GetConfig().Magic, containerMagic)
	}

	bfs.getFunc = bfs.objectGet
	bfs.readFunc = bfs.readBlock
	bfs.heightFunc = bfs.chain.BlockHeight
	if bfs.operationMode == Headers {
		bfs.getFunc = bfs.objectGetRange
		bfs.readFunc = bfs.readHeader
		bfs.heightFunc = bfs.chain.HeaderHeight
	}
	if len(stopAt) > 0 {
		bfs.stopAt = stopAt[0]
	}

	// Start routine that manages Service shutdown process.
	go bfs.exiter()

	// Start OIDs downloader routine.
	go bfs.oidDownloader()

	// Start the set of blocks downloading routines.
	for range bfs.cfg.DownloaderWorkersCount {
		bfs.wg.Add(1)
		go bfs.blockDownloader()
	}
	return nil
}

// oidDownloader runs the appropriate blocks OID fetching method based on the configuration.
func (bfs *Service) oidDownloader() {
	defer close(bfs.oidDownloaderToExiter)

	var err error
	if bfs.cfg.SkipIndexFilesSearch {
		err = bfs.fetchOIDsBySearch()
	} else {
		err = bfs.fetchOIDsFromIndexFiles()
	}
	var force bool
	if err != nil {
		if !neofs.IsContextCanceledErr(err) {
			bfs.log.Error("NeoFS BlockFetcher service: OID downloading routine failed", zap.Error(err))
		}
		force = true
	}
	// Stop the service since there's nothing to do anymore.
	bfs.stopService(force)
}

// blockDownloader downloads the block from NeoFS and sends it to the blocks channel.
func (bfs *Service) blockDownloader() {
	defer bfs.wg.Done()

	for indexedOid := range bfs.oidsCh {
		var (
			blkOid = indexedOid.OID
			index  = indexedOid.Index
			obj    any
		)
		err := bfs.Retry(func() error {
			ctx, cancel := context.WithTimeout(bfs.Ctx, bfs.cfg.Timeout)
			defer cancel()

			rc, err := bfs.getFunc(ctx, blkOid.String(), int(index))
			if err != nil {
				if neofs.IsContextCanceledErr(err) {
					return nil
				}
				return err
			}
			obj, err = bfs.readFunc(rc)
			if err != nil {
				if neofs.IsContextCanceledErr(err) {
					return nil
				}
				return err
			}
			return nil
		})
		if err != nil {
			if neofs.IsContextCanceledErr(err) {
				return
			}
			bfs.log.Error("failed to get object", zap.String("oid", blkOid.String()), zap.Error(err))
			bfs.stopService(true)
			return
		}
		select {
		case <-bfs.Ctx.Done():
			return
		default:
			err = bfs.enqueue(obj.(bqueue.Indexable))
			if err != nil {
				bfs.log.Error("failed to enqueue object", zap.String("oid", blkOid.String()), zap.Error(err))
				bfs.stopService(true)
				return
			}
		}
	}
}

// fetchOIDsFromIndexFiles fetches block OIDs from NeoFS by searching index files first.
func (bfs *Service) fetchOIDsFromIndexFiles() error {
	h := bfs.heightFunc()
	startIndex := h / bfs.cfg.IndexFileSize
	skip := h % bfs.cfg.IndexFileSize

	if bfs.stopAt > 0 && h >= bfs.stopAt {
		return nil
	}

	for {
		select {
		case <-bfs.exiterToOIDDownloader:
			return nil
		default:
			filters := object.NewSearchFilters()
			filters.AddFilter(bfs.cfg.IndexFileAttribute, fmt.Sprintf("%d", startIndex), object.MatchStringEqual)
			filters.AddFilter("IndexSize", fmt.Sprintf("%d", bfs.cfg.IndexFileSize), object.MatchStringEqual)

			ctx, cancel := context.WithTimeout(bfs.Ctx, bfs.cfg.Timeout)
			resultsChan, errChan := neofs.ObjectSearch(ctx, bfs.Pool, bfs.Account.PrivateKey(), bfs.ContainerID, filters, []string{bfs.cfg.IndexFileAttribute})

			var obj *client.SearchResultItem

		loop:
			for {
				select {
				case item, ok := <-resultsChan:
					if !ok {
						break loop
					}
					obj = &item
					break loop
				case err := <-errChan:
					if err != nil && !neofs.IsContextCanceledErr(err) {
						cancel()
						return fmt.Errorf("failed to find '%s' object with index %d: %w", bfs.cfg.IndexFileAttribute, startIndex, err)
					}
					break loop
				case <-bfs.exiterToOIDDownloader:
					cancel()
					return nil
				}
			}
			cancel()

			if obj == nil {
				bfs.log.Info(fmt.Sprintf("NeoFS BlockFetcher service: no '%s' object found with index %d, stopping", bfs.cfg.IndexFileAttribute, startIndex))
				return nil
			}

			blockCtx, blockCancel := context.WithTimeout(bfs.Ctx, bfs.cfg.Timeout)
			oidsRC, err := bfs.objectGet(blockCtx, obj.ID.String(), -1)
			if err != nil {
				blockCancel()
				if neofs.IsContextCanceledErr(err) {
					return nil
				}
				return fmt.Errorf("failed to fetch '%s' object with index %d: %w", bfs.cfg.IndexFileAttribute, startIndex, err)
			}

			err = bfs.streamBlockOIDs(oidsRC, int(startIndex), int(skip))
			blockCancel()
			if err != nil {
				if neofs.IsContextCanceledErr(err) {
					return nil
				}
				return fmt.Errorf("failed to stream block OIDs with index %d: %w", startIndex, err)
			}
			if bfs.stopAt > 0 && startIndex >= bfs.stopAt/bfs.cfg.IndexFileSize {
				return nil
			}
			startIndex++
			skip = 0
		}
	}
}

// streamBlockOIDs reads block OIDs from the read closer and sends them to the OIDs channel.
func (bfs *Service) streamBlockOIDs(rc io.ReadCloser, startIndex, skip int) error {
	defer rc.Close()
	oidBytes := make([]byte, oid.Size)
	oidsProcessed := 0

	for {
		_, err := io.ReadFull(rc, oidBytes)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read OID: %w", err)
		}

		if oidsProcessed < skip {
			oidsProcessed++
			continue
		}

		var oidBlock oid.ID
		if err := oidBlock.Decode(oidBytes); err != nil {
			return fmt.Errorf("failed to decode OID: %w", err)
		}
		index := uint32(startIndex*int(bfs.cfg.IndexFileSize) + oidsProcessed)
		select {
		case <-bfs.exiterToOIDDownloader:
			return nil
		case bfs.oidsCh <- indexedOID{Index: index, OID: oidBlock}:
		}

		if bfs.stopAt > 0 && index == bfs.stopAt {
			return nil
		}

		oidsProcessed++
	}
	if oidsProcessed != int(bfs.cfg.IndexFileSize) {
		return fmt.Errorf("block OIDs count mismatch: expected %d, processed %d", bfs.cfg.IndexFileSize, oidsProcessed)
	}
	return nil
}

// fetchOIDsBySearch fetches block OIDs from NeoFS by searching through the Block objects.
func (bfs *Service) fetchOIDsBySearch() error {
	startIndex := bfs.heightFunc()

	filters := object.NewSearchFilters()
	filters.AddFilter(bfs.cfg.BlockAttribute, fmt.Sprintf("%d", startIndex), object.MatchNumGE)

	ctx, cancel := context.WithTimeout(bfs.Ctx, bfs.cfg.Timeout)
	defer cancel()

	results, errs := neofs.ObjectSearch(ctx, bfs.Pool, bfs.Account.PrivateKey(), bfs.ContainerID, filters, []string{bfs.cfg.BlockAttribute})
	var lastIndex uint64
	for {
		select {
		case <-bfs.exiterToOIDDownloader:
			return nil
		case item, ok := <-results:
			if !ok {
				if err, ok := <-errs; ok && err != nil && !neofs.IsContextCanceledErr(err) {
					return err
				}
				return nil
			}
			if len(item.Attributes) == 0 {
				return fmt.Errorf("search result item %s has no attributes %s", item.ID, bfs.cfg.BlockAttribute)
			}
			indexStr := item.Attributes[0]
			index, err := strconv.ParseUint(indexStr, 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse block index %q: %w", indexStr, err)
			}
			if index > uint64(bfs.stopAt) && bfs.stopAt > 0 {
				return nil
			}
			if index <= lastIndex {
				continue
			}
			lastIndex = index

			select {
			case <-bfs.exiterToOIDDownloader:
				return nil
			case bfs.oidsCh <- indexedOID{Index: uint32(index), OID: item.ID}:
			}
		}
	}
}

// readBlock decodes the block from the read closer and prepares it for adding to the blockchain.
func (bfs *Service) readBlock(rc io.ReadCloser) (any, error) {
	b := block.New(bfs.stateRootInHeader)
	r := gio.NewBinReaderFromIO(rc)
	b.DecodeBinary(r)
	rc.Close()
	return b, r.Err
}

// readHeader decodes the header from the read closer and prepares it for adding to the blockchain.
func (bfs *Service) readHeader(rc io.ReadCloser) (any, error) {
	b := block.New(bfs.stateRootInHeader)
	r := gio.NewBinReaderFromIO(rc)
	b.Header.DecodeBinary(r)
	rc.Close()
	return &b.Header, r.Err
}

// Shutdown stops the NeoFS BlockFetcher service. It prevents service from new
// block OIDs search, cancels all in-progress downloading operations and waits
// until all service routines finish their work.
func (bfs *Service) Shutdown() {
	if !bfs.IsActive() || bfs.IsShutdown() {
		return
	}
	bfs.stopService(true)
	<-bfs.exiterToShutdown
}

// stopService close quitting goroutine once. It's the only entrypoint to shutdown
// procedure.
func (bfs *Service) stopService(force bool) {
	bfs.quitOnce.Do(func() {
		bfs.quit <- force
		close(bfs.quit)
	})
}

// exiter is a routine that is listening to a quitting signal and manages graceful
// Service shutdown process.
func (bfs *Service) exiter() {
	if !bfs.isActive.Load() {
		return
	}
	// Closing signal may come from anyone, but only once.
	force := <-bfs.quit
	bfs.log.Info("shutting down NeoFS BlockFetcher service",
		zap.Bool("force", force), zap.String("mode", bfs.operationMode.String()))

	bfs.isActive.CompareAndSwap(true, false)
	bfs.isShutdown.CompareAndSwap(false, true)
	// Cansel all pending OIDs/blocks downloads in case if shutdown requested by user
	// or caused by downloading error.
	if force {
		bfs.CtxCancel()
	}

	// Send signal to OID downloader to stop. Wait until OID downloader finishes his
	// work.
	close(bfs.exiterToOIDDownloader)
	<-bfs.oidDownloaderToExiter

	// Close OIDs channel to let block downloaders know that there are no more OIDs
	// expected. Wait until all downloaders finish their work.
	close(bfs.oidsCh)
	bfs.wg.Wait()

	// Everything is done, release resources, turn off the activity marker and let
	// the server know about it.
	_ = bfs.Pool.Close()
	_ = bfs.log.Sync()
	if bfs.shutdownCallback != nil {
		bfs.shutdownCallback()
	}

	// Notify Shutdown routine in case if it's user-triggered shutdown.
	close(bfs.exiterToShutdown)
}

// IsShutdown returns true if the NeoFS BlockFetcher service is completely shutdown.
// The service can not be started again.
func (bfs *Service) IsShutdown() bool {
	return bfs.isShutdown.Load()
}

// IsActive returns true if the NeoFS BlockFetcher service is running.
func (bfs *Service) IsActive() bool {
	return bfs.isActive.Load()
}

func (bfs *Service) objectGet(ctx context.Context, oid string, index int) (io.ReadCloser, error) {
	u, err := url.Parse(fmt.Sprintf("%s:%s/%s", neofs.URIScheme, bfs.cfg.ContainerID, oid))
	if err != nil {
		return nil, err
	}
	var rc io.ReadCloser
	err = bfs.Retry(func() error {
		rc, err = neofs.GetWithClient(ctx, bfs.Pool, bfs.Account.PrivateKey(), u, false)
		return err
	})
	return rc, err
}

func (bfs *Service) objectGetRange(ctx context.Context, oid string, height int) (io.ReadCloser, error) {
	nearestHeight := 0
	for h := range bfs.headerSizeMap {
		if h <= height && h > nearestHeight {
			nearestHeight = h
		}
		if nearestHeight >= height {
			break
		}
	}

	size := bfs.headerSizeMap[nearestHeight]
	u, err := url.Parse(fmt.Sprintf("%s:%s/%s/%s/%d|%d", neofs.URIScheme, bfs.cfg.ContainerID, oid, "range", 0, size))
	if err != nil {
		return nil, err
	}
	var rc io.ReadCloser
	err = bfs.Retry(func() error {
		rc, err = neofs.GetWithClient(ctx, bfs.Pool, bfs.Account.PrivateKey(), u, false)
		return err
	})
	return rc, err
}
