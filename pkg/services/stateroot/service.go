package stateroot

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/stateroot"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

type (
	// Ledger is an interface to Blockchain sufficient for Service.
	Ledger interface {
		IsHardforkEnabled(hf *config.Hardfork, blockHeight uint32) bool
		GetConfig() config.Blockchain
		GetDesignatedByRole(role noderoles.Role) (keys.PublicKeys, uint32, error)
		GetMillisecondsPerBlock() uint32
		HeaderHeight() uint32
		BlockHeight() uint32
		SubscribeForBlocks(ch chan *block.Block)
		UnsubscribeFromBlocks(ch chan *block.Block)
	}

	// Service represents a state root service.
	Service interface {
		Name() string
		OnPayload(p *payload.Extensible) error
		AddSignature(height uint32, validatorIndex int32, sig []byte) error
		GetConfig() config.StateRoot
		// Start runs service instance in a separate goroutine.
		// The service only starts once, subsequent calls to Start are no-op.
		Start()
		// Shutdown stops the service. It can only be called once, subsequent calls
		// to Shutdown on the same instance are no-op. The instance that was stopped can
		// not be started again by calling Start (use a new instance if needed).
		Shutdown()
		// IsAuthorized returns whether state root service currently is authorized to sign
		// state roots. It returns true iff designated StateValidator node's account
		// provided to the state root service in decrypted state.
		IsAuthorized() bool
	}

	service struct {
		*stateroot.Module
		chain Ledger

		MainCfg config.StateRoot
		Network netmode.Magic

		log       *zap.Logger
		started   atomic.Bool
		accMtx    sync.RWMutex
		accHeight uint32
		myIndex   byte
		wallet    *wallet.Wallet
		acc       *wallet.Account

		srMtx           sync.Mutex
		incompleteRoots map[uint32]*incompleteRoot

		maxRetries      int
		relayExtensible RelayCallback
		// blockCh is a channel used to receive block notifications from the
		// Blockchain. It has a tiny buffer in order to avoid Blockchain blocking
		// on block addition under the high load.
		blockCh chan *block.Block
		stopCh  chan struct{}
		done    chan struct{}
	}
)

const (
	// Category is a message category for extensible payloads.
	Category = "StateService"
)

// New returns a new state root service instance using the underlying module.
func New(cfg config.StateRoot, sm *stateroot.Module, log *zap.Logger, bc Ledger, cb RelayCallback) (Service, error) {
	bcConf := bc.GetConfig()
	s := &service{
		Module:          sm,
		Network:         bcConf.Magic,
		chain:           bc,
		log:             log,
		incompleteRoots: make(map[uint32]*incompleteRoot),
		blockCh:         make(chan *block.Block, 1),
		stopCh:          make(chan struct{}),
		done:            make(chan struct{}),
		maxRetries:      voteValidEndInc,
		relayExtensible: cb,
	}

	s.MainCfg = cfg
	if cfg.Enabled {
		// TODO: disable state service once transition to Faun happens.
		var err error
		w := cfg.UnlockWallet
		if s.wallet, err = wallet.NewWalletFromFile(w.Path); err != nil {
			return nil, err
		}

		var haveAccount = slices.ContainsFunc(s.wallet.Accounts, func(acc *wallet.Account) bool {
			return acc.Decrypt(w.Password, s.wallet.Scrypt) == nil
		})
		if !haveAccount {
			return nil, errors.New("no wallet account could be unlocked")
		}

		keys, h, err := bc.GetDesignatedByRole(noderoles.StateValidator)
		if err != nil {
			return nil, fmt.Errorf("failed to get designated StateValidators: %w", err)
		}
		s.updateValidators(h, keys)

		s.SetUpdateValidatorsCallback(s.updateValidators)
	}
	return s, nil
}

// OnPayload implements Service interface.
func (s *service) OnPayload(ep *payload.Extensible) error {
	m := &Message{}
	r := io.NewBinReaderFromBuf(ep.Data)
	m.DecodeBinary(r)
	if r.Err != nil {
		return r.Err
	}
	switch m.Type {
	case RootT:
		sr := m.Payload.(*state.MPTRoot)
		if sr.Index == 0 {
			return nil
		}
		err := s.AddStateRoot(sr)
		if errors.Is(err, stateroot.ErrStateMismatch) {
			s.log.Error("can't add SV-signed state root", zap.Error(err))
			return nil
		}
		s.srMtx.Lock()
		ir, ok := s.incompleteRoots[sr.Index]
		s.srMtx.Unlock()
		if ok {
			ir.Lock()
			ir.isSent = true
			ir.Unlock()
		}
		return err
	case VoteT:
		v := m.Payload.(*Vote)
		return s.AddSignature(v.Height, v.ValidatorIndex, v.Signature)
	}
	return nil
}

func (s *service) updateValidators(height uint32, pubs keys.PublicKeys) {
	s.accMtx.Lock()
	defer s.accMtx.Unlock()

	s.acc = nil
	for i := range pubs {
		if acc := s.wallet.GetAccount(pubs[i].GetScriptHash()); acc != nil {
			err := acc.Decrypt(s.MainCfg.UnlockWallet.Password, s.wallet.Scrypt)
			if err == nil {
				s.acc = acc
				s.accHeight = height
				s.myIndex = byte(i)
				break
			}
		}
	}
}

// IsAuthorized implements Service interface.
func (s *service) IsAuthorized() bool {
	_, acc := s.getAccount()
	return acc != nil
}
