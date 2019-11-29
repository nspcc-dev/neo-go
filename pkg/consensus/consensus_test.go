package consensus

import (
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/nspcc-dev/dbft/block"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	srv := newTestService(t)
	tx := &transaction.Transaction{
		Type: transaction.MinerType,
		Data: &transaction.MinerTX{Nonce: 12345},
	}
	item := core.NewPoolItem(tx, new(feer))
	srv.Chain.GetMemPool().TryAdd(tx.Hash(), item)

	var txx []block.Transaction
	require.NotPanics(t, func() { txx = srv.getVerifiedTx(1) })
	require.Len(t, txx, 2)
	require.Equal(t, tx, txx[1])
}

func newTestService(t *testing.T) *service {
	srv, err := NewService(Config{
		Broadcast: func(*Payload) {},
		Chain:     newTestChain(t),
		RequestTx: func(...util.Uint256) {},
		Wallet: &config.WalletConfig{
			Path:     "6PYLmjBYJ4wQTCEfqvnznGJwZeW9pfUcV5m5oreHxqryUgqKpTRAFt9L8Y",
			Password: "one",
		},
	})
	require.NoError(t, err)

	return srv.(*service)
}

func newTestChain(t *testing.T) *core.Blockchain {
	unitTestNetCfg, err := config.Load("../../config", config.ModeUnitTestNet)
	require.NoError(t, err)

	chain, err := core.NewBlockchain(storage.NewMemoryStore(), unitTestNetCfg.ProtocolConfiguration)
	require.NoError(t, err)

	go chain.Run()

	return chain
}

type feer struct{}

func (fs *feer) NetworkFee(*transaction.Transaction) util.Fixed8 { return util.Fixed8(0) }
func (fs *feer) IsLowPriority(*transaction.Transaction) bool     { return false }
func (fs *feer) FeePerByte(*transaction.Transaction) util.Fixed8 { return util.Fixed8(0) }
func (fs *feer) SystemFee(*transaction.Transaction) util.Fixed8  { return util.Fixed8(0) }
