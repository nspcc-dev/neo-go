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
	"go.uber.org/zap/zaptest"
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

func TestService_ValidatePayload(t *testing.T) {
	srv := newTestService(t)
	priv, _ := getTestValidator(1)
	p := new(Payload)

	p.SetPayload(&prepareRequest{})

	t.Run("invalid validator index", func(t *testing.T) {
		p.SetValidatorIndex(11)
		require.NoError(t, p.Sign(priv))

		var ok bool
		require.NotPanics(t, func() { ok = srv.validatePayload(p) })
		require.False(t, ok)
	})

	t.Run("wrong validator index", func(t *testing.T) {
		p.SetValidatorIndex(2)
		require.NoError(t, p.Sign(priv))
		require.False(t, srv.validatePayload(p))
	})

	t.Run("normal case", func(t *testing.T) {
		p.SetValidatorIndex(1)
		require.NoError(t, p.Sign(priv))
		require.True(t, srv.validatePayload(p))
	})
}

func TestService_getTx(t *testing.T) {
	srv := newTestService(t)

	t.Run("transaction in mempool", func(t *testing.T) {
		tx := newMinerTx(1234)
		h := tx.Hash()

		require.Equal(t, nil, srv.getTx(h))

		item := core.NewPoolItem(tx, new(feer))
		srv.Chain.GetMemPool().TryAdd(h, item)

		got := srv.getTx(h)
		require.NotNil(t, got)
		require.Equal(t, h, got.Hash())
	})

	t.Run("transaction in local cache", func(t *testing.T) {
		tx := newMinerTx(4321)
		h := tx.Hash()

		require.Equal(t, nil, srv.getTx(h))

		srv.txx.Add(tx)

		got := srv.getTx(h)
		require.NotNil(t, got)
		require.Equal(t, h, got.Hash())
	})
}

func TestService_OnPayload(t *testing.T) {
	srv := newTestService(t)

	priv, _ := getTestValidator(1)
	p := new(Payload)
	p.SetValidatorIndex(1)
	p.SetPayload(&prepareRequest{})

	// payload is not signed
	srv.OnPayload(p)
	shouldNotReceive(t, srv.messages)
	require.Nil(t, srv.GetPayload(p.Hash()))

	require.NoError(t, p.Sign(priv))
	srv.OnPayload(p)
	shouldReceive(t, srv.messages)
	require.Equal(t, p, srv.GetPayload(p.Hash()))

	// payload has already been received
	srv.OnPayload(p)
	shouldNotReceive(t, srv.messages)
}

func shouldReceive(t *testing.T, ch chan Payload) {
	select {
	case <-ch:
	default:
		require.Fail(t, "missing expected message")
	}
}

func shouldNotReceive(t *testing.T, ch chan Payload) {
	select {
	case <-ch:
		require.Fail(t, "unexpected message receive")
	default:
	}
}

func newTestService(t *testing.T) *service {
	srv, err := NewService(Config{
		Logger:    zaptest.NewLogger(t),
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

func getTestValidator(i int) (*privateKey, *publicKey) {
	var wallet *config.WalletConfig
	switch i {
	case 0:
		wallet = &config.WalletConfig{
			Path:     "6PYLmjBYJ4wQTCEfqvnznGJwZeW9pfUcV5m5oreHxqryUgqKpTRAFt9L8Y",
			Password: "one",
		}
	case 1:
		wallet = &config.WalletConfig{
			Path:     "6PYXHjPaNvW8YknSXaKsTWjf9FRxo1s4naV2jdmSQEgzaqKGX368rndN3L",
			Password: "two",
		}
	case 2:
		wallet = &config.WalletConfig{
			Path:     "6PYX86vYiHfUbpD95hfN1xgnvcSxy5skxfWYKu3ztjecxk6ikYs2kcWbeh",
			Password: "three",
		}
	case 3:
		wallet = &config.WalletConfig{
			Path:     "6PYRXVwHSqFSukL3CuXxdQ75VmsKpjeLgQLEjt83FrtHf1gCVphHzdD4nc",
			Password: "four",
		}
	default:
		return nil, nil
	}

	priv, pub := getKeyPair(wallet)

	return priv.(*privateKey), pub.(*publicKey)
}

func newTestChain(t *testing.T) *core.Blockchain {
	unitTestNetCfg, err := config.Load("../../config", config.ModeUnitTestNet)
	require.NoError(t, err)

	chain, err := core.NewBlockchain(storage.NewMemoryStore(), unitTestNetCfg.ProtocolConfiguration, zaptest.NewLogger(t))
	require.NoError(t, err)

	go chain.Run()

	return chain
}

type feer struct{}

func (fs *feer) NetworkFee(*transaction.Transaction) util.Fixed8 { return util.Fixed8(0) }
func (fs *feer) IsLowPriority(*transaction.Transaction) bool     { return false }
func (fs *feer) FeePerByte(*transaction.Transaction) util.Fixed8 { return util.Fixed8(0) }
func (fs *feer) SystemFee(*transaction.Transaction) util.Fixed8  { return util.Fixed8(0) }
