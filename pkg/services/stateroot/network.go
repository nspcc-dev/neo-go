package stateroot

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"go.uber.org/zap"
)

// RelayCallback represents callback for sending validated state roots.
type RelayCallback = func(*payload.Extensible)

// AddSignature adds state root signature.
func (s *service) AddSignature(height uint32, validatorIndex int32, sig []byte) error {
	if !s.MainCfg.Enabled {
		return nil
	}
	acc := s.getAccount()
	if acc == nil {
		return nil
	}

	pubs := s.GetStateValidators(height)
	if validatorIndex < 0 || int(validatorIndex) >= len(pubs) {
		return errors.New("invalid validator index")
	}
	pub := pubs[validatorIndex]

	incRoot := s.getIncompleteRoot(height)
	if incRoot == nil {
		return nil
	}

	incRoot.Lock()
	if incRoot.root != nil {
		ok := pub.VerifyHashable(sig, uint32(s.Network), incRoot.root)
		if !ok {
			incRoot.Unlock()
			return fmt.Errorf("invalid state root signature for %d", validatorIndex)
		}
	}
	incRoot.addSignature(pub, sig)
	sr, ready := incRoot.finalize(pubs)
	incRoot.Unlock()

	if ready {
		err := s.AddStateRoot(sr)
		if err != nil {
			s.log.Error("can't add validated state root", zap.Error(err))
		}
		s.sendValidatedRoot(sr, acc.PrivateKey())
	}
	return nil
}

// GetConfig returns service configuration.
func (s *service) GetConfig() config.StateRoot {
	return s.MainCfg
}

func (s *service) getIncompleteRoot(height uint32) *incompleteRoot {
	s.srMtx.Lock()
	defer s.srMtx.Unlock()
	if incRoot, ok := s.incompleteRoots[height]; ok {
		return incRoot
	}
	incRoot := &incompleteRoot{sigs: make(map[string]*rootSig)}
	s.incompleteRoots[height] = incRoot
	return incRoot
}

func (s *service) sendValidatedRoot(r *state.MPTRoot, priv *keys.PrivateKey) {
	w := io.NewBufBinWriter()
	m := NewMessage(RootT, r)
	m.EncodeBinary(w.BinWriter)
	ep := &payload.Extensible{
		Category:        Category,
		ValidBlockStart: r.Index,
		ValidBlockEnd:   r.Index + transaction.MaxValidUntilBlockIncrement,
		Sender:          priv.GetScriptHash(),
		Data:            w.Bytes(),
		Witness: transaction.Witness{
			VerificationScript: s.getAccount().GetVerificationScript(),
		},
	}
	sig := priv.SignHashable(uint32(s.Network), ep)
	buf := io.NewBufBinWriter()
	emit.Bytes(buf.BinWriter, sig)
	ep.Witness.InvocationScript = buf.Bytes()
	s.onValidatedRoot(ep)
}
