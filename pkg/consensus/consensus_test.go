package consensus

import (
	"testing"

	"github.com/nspcc-dev/dbft/block"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/cache"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewService(t *testing.T) {
	srv := newTestService(t)
	tx := &transaction.Transaction{
		Type: transaction.MinerType,
		Data: &transaction.MinerTX{Nonce: 12345},
	}
	require.NoError(t, srv.Chain.PoolTx(tx))

	var txx []block.Transaction
	require.NotPanics(t, func() { txx = srv.getVerifiedTx(1) })
	require.Len(t, txx, 2)
	require.Equal(t, tx, txx[1])
	srv.Chain.Close()
}

func TestService_GetVerified(t *testing.T) {
	srv := newTestService(t)
	txs := []*transaction.Transaction{
		newMinerTx(1),
		newMinerTx(2),
		newMinerTx(3),
		newMinerTx(4),
	}
	require.NoError(t, srv.Chain.PoolTx(txs[3]))

	hashes := []util.Uint256{txs[0].Hash(), txs[1].Hash(), txs[2].Hash()}

	p := new(Payload)
	p.message = &message{}
	p.SetType(payload.PrepareRequestType)
	p.SetPayload(&prepareRequest{transactionHashes: hashes, minerTx: *newMinerTx(999)})
	p.SetValidatorIndex(1)

	priv, _ := getTestValidator(1)
	require.NoError(t, p.Sign(priv))

	srv.OnPayload(p)
	require.Equal(t, hashes, srv.lastProposal)

	srv.dbft.ViewNumber = 1

	t.Run("new transactions will be proposed in case of failure", func(t *testing.T) {
		txx := srv.getVerifiedTx(10)
		require.Equal(t, 2, len(txx), "there is only 1 tx in mempool")
		require.Equal(t, txs[3], txx[1])
	})

	t.Run("more than half of the last proposal will be reused", func(t *testing.T) {
		for _, tx := range txs[:2] {
			require.NoError(t, srv.Chain.PoolTx(tx))
		}

		txx := srv.getVerifiedTx(10)
		require.Contains(t, txx, txs[0])
		require.Contains(t, txx, txs[1])
		require.NotContains(t, txx, txs[2])
	})
	srv.Chain.Close()
}

func TestService_ValidatePayload(t *testing.T) {
	srv := newTestService(t)
	priv, _ := getTestValidator(1)
	p := new(Payload)
	p.message = &message{}

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
	srv.Chain.Close()
}

func TestService_getTx(t *testing.T) {
	srv := newTestService(t)

	t.Run("transaction in mempool", func(t *testing.T) {
		tx := newMinerTx(1234)
		h := tx.Hash()

		require.Equal(t, nil, srv.getTx(h))

		require.NoError(t, srv.Chain.PoolTx(tx))

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
	srv.Chain.Close()
}

func TestService_OnPayload(t *testing.T) {
	srv := newTestService(t)

	priv, _ := getTestValidator(1)
	p := new(Payload)
	p.message = &message{}
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
	srv.Chain.Close()
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
		Broadcast: func(cache.Hashable) {},
		Chain:     newTestChain(t),
		RequestTx: func(...util.Uint256) {},
		Wallet: &wallet.Config{
			Path:     "./testdata/wallet1.json",
			Password: "one",
		},
	})
	require.NoError(t, err)

	return srv.(*service)
}

func getTestValidator(i int) (*privateKey, *publicKey) {
	var wif, password string

	// Sorted by public key.
	switch i {
	case 0:
		wif = "6PYXHjPaNvW8YknSXaKsTWjf9FRxo1s4naV2jdmSQEgzaqKGX368rndN3L"
		password = "two"

	case 1:
		wif = "6PYRXVwHSqFSukL3CuXxdQ75VmsKpjeLgQLEjt83FrtHf1gCVphHzdD4nc"
		password = "four"

	case 2:
		wif = "6PYLmjBYJ4wQTCEfqvnznGJwZeW9pfUcV5m5oreHxqryUgqKpTRAFt9L8Y"
		password = "one"

	case 3:
		wif = "6PYX86vYiHfUbpD95hfN1xgnvcSxy5skxfWYKu3ztjecxk6ikYs2kcWbeh"
		password = "three"

	default:
		return nil, nil
	}

	key, err := keys.NEP2Decrypt(wif, password)
	if err != nil {
		return nil, nil
	}

	return &privateKey{PrivateKey: key}, &publicKey{PublicKey: key.PublicKey()}
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
func (fs *feer) IsLowPriority(util.Fixed8) bool                  { return false }
func (fs *feer) FeePerByte(*transaction.Transaction) util.Fixed8 { return util.Fixed8(0) }
func (fs *feer) SystemFee(*transaction.Transaction) util.Fixed8  { return util.Fixed8(0) }
