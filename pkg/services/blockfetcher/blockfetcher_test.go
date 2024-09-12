package blockfetcher

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockLedger struct {
	height uint32
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

func (m *mockPutBlockFunc) putBlock(b *block.Block) error {
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
			Timeout:                0,
			OIDBatchSize:           0,
			DownloaderWorkersCount: 0,
		}
		_, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback)
		require.Error(t, err)
	})

	t.Run("no addresses", func(t *testing.T) {
		cfg := config.NeoFSBlockFetcher{
			Addresses: []string{},
		}
		_, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback)
		require.Error(t, err)
	})

	t.Run("default values", func(t *testing.T) {
		cfg := config.NeoFSBlockFetcher{
			Addresses: []string{"http://localhost:8080"},
		}
		service, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback)
		require.NoError(t, err)
		require.NotNil(t, service)

		require.Equal(t, service.IsActive(), false)
		require.Equal(t, service.cfg.Timeout, defaultTimeout)
		require.Equal(t, service.cfg.OIDBatchSize, defaultOIDBatchSize)
		require.Equal(t, service.cfg.DownloaderWorkersCount, defaultDownloaderWorkersCount)
		require.Equal(t, service.IsActive(), false)
	})

	t.Run("SDK client", func(t *testing.T) {
		cfg := config.NeoFSBlockFetcher{
			Addresses: []string{"http://localhost:8080"},
		}
		service, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback)
		require.NoError(t, err)
		err = service.Start()
		require.Error(t, err)
		require.Contains(t, err.Error(), "create SDK client")
		require.Equal(t, service.IsActive(), false)
	})

	t.Run("invalid wallet", func(t *testing.T) {
		cfg := config.NeoFSBlockFetcher{
			Addresses: []string{"http://localhost:8080"},
			InternalService: config.InternalService{
				Enabled: true,
				UnlockWallet: config.Wallet{
					Path:     "invalid/path/to/wallet.json",
					Password: "wrong-password",
				},
			},
		}
		_, err := New(ledger, cfg, logger, mockPut.putBlock, shutdownCallback)
		require.Error(t, err)
	})
}
