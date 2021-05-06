package stateroot

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

// Run runs service instance in a separate goroutine.
func (s *service) Run() {
	s.log.Info("starting state validation service")
	s.chain.SubscribeForBlocks(s.blockCh)
	go s.run()
}

func (s *service) run() {
runloop:
	for {
		select {
		case b := <-s.blockCh:
			r, err := s.GetStateRoot(b.Index)
			if err != nil {
				s.log.Error("can't get state root for new block", zap.Error(err))
			} else if err := s.signAndSend(r); err != nil {
				s.log.Error("can't sign or send state root", zap.Error(err))
			}
		case <-s.done:
			break runloop
		}
	}
drainloop:
	for {
		select {
		case <-s.blockCh:
		default:
			break drainloop
		}
	}
}

// Shutdown stops the service.
func (s *service) Shutdown() {
	s.chain.UnsubscribeFromBlocks(s.blockCh)
	close(s.done)
}

func (s *service) signAndSend(r *state.MPTRoot) error {
	if !s.MainCfg.Enabled {
		return nil
	}

	myIndex, acc := s.getAccount()
	if acc == nil {
		return nil
	}

	sig := acc.PrivateKey().SignHashable(uint32(s.Network), r)
	incRoot := s.getIncompleteRoot(r.Index)
	incRoot.Lock()
	incRoot.root = r
	incRoot.addSignature(acc.PrivateKey().PublicKey(), sig)
	incRoot.reverify(s.Network)
	s.trySendRoot(incRoot, acc)
	incRoot.Unlock()

	msg := NewMessage(VoteT, &Vote{
		ValidatorIndex: int32(myIndex),
		Height:         r.Index,
		Signature:      sig,
	})

	w := io.NewBufBinWriter()
	msg.EncodeBinary(w.BinWriter)
	if w.Err != nil {
		return w.Err
	}
	e := &payload.Extensible{
		Category:        Category,
		ValidBlockStart: r.Index,
		ValidBlockEnd:   r.Index + transaction.MaxValidUntilBlockIncrement,
		Sender:          acc.PrivateKey().GetScriptHash(),
		Data:            w.Bytes(),
		Witness: transaction.Witness{
			VerificationScript: acc.GetVerificationScript(),
		},
	}
	sig = acc.PrivateKey().SignHashable(uint32(s.Network), e)
	buf := io.NewBufBinWriter()
	emit.Bytes(buf.BinWriter, sig)
	e.Witness.InvocationScript = buf.Bytes()
	s.relayExtensible(e)
	return nil
}

// getAccount returns current index and account for the node running this service.
func (s *service) getAccount() (byte, *wallet.Account) {
	s.accMtx.RLock()
	defer s.accMtx.RUnlock()
	return s.myIndex, s.acc
}
