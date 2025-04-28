package blockfetcher

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/network/bqueue"
	"github.com/nspcc-dev/neo-go/pkg/services/helpers/neofs"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockLedger struct {
	height uint32
}

func (m *mockLedger) HeaderHeight() uint32 {
	return m.height
}

func (m *mockLedger) GetConfig() config.Blockchain {
	return config.Blockchain{}
}

func (m *mockLedger) BlockHeight() uint32 {
	return m.height
}

type mockPutBlockFunc struct {
	putCalled bool
}

func (m *mockPutBlockFunc) putBlock(b bqueue.Indexable) error {
	m.putCalled = true
	return nil
}

func TestServiceConstructor(t *testing.T) {
	logger := zap.NewNop()
	ledger := &mockLedger{height: 10}
	mockPut := &mockPutBlockFunc{}
	shutdownCallback := func() {}

	t.Run("empty configuration", func(t *testing.T) {
		cfg := config.NeoFSBlockFetcher{
			NeoFSService: config.NeoFSService{
				InternalService: config.InternalService{
					Enabled: true,
				},
				Timeout: 0,
			},
			OIDBatchSize:           0,
			DownloaderWorkersCount: 0,
		}
		_, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback, Blocks)
		require.Error(t, err)
	})

	t.Run("no addresses", func(t *testing.T) {
		cfg := config.NeoFSBlockFetcher{
			NeoFSService: config.NeoFSService{
				InternalService: config.InternalService{
					Enabled: true,
				},
				Addresses: []string{},
			},
		}
		_, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback, Blocks)
		require.Error(t, err)
	})

	t.Run("default values", func(t *testing.T) {
		cfg := config.NeoFSBlockFetcher{
			NeoFSService: config.NeoFSService{
				InternalService: config.InternalService{
					Enabled: true,
				},
				Addresses: []string{"localhost:8080"},
			},
			BQueueSize: DefaultQueueCacheSize,
		}
		service, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback, Blocks)
		require.NoError(t, err)
		require.NotNil(t, service)

		require.Equal(t, service.IsActive(), false)
		require.Equal(t, service.cfg.Timeout, neofs.DefaultTimeout)
		require.Equal(t, service.cfg.OIDBatchSize, DefaultQueueCacheSize/2)
		require.Equal(t, service.cfg.DownloaderWorkersCount, neofs.DefaultDownloaderWorkersCount)
		require.Equal(t, service.IsActive(), false)
	})

	t.Run("NeoFS client", func(t *testing.T) {
		cfg := config.NeoFSBlockFetcher{
			NeoFSService: config.NeoFSService{
				InternalService: config.InternalService{
					Enabled: true,
				},
				Addresses: []string{"localhost:1"},
			},
		}
		service, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback, Blocks)
		require.NoError(t, err)
		err = service.Start()
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to dial NeoFS pool:")
		require.Equal(t, service.IsActive(), false)
	})

	t.Run("invalid wallet", func(t *testing.T) {
		cfg := config.NeoFSBlockFetcher{
			NeoFSService: config.NeoFSService{
				Addresses: []string{"http://localhost:8080"},
				InternalService: config.InternalService{
					Enabled: true,
					UnlockWallet: config.Wallet{
						Path:     "invalid/path/to/wallet.json",
						Password: "wrong-password",
					},
				},
			},
		}
		_, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback, Blocks)
		require.Error(t, err)
		require.Contains(t, err.Error(), "open wallet: open invalid/path/to/wallet.json:")
	})
}
