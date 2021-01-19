package consensus

import (
	"errors"
	"testing"
	"time"

	"github.com/nspcc-dev/dbft/block"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/cache"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
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
	require.NotPanics(t, func() { txx = srv.getVerifiedTx() })
	require.Len(t, txx, 2)
	require.Equal(t, tx, txx[1])
	srv.Chain.Close()
}

func TestService_GetVerified(t *testing.T) {
	srv := newTestService(t)
	srv.dbft.Start()

	txs := []*transaction.Transaction{
		newMinerTx(1),
		newMinerTx(2),
		newMinerTx(3),
		newMinerTx(4),
	}
	require.NoError(t, srv.Chain.PoolTx(txs[3]))

	hashes := []util.Uint256{txs[0].Hash(), txs[1].Hash(), txs[2].Hash()}

	// Everyone sends a message.
	for i := 0; i < 4; i++ {
		var p *Payload
		priv, _ := getTestValidator(i)
		// To properly sign stateroot in prepare request.
		srv.dbft.Priv = priv
		// One PrepareRequest and three ChangeViews.
		if i == 1 {
			req := &prepareRequest{
				transactionHashes: hashes,
				minerTx:           *newMinerTx(999),
			}
			if srv.stateRootEnabled() {
				req.stateRootEnabled = srv.stateRootEnabled()
				sr, err := srv.Chain.GetStateRoot(srv.Chain.BlockHeight())
				require.NoError(t, err)
				sig, err := priv.Sign(sr.GetSignedPart())
				require.NoError(t, err)
				copy(req.stateRootSig[:], sig)
			}
			p = srv.newPayload(&srv.dbft.Context, payload.PrepareRequestType, req).(*Payload)
		} else {
			p = srv.newPayload(&srv.dbft.Context, payload.ChangeViewType, &changeView{
				newViewNumber: 1,
				timestamp:     uint32(time.Now().Unix()),
			}).(*Payload)
		}
		p.SetHeight(1)
		p.SetValidatorIndex(uint16(i))

		require.NoError(t, p.Sign(priv))

		// Skip srv.OnPayload, because the service is not really started.
		srv.dbft.OnReceive(p)
	}
	require.Equal(t, uint8(1), srv.dbft.ViewNumber)
	require.Equal(t, hashes, srv.lastProposal)

	t.Run("new transactions will be proposed in case of failure", func(t *testing.T) {
		txx := srv.getVerifiedTx()
		require.Equal(t, 2, len(txx), "there is only 1 tx in mempool")
		require.Equal(t, txs[3], txx[1])
	})

	t.Run("more than half of the last proposal will be reused", func(t *testing.T) {
		for _, tx := range txs[:2] {
			require.NoError(t, srv.Chain.PoolTx(tx))
		}

		txx := srv.getVerifiedTx()
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

func TestService_PrepareRequest(t *testing.T) {
	srv := newTestService(t)
	defer srv.Chain.Close()

	srv.dbft.Start()
	defer srv.dbft.Timer.Stop()

	priv, _ := getTestValidator(1)
	prevHash := srv.Chain.CurrentBlockHash()

	checkRequest := func(t *testing.T, expectedErr error, version uint32, prevHash util.Uint256, sig []byte) {
		req := &prepareRequest{
			stateRootEnabled: true,
			minerTx:          *newMinerTx(123),
		}
		req.transactionHashes = []util.Uint256{req.minerTx.Hash()}
		copy(req.stateRootSig[:], sig)
		p := srv.newPayload(&srv.dbft.Context, payload.PrepareRequestType, req)
		p.SetValidatorIndex(1)
		p.(*Payload).SetVersion(version)
		p.(*Payload).SetPrevHash(prevHash)
		require.NoError(t, p.(*Payload).Sign(priv))
		err := srv.verifyRequest(p)
		if expectedErr == nil {
			require.NoError(t, err)
			return
		}
		require.True(t, errors.Is(err, expectedErr), "got: %v", err)
	}

	sr, err := srv.Chain.GetStateRoot(srv.dbft.BlockIndex - 1)
	require.NoError(t, err)
	sig, err := priv.Sign(sr.GetSignedPart())
	require.NoError(t, err)

	checkRequest(t, errInvalidVersion, 0xFF, prevHash, sig)
	checkRequest(t, errInvalidPrevHash, 0, random.Uint256(), sig)
	checkRequest(t, errInvalidStateRoot, 0, prevHash, []byte{})
	checkRequest(t, nil, 0, prevHash, sig)
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
	// This test directly reads things from srv.messages that normally
	// is read by internal goroutine started with Start(). So let's
	// pretend we really did start already.
	srv.started.Store(true)

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
