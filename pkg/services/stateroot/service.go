package stateroot

import (
	"errors"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

type (
	// Service represents state root service.
	Service interface {
		blockchainer.StateRoot
		OnPayload(p *payload.Extensible) error
		AddSignature(height uint32, validatorIndex int32, sig []byte) error
		GetConfig() config.StateRoot
		SetRelayCallback(RelayCallback)
	}

	service struct {
		blockchainer.StateRoot

		MainCfg config.StateRoot
		Network netmode.Magic

		log     *zap.Logger
		accMtx  sync.RWMutex
		myIndex byte
		wallet  *wallet.Wallet
		acc     *wallet.Account

		srMtx           sync.Mutex
		incompleteRoots map[uint32]*incompleteRoot

		cbMtx           sync.RWMutex
		onValidatedRoot RelayCallback
	}
)

const (
	// Category is message category for extensible payloads.
	Category = "StateService"
)

// New returns new state root service instance using underlying module.
func New(cfg config.StateRoot, log *zap.Logger, mod blockchainer.StateRoot) (Service, error) {
	s := &service{
		StateRoot:       mod,
		log:             log,
		incompleteRoots: make(map[uint32]*incompleteRoot),
	}

	s.MainCfg = cfg
	if cfg.Enabled {
		var err error
		w := cfg.UnlockWallet
		if s.wallet, err = wallet.NewWalletFromFile(w.Path); err != nil {
			return nil, err
		}

		haveAccount := false
		for _, acc := range s.wallet.Accounts {
			if err := acc.Decrypt(w.Password); err == nil {
				haveAccount = true
				break
			}
		}
		if !haveAccount {
			return nil, errors.New("no wallet account could be unlocked")
		}

		s.SetUpdateValidatorsCallback(s.updateValidators)
		s.SetSignAndSendCallback(s.signAndSend)
	}
	return s, nil
}

// OnPayload implements Service interface.
func (s *service) OnPayload(ep *payload.Extensible) error {
	m := new(Message)
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
		return s.AddStateRoot(sr)
	case VoteT:
		v := m.Payload.(*Vote)
		return s.AddSignature(v.Height, v.ValidatorIndex, v.Signature)
	}
	return nil
}

func (s *service) updateValidators(pubs keys.PublicKeys) {
	s.accMtx.Lock()
	defer s.accMtx.Unlock()

	s.acc = nil
	for i := range pubs {
		if acc := s.wallet.GetAccount(pubs[i].GetScriptHash()); acc != nil {
			err := acc.Decrypt(s.MainCfg.UnlockWallet.Password)
			if err == nil {
				s.acc = acc
				s.myIndex = byte(i)
				break
			}
		}
	}
}
