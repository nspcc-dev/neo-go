package stateroot

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

// RelayCallback represents callback for sending validated state roots.
type RelayCallback = func(*payload.Extensible)

// AddSignature adds state root signature.
func (s *service) AddSignature(height uint32, validatorIndex int32, sig []byte) error {
	if !s.MainCfg.Enabled {
		return nil
	}
	_, acc := s.getAccount()
	if acc == nil {
		return nil
	}

	incRoot := s.getIncompleteRoot(height)
	if incRoot == nil {
		return nil
	}

	incRoot.Lock()
	defer incRoot.Unlock()

	if validatorIndex < 0 || int(validatorIndex) >= len(incRoot.svList) {
		return errors.New("invalid validator index")
	}

	pub := incRoot.svList[validatorIndex]
	if incRoot.root != nil {
		ok := pub.VerifyHashable(sig, uint32(s.Network), incRoot.root)
		if !ok {
			return fmt.Errorf("invalid state root signature for %d", validatorIndex)
		}
	}
	incRoot.addSignature(pub, sig)
	s.trySendRoot(incRoot, acc)
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
	incRoot := &incompleteRoot{svList: s.GetStateValidators(height), sigs: make(map[string]*rootSig)}
	s.incompleteRoots[height] = incRoot
	return incRoot
}

// trySendRoot attempts to finalize and send MPTRoot, it must be called with ir locked.
func (s *service) trySendRoot(ir *incompleteRoot, acc *wallet.Account) {
	if ir.isSent {
		return
	}
	sr, ready := ir.finalize()
	if ready {
		err := s.AddStateRoot(sr)
		if err != nil {
			s.log.Error("can't add validated state root", zap.Error(err))
		}
		s.sendValidatedRoot(sr, acc)
		ir.isSent = true
	}
}

func (s *service) sendValidatedRoot(r *state.MPTRoot, acc *wallet.Account) {
	priv := acc.PrivateKey()
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
			VerificationScript: acc.GetVerificationScript(),
		},
	}
	sig := priv.SignHashable(uint32(s.Network), ep)
	buf := io.NewBufBinWriter()
	emit.Bytes(buf.BinWriter, sig)
	ep.Witness.InvocationScript = buf.Bytes()
	s.relayExtensible(ep)
}
