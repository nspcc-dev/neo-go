package stateroot

import (
	"errors"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
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
		Run()
		Shutdown()
	}

	service struct {
		blockchainer.StateRoot
		chain blockchainer.Blockchainer

		MainCfg config.StateRoot
		Network netmode.Magic

		log       *zap.Logger
		accMtx    sync.RWMutex
		accHeight uint32
		myIndex   byte
		wallet    *wallet.Wallet
		acc       *wallet.Account

		srMtx           sync.Mutex
		incompleteRoots map[uint32]*incompleteRoot

		cbMtx           sync.RWMutex
		onValidatedRoot RelayCallback
		blockCh         chan *block.Block
		done            chan struct{}
	}
)

const (
	// Category is message category for extensible payloads.
	Category = "StateService"
)

// New returns new state root service instance using underlying module.
func New(cfg config.StateRoot, log *zap.Logger, bc blockchainer.Blockchainer) (Service, error) {
	s := &service{
		StateRoot:       bc.GetStateModule(),
		Network:         bc.GetConfig().Magic,
		chain:           bc,
		log:             log,
		incompleteRoots: make(map[uint32]*incompleteRoot),
		blockCh:         make(chan *block.Block),
		done:            make(chan struct{}),
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
		return s.AddStateRoot(sr)
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
			err := acc.Decrypt(s.MainCfg.UnlockWallet.Password)
			if err == nil {
				s.acc = acc
				s.accHeight = height
				s.myIndex = byte(i)
				break
			}
		}
	}
}
