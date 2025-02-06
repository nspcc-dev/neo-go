package blockfetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	gio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/bqueue"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	"github.com/nspcc-dev/neofs-sdk-go/container"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"github.com/nspcc-dev/neofs-sdk-go/pool"
	"github.com/nspcc-dev/neofs-sdk-go/user"
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

// poolWrapper wraps a NeoFS pool to adapt its Close method to return an error.
type poolWrapper struct {
	*pool.Pool
}

// Close closes the pool and returns nil.
func (p poolWrapper) Close() error {
	p.Pool.Close()
	return nil
}

type indexedOID struct {
	Index int
	OID   oid.ID
}

// Service is a service that fetches blocks from NeoFS.
type Service struct {
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
	pool    poolWrapper
	enqueue func(obj bqueue.Indexable) error
	account *wallet.Account

	oidsCh chan indexedOID
	// wg is a wait group for block downloaders.
	wg sync.WaitGroup

	// Global context for download operations cancellation.
	ctx       context.Context
	ctxCancel context.CancelFunc

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
}

// New creates a new BlockFetcher Service.
func New(chain Ledger, cfg config.NeoFSBlockFetcher, logger *zap.Logger, put func(item bqueue.Indexable) error, shutdownCallback func(), opt OperationMode) (*Service, error) {
	var (
		account *wallet.Account
		err     error
	)
	if !cfg.Enabled {
		return &Service{}, nil
	}
	if cfg.UnlockWallet.Path != "" {
		walletFromFile, err := wallet.NewWalletFromFile(cfg.UnlockWallet.Path)
		if err != nil {
			return nil, err
		}
		for _, acc := range walletFromFile.Accounts {
			if err := acc.Decrypt(cfg.UnlockWallet.Password, walletFromFile.Scrypt); err == nil {
				account = acc
				break
			}
		}
		if account == nil {
			return nil, errors.New("failed to decrypt any account in the wallet")
		}
	} else {
		account, err = wallet.NewAccount()
		if err != nil {
			return nil, err
		}
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

	params := pool.DefaultOptions()
	params.SetHealthcheckTimeout(neofs.DefaultHealthcheckTimeout)
	params.SetNodeDialTimeout(neofs.DefaultDialTimeout)
	params.SetNodeStreamTimeout(neofs.DefaultStreamTimeout)
	p, err := pool.New(pool.NewFlatNodeParams(cfg.Addresses), user.NewAutoIDSignerRFC6979(account.PrivateKey().PrivateKey), params)
	if err != nil {
		return nil, err
	}
	return &Service{
		chain:         chain,
		pool:          poolWrapper{Pool: p},
		log:           logger,
		cfg:           cfg,
		operationMode: opt,
		headerSizeMap: getHeaderSizeMap(chain.GetConfig()),

		enqueue:           put,
		account:           account,
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
	headerSizeMap[0] = block.GetExpectedHeaderSize(chain.StateRootInHeader, chain.GetNumOfCNs(0))
	for height := range chain.CommitteeHistory {
		headerSizeMap[int(height)] = block.GetExpectedHeaderSize(chain.StateRootInHeader, chain.GetNumOfCNs(height))
	}
	return headerSizeMap
}

// Start runs the NeoFS BlockFetcher service.
func (bfs *Service) Start() error {
	if bfs.IsShutdown() {
		return errors.New("service is already shut down")
	}
	if !bfs.isActive.CompareAndSwap(false, true) {
		return nil
	}
	bfs.log.Info("starting NeoFS BlockFetcher service")
	var (
		containerID  cid.ID
		containerObj container.Container
		err          error
	)
	bfs.ctx, bfs.ctxCancel = context.WithCancel(context.Background())
	if err = bfs.pool.Dial(context.Background()); err != nil {
		bfs.isActive.CompareAndSwap(true, false)
		return fmt.Errorf("failed to dial NeoFS pool: %w", err)
	}

	err = containerID.DecodeString(bfs.cfg.ContainerID)
	if err != nil {
		bfs.isActive.CompareAndSwap(true, false)
		return fmt.Errorf("failed to decode container ID: %w", err)
	}

	err = bfs.retry(func() error {
		containerObj, err = bfs.pool.ContainerGet(bfs.ctx, containerID, client.PrmContainerGet{})
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
		if !isContextCanceledErr(err) {
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
		index := indexedOid.Index
		blkOid := indexedOid.OID
		ctx, cancel := context.WithTimeout(bfs.ctx, bfs.cfg.Timeout)
		defer cancel()

		rc, err := bfs.getFunc(ctx, blkOid.String(), index)
		if err != nil {
			if isContextCanceledErr(err) {
				return
			}
			bfs.log.Error("failed to get object", zap.String("oid", blkOid.String()), zap.Error(err))
			bfs.stopService(true)
			return
		}

		obj, err := bfs.readFunc(rc)
		if err != nil {
			if isContextCanceledErr(err) {
				return
			}
			bfs.log.Error("failed to decode object from stream", zap.String("oid", blkOid.String()), zap.Error(err))
			bfs.stopService(true)
			return
		}
		select {
		case <-bfs.ctx.Done():
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

	for {
		select {
		case <-bfs.exiterToOIDDownloader:
			return nil
		default:
			prm := client.PrmObjectSearch{}
			filters := object.NewSearchFilters()
			filters.AddFilter(bfs.cfg.IndexFileAttribute, fmt.Sprintf("%d", startIndex), object.MatchStringEqual)
			filters.AddFilter("IndexSize", fmt.Sprintf("%d", bfs.cfg.IndexFileSize), object.MatchStringEqual)
			prm.SetFilters(filters)

			ctx, cancel := context.WithTimeout(bfs.ctx, bfs.cfg.Timeout)
			blockOidsObject, err := bfs.objectSearch(ctx, prm)
			cancel()
			if err != nil {
				if isContextCanceledErr(err) {
					return nil
				}
				return fmt.Errorf("failed to find '%s' object with index %d: %w", bfs.cfg.IndexFileAttribute, startIndex, err)
			}
			if len(blockOidsObject) == 0 {
				bfs.log.Info(fmt.Sprintf("NeoFS BlockFetcher service: no '%s' object found with index %d, stopping", bfs.cfg.IndexFileAttribute, startIndex))
				return nil
			}

			blockCtx, blockCancel := context.WithTimeout(bfs.ctx, bfs.cfg.Timeout)
			defer blockCancel()
			oidsRC, err := bfs.objectGet(blockCtx, blockOidsObject[0].String(), -1)
			if err != nil {
				if isContextCanceledErr(err) {
					return nil
				}
				return fmt.Errorf("failed to fetch '%s' object with index %d: %w", bfs.cfg.IndexFileAttribute, startIndex, err)
			}

			err = bfs.streamBlockOIDs(oidsRC, int(startIndex), int(skip))
			if err != nil {
				if isContextCanceledErr(err) {
					return nil
				}
				return fmt.Errorf("failed to stream block OIDs with index %d: %w", startIndex, err)
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

		select {
		case <-bfs.exiterToOIDDownloader:
			return nil
		case bfs.oidsCh <- indexedOID{Index: startIndex*int(bfs.cfg.IndexFileSize) + oidsProcessed, OID: oidBlock}:
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
	//We need to search with EQ filter to avoid partially-completed SEARCH responses.
	batchSize := uint32(neofs.DefaultSearchBatchSize)

	for {
		select {
		case <-bfs.exiterToOIDDownloader:
			return nil
		default:
			prm := client.PrmObjectSearch{}
			filters := object.NewSearchFilters()
			if startIndex == startIndex+batchSize-1 {
				filters.AddFilter(bfs.cfg.BlockAttribute, fmt.Sprintf("%d", startIndex), object.MatchStringEqual)
			} else {
				filters.AddFilter(bfs.cfg.BlockAttribute, fmt.Sprintf("%d", startIndex), object.MatchNumGE)
				filters.AddFilter(bfs.cfg.BlockAttribute, fmt.Sprintf("%d", startIndex+batchSize-1), object.MatchNumLE)
			}
			prm.SetFilters(filters)
			ctx, cancel := context.WithTimeout(bfs.ctx, bfs.cfg.Timeout)
			blockOids, err := bfs.objectSearch(ctx, prm)
			cancel()
			if err != nil {
				if isContextCanceledErr(err) {
					return nil
				}
				return err
			}

			if len(blockOids) == 0 {
				bfs.log.Info(fmt.Sprintf("NeoFS BlockFetcher service: no block found with index %d, stopping", startIndex))
				return nil
			}
			index := int(startIndex)
			for _, oid := range blockOids {
				select {
				case <-bfs.exiterToOIDDownloader:
					return nil
				case bfs.oidsCh <- indexedOID{Index: index, OID: oid}:
				}
				index++ //Won't work properly if neofs.ObjectSearch results are not ordered.
			}
			startIndex += batchSize
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
		zap.Bool("force", force),
	)

	bfs.isActive.CompareAndSwap(true, false)
	bfs.isShutdown.CompareAndSwap(false, true)
	// Cansel all pending OIDs/blocks downloads in case if shutdown requested by user
	// or caused by downloading error.
	if force {
		bfs.ctxCancel()
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
	_ = bfs.pool.Close()
	_ = bfs.log.Sync()
	bfs.shutdownCallback()

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

// retry function with exponential backoff.
func (bfs *Service) retry(action func() error) error {
	var (
		err     error
		backoff = neofs.InitialBackoff
		timer   = time.NewTimer(0)
	)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	for i := range neofs.MaxRetries {
		if err = action(); err == nil {
			return nil
		}
		if i == neofs.MaxRetries-1 {
			break
		}
		timer.Reset(backoff)

		select {
		case <-timer.C:
		case <-bfs.ctx.Done():
			return bfs.ctx.Err()
		}
		backoff *= time.Duration(neofs.BackoffFactor)
		if backoff > neofs.MaxBackoff {
			backoff = neofs.MaxBackoff
		}
	}
	return err
}

func (bfs *Service) objectGet(ctx context.Context, oid string, index int) (io.ReadCloser, error) {
	u, err := url.Parse(fmt.Sprintf("%s:%s/%s", neofs.URIScheme, bfs.cfg.ContainerID, oid))
	if err != nil {
		return nil, err
	}
	var rc io.ReadCloser
	err = bfs.retry(func() error {
		rc, err = neofs.GetWithClient(ctx, bfs.pool, bfs.account.PrivateKey(), u, false)
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
	err = bfs.retry(func() error {
		rc, err = neofs.GetWithClient(ctx, bfs.pool, bfs.account.PrivateKey(), u, false)
		return err
	})
	return rc, err
}

func (bfs *Service) objectSearch(ctx context.Context, prm client.PrmObjectSearch) ([]oid.ID, error) {
	var (
		oids []oid.ID
		err  error
	)
	err = bfs.retry(func() error {
		oids, err = neofs.ObjectSearch(ctx, bfs.pool, bfs.account.PrivateKey(), bfs.cfg.ContainerID, prm)
		return err
	})
	return oids, err
}

// isContextCanceledErr returns whether error is a wrapped [context.Canceled].
// Ref. https://github.com/nspcc-dev/neofs-sdk-go/issues/624.
func isContextCanceledErr(err error) bool {
	return errors.Is(err, context.Canceled) ||
		strings.Contains(err.Error(), "context canceled")
}
