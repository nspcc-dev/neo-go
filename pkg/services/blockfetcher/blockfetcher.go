package blockfetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	cli "github.com/nspcc-dev/neo-go/cli/server"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	gio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle/neofs"
	"github.com/nspcc-dev/neofs-sdk-go/client"
	cid "github.com/nspcc-dev/neofs-sdk-go/container/id"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	oid "github.com/nspcc-dev/neofs-sdk-go/object/id"
	"go.uber.org/zap"
)

type Service struct {
	chain       *core.Blockchain
	log         *zap.Logger
	client      *client.Client
	db          storage.Store
	quit        chan bool
	containerID cid.ID
	Timeout     time.Duration
	Nodes       []string
	dumpDir     string
}

// ConfigObject represents the configuration object in NeoFS.
type ConfigObject struct {
	HashOID   string   `json:"hash_oid"`
	BlockOIDs []string `json:"block_oid"`
	Height    uint32   `json:"height"`
	Timestamp int64    `json:"timestamp"`
	Step      uint32   `json:"step"`
}

// Config is the configuration of the BlockFetcherService.
type Config struct {
	ContainerID string
	Nodes       []string
	Timeout     time.Duration
	DumpDir     string
}

// New creates a new BlockFetcherService.
func New(chain *core.Blockchain, store storage.Store, cfg Config, logger *zap.Logger) *Service {
	neofsClient, err := client.New(client.PrmInit{})
	if err != nil {
		logger.Error("Failed to create NeoFS client", zap.Error(err))
		return nil
	}
	var containerID cid.ID
	err = containerID.DecodeString(cfg.ContainerID)
	if err != nil {
		logger.Error("Failed to decode container ID", zap.Error(err))
		return nil
	}
	return &Service{
		chain:       chain,
		log:         logger,
		client:      neofsClient,
		db:          store,
		quit:        make(chan bool),
		dumpDir:     cfg.DumpDir,
		containerID: containerID,
		Nodes:       cfg.Nodes,
		Timeout:     cfg.Timeout,
	}
}

// Name implements the core.Service interface.
func (bfs *Service) Name() string {
	return "BlockFetcherService"
}

// Start implements the core.Service interface.
func (bfs *Service) Start() {
	bfs.log.Info("Starting Block Fetcher Service")
	err := bfs.fetchData()
	if err != nil {
		close(bfs.quit)
		return
	}
}

// Shutdown implements the core.Service interface.
func (bfs *Service) Shutdown() {
	bfs.log.Info("Shutting down Block Fetcher Service")
	close(bfs.quit)
}

func (bfs *Service) fetchData() error {
	for {
		select {
		case <-bfs.quit:
			return nil
		default:
			prm := client.PrmObjectSearch{}
			filters := object.NewSearchFilters()
			filters.AddFilter("type", "config", object.MatchStringEqual)
			prm.SetFilters(filters)

			configOid, err := bfs.search(prm)
			if err != nil {
				bfs.log.Error("Failed to fetch object IDs", zap.Error(err))
				return err
			}
			cfg, err := bfs.get(configOid[0].String())
			var configObj ConfigObject
			err = json.Unmarshal(cfg, &configObj)
			if err != nil {
				bfs.log.Error("Failed to unmarshal configuration data", zap.Error(err))
				return err
			}
			_, err = bfs.get(configObj.HashOID)
			if err != nil {
				bfs.log.Error("Failed to fetch hash", zap.Error(err))
				return err
			}
			for _, blockOID := range configObj.BlockOIDs {
				data, err := bfs.get(blockOID)
				if err != nil {
					bfs.log.Error(fmt.Sprintf("Failed to fetch block %s", blockOID), zap.Error(err))
					return err
				}
				err = bfs.processBlock(data, configObj.Step)
				if err != nil {
					bfs.log.Error(fmt.Sprintf("Failed to process block %s", blockOID), zap.Error(err))
					return err
				}
			}
		}
		close(bfs.quit)
		return nil
	}
}

func (bfs *Service) processBlock(data []byte, count uint32) error {
	br := gio.NewBinReaderFromBuf(data)
	dump := cli.NewDump()
	var lastIndex uint32

	err := chaindump.Restore(bfs.chain, br, 0, count, func(b *block.Block) error {
		batch := bfs.chain.LastBatch()
		if batch != nil {
			dump.Add(b.Index, batch)
			lastIndex = b.Index
			if b.Index%1000 == 0 {
				if err := dump.TryPersist(bfs.dumpDir, lastIndex); err != nil {
					return fmt.Errorf("can't dump storage to file: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to restore blocks: %w", err)
	}

	if err = dump.TryPersist(bfs.dumpDir, lastIndex); err != nil {
		return fmt.Errorf("final persistence failed: %w", err)
	}
	return nil
}

func (bfs *Service) get(oid string) ([]byte, error) {
	privateKey, err := keys.NewPrivateKey()
	ctx, cancel := context.WithTimeout(context.Background(), bfs.Timeout)
	defer cancel()
	u, err := url.Parse(fmt.Sprintf("neofs:%s/%s", bfs.containerID, oid))
	if err != nil {
		return nil, err
	}
	rc, err := neofs.Get(ctx, bfs.client, privateKey, u, bfs.Nodes[0])
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (bfs *Service) search(prm client.PrmObjectSearch) ([]oid.ID, error) {
	privateKey, err := keys.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), bfs.Timeout)
	defer cancel()
	return neofs.ObjectSearch(ctx, bfs.client, privateKey, bfs.containerID, bfs.Nodes[0], prm)
}
