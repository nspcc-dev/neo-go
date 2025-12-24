package statefetcher

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockLedger struct {
	height        uint32
	lastStoredKey []byte
}

func (m *mockLedger) HeaderHeight() uint32 { return m.height }

func (m *mockLedger) GetConfig() config.Blockchain { return config.Blockchain{} }

func (m *mockLedger) GetLastStoredKey() []byte { return m.lastStoredKey }

func (m *mockLedger) AddContractStorageItems(kvs []storage.KeyValue) error { return nil }

func (m *mockLedger) InitContractStorageSync(r state.MPTRoot) error { return nil }

func (m *mockLedger) VerifyWitness(h util.Uint160, c hash.Hashable, w *transaction.Witness, gas int64) (int64, error) {
	return 0, nil
}

func TestServiceConstructor(t *testing.T) {
	logger := zap.NewNop()
	ledger := &mockLedger{height: 100}
	shutdown := func() {}

	t.Run("empty configuration", func(t *testing.T) {
		cfg := config.NeoFSStateFetcher{}
		s, err := New(ledger, cfg, 0, logger, shutdown)
		require.NoError(t, err)
		require.Equal(t, &Service{}, s)
	})

	t.Run("no addresses", func(t *testing.T) {
		cfg := config.NeoFSStateFetcher{
			NeoFSService: config.NeoFSService{
				InternalService: config.InternalService{
					Enabled: true,
				},
				Addresses: []string{},
			},
		}
		_, err := New(ledger, cfg, 0, logger, shutdown)
		require.ErrorContains(t, err, "failed to create service: empty endpoints")
	})

	t.Run("invalid wallet", func(t *testing.T) {
		cfg := config.NeoFSStateFetcher{
			NeoFSService: config.NeoFSService{
				InternalService: config.InternalService{
					Enabled: true,
					UnlockWallet: config.Wallet{
						Path:     "bad/path.json",
						Password: "wrong-pwd",
					},
				},
				Addresses: []string{"http://localhost:8080"},
			},
		}
		_, err := New(ledger, cfg, 0, logger, shutdown)
		require.ErrorContains(t, err, "open wallet:")
	})

	t.Run("IsActive and IsShutdown check", func(t *testing.T) {
		cfg := config.NeoFSStateFetcher{
			NeoFSService: config.NeoFSService{
				InternalService: config.InternalService{
					Enabled: false,
				},
			},
		}
		svc, err := New(ledger, cfg, 0, logger, shutdown)
		require.NoError(t, err)
		require.False(t, svc.IsActive())
		require.False(t, svc.IsShutdown())
	})
}
