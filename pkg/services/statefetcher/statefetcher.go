package statefetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	gio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/nspcc-dev/neo-go/pkg/util"
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

const batchSize = 1000

// Ledger is an interface to Blockchain sufficient for Service.
type Ledger interface {
	GetConfig() config.Blockchain
	HeaderHeight() uint32
}

// Service handles state fetching and MPT updates from NeoFS.
type Service struct {
	// isActive is an atomic flag that indicates whether the service is running.
	isActive atomic.Bool
	cfg      config.NeoFSBlockFetcher
	// Global context for download operations cancellation.
	ctx       context.Context
	ctxCancel context.CancelFunc

	chain       Ledger
	stateModule *stateroot.Module
	pool        neofs.PoolWrapper
	account     *wallet.Account
	log         *zap.Logger
}

// New creates a new Service instance.
func New(chain Ledger, stateModule *stateroot.Module, cfg config.NeoFSBlockFetcher, logger *zap.Logger) (*Service, error) {
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
	bfs := Service{
		pool:        neofs.PoolWrapper{Pool: p},
		log:         logger,
		cfg:         cfg,
		account:     account,
		chain:       chain,
		stateModule: stateModule,
	}

	return &bfs, nil
}

// Start begins the state fetching and MPT update process.
func (bfs *Service) Start() error {
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
	currentHeight := bfs.chain.HeaderHeight()
	var lastSavedRoot *state.MPTRoot
	var lastSavedHeight uint32
	for h := currentHeight; h > 0; h-- {
		root, err := bfs.stateModule.GetStateRoot(h)
		if err == nil {
			lastSavedRoot = root
			lastSavedHeight = h
			break
		}
	}
	if lastSavedRoot != nil {
		bfs.log.Info("found last saved state root in store",
			zap.String("root", lastSavedRoot.Root.String()),
			zap.Uint32("height", lastSavedHeight))
	} else {
		bfs.log.Info("no saved state root found in store up to current height",
			zap.Uint32("currentHeight", currentHeight))
	}
	if lastSavedHeight == currentHeight-1 {
		bfs.log.Info("no new state root to fetch, exiting")
		return nil
	}
	reader, err := bfs.getMPTNodes()
	if err != nil {
		bfs.log.Error("failed to get MPT nodes", zap.Error(err))
		return err
	}
	defer reader.Close()

	br := gio.NewBinReaderFromIO(reader)

	version := br.ReadB()
	if version != 0 || br.Err != nil {
		return fmt.Errorf("invalid version: %d, error: %w", version, br.Err)
	}
	_ = br.ReadU32LE()
	syncHeight := br.ReadU32LE()
	var stateRoot util.Uint256
	br.ReadBytes(stateRoot[:])
	if br.Err != nil {
		return fmt.Errorf("failed to read NeoFS header: %w", br.Err)
	}
	if syncHeight > currentHeight {
		return fmt.Errorf("sync height %d exceeds current height %d", syncHeight, currentHeight)
	}

	bfs.log.Info("NeoFS state root", zap.String("root", stateRoot.String()), zap.Uint32("height", syncHeight))

	localMPT := mpt.NewTrie(mpt.EmptyNode{}, mpt.ModeAll, bfs.stateModule.Store)
	bfs.log.Info("initial MPT root", zap.String("root", localMPT.StateRoot().String()))
	batch := make(map[string][]byte, batchSize)
	processed := 0
	cache := storage.NewMemCachedStore(bfs.stateModule.Store)
	tempPrefix := []byte{byte(storage.STTempStorage)}

	for {
		neoFSKey := br.ReadVarBytes()
		if br.Err != nil {
			if errors.Is(br.Err, io.EOF) {
				break
			}
			bfs.log.Error("failed to read key", zap.Error(br.Err))
			return br.Err
		}
		value := br.ReadVarBytes()
		if br.Err != nil {
			bfs.log.Error("failed to read value", zap.Error(br.Err))
			return br.Err
		}

		fullKey := make([]byte, len(tempPrefix)+len(neoFSKey))
		copy(fullKey, tempPrefix)
		copy(fullKey[len(tempPrefix):], neoFSKey)
		key := neoFSKey
		if processed < 5 {
			bfs.log.Debug("key sample", zap.ByteString("fullKey", fullKey), zap.ByteString("mptKey", key), zap.ByteString("value", value))
		}

		cache.Put(fullKey, value)
		batch[string(key)] = value
		processed++

		if len(batch) >= batchSize {
			if err := bfs.processMPTBatch(localMPT, batch, syncHeight, cache); err != nil {
				bfs.log.Error("failed to process MPT batch", zap.Error(err))
				return err
			}
			if n, err := cache.PersistSync(); err != nil {
				bfs.log.Error("failed to persist cache", zap.Error(err), zap.Int("items", n))
				return err
			}
			bfs.log.Info("batch root", zap.String("root", localMPT.StateRoot().String()))
			batch = make(map[string][]byte, batchSize)
			cache = storage.NewMemCachedStore(bfs.stateModule.Store)
			bfs.log.Info("processed and persisted MPT batch", zap.Int("items", processed))
		}
	}

	if len(batch) > 0 {
		if err := bfs.processMPTBatch(localMPT, batch, syncHeight, cache); err != nil {
			bfs.log.Error("failed to process final MPT batch", zap.Error(err))
			return err
		}
		if n, err := cache.PersistSync(); err != nil {
			bfs.log.Error("failed to persist final cache", zap.Error(err), zap.Int("items", n))
			return err
		}
	}

	computedRoot := localMPT.StateRoot()
	bfs.log.Info("computed MPT root", zap.String("root", computedRoot.String()))

	if !computedRoot.Equals(stateRoot) {
		bfs.log.Error("state root mismatch",
			zap.String("computed", computedRoot.String()),
			zap.String("neofs", stateRoot.String()))
		return fmt.Errorf("state root mismatch: expected %s, got %s", stateRoot, computedRoot)
	}
	if err := bfs.finalizeState(syncHeight, cache); err != nil {
		bfs.log.Error("failed to finalize state", zap.Error(err))
		return err
	}

	bfs.log.Info("completed MPT state sync",
		zap.Uint32("height", syncHeight),
		zap.Int("total_items", processed),
		zap.String("root", computedRoot.String()))
	return nil
}

func (bfs *Service) processMPTBatch(trie *mpt.Trie, changes map[string][]byte, height uint32, cache *storage.MemCachedStore) error {
	batch := mpt.MapToMPTBatch(changes)
	trie.Store = cache
	if _, err := trie.PutBatch(batch); err != nil {
		return fmt.Errorf("failed to apply MPT batch at height %d: %w", height, err)
	}
	trie.Flush(height)
	bfs.log.Info("applied MPT batch", zap.Int("items", len(changes)), zap.String("new_root", trie.StateRoot().String()))
	return nil
}
func (bfs *Service) getMPTNodes() (io.ReadCloser, error) {
	headerHeight := bfs.chain.HeaderHeight() - 1
	var stateOidsObject []oid.ID
	var err error
	ctx, cancel := context.WithTimeout(bfs.ctx, bfs.cfg.Timeout)
	defer cancel()
	for i := headerHeight; true; i-- {
		bfs.log.Info("Searching MPT nodes", zap.Uint32("height", i))
		prm := client.PrmObjectSearch{}
		filters := object.NewSearchFilters()
		filters.AddFilter(neofs.DefaultStateAttribute, fmt.Sprintf("%d", i), object.MatchStringEqual)
		prm.SetFilters(filters)

		err = bfs.retry(func() error {
			stateOidsObject, err = neofs.ObjectSearch(ctx, bfs.pool, bfs.account.PrivateKey(), bfs.cfg.ContainerID, prm)
			return err
		})
		if err != nil {
			return nil, fmt.Errorf("failed to find '%s' object with index %d: %w", neofs.DefaultStateAttribute, i, err)
		}
		bfs.log.Info("Found objects for height", zap.Uint32("height", i), zap.Int("count", len(stateOidsObject)))

		if len(stateOidsObject) != 0 {
			break
		}
	}
	if len(stateOidsObject) == 0 {
		return nil, fmt.Errorf("failed to find '%s' objects: %w", neofs.DefaultStateAttribute, err)
	}
	bfs.log.Info("Fetching object", zap.String("oid", stateOidsObject[0].String()))
	rc, err := bfs.objectGet(bfs.ctx, stateOidsObject[0].String())
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	return rc, nil
}

func (bfs *Service) objectGet(ctx context.Context, oid string) (io.ReadCloser, error) {
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

// IsActive returns true if the NeoFS StateFetcher service is running.
func (bfs *Service) IsActive() bool {
	return bfs.isActive.Load()
}

func (bfs *Service) finalizeState(height uint32, cache *storage.MemCachedStore) error {
	bfs.log.Info("finalizing state: moving STTempStorage to STStorage", zap.Uint32("height", height))
	tempPrefix := []byte{byte(storage.STTempStorage)}
	permPrefix := []byte{byte(storage.STStorage)}

	bfs.stateModule.Store.Seek(storage.SeekRange{Prefix: tempPrefix}, func(k, v []byte) bool {
		newKey := append(permPrefix, k[1:]...)
		cache.Put(newKey, v)
		cache.Delete(k)
		return true
	})

	if n, err := cache.PersistSync(); err != nil {
		return fmt.Errorf("failed to persist finalized state: %w (items: %d)", err, n)
	}

	bfs.log.Info("state finalized successfully", zap.Uint32("height", height))
	return nil
}
