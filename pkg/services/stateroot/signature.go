package stateroot

import (
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

type (
	incompleteRoot struct {
		sync.RWMutex
		// isSent is true state root was already broadcasted.
		isSent bool
		// request is oracle request.
		root *state.MPTRoot
		// sigs contains signature from every oracle node.
		sigs map[string]*rootSig
	}

	rootSig struct {
		// pub is cached public key.
		pub *keys.PublicKey
		// ok is true if signature was verified.
		ok bool
		// sig is state root signature.
		sig []byte
	}
)

func newIncompleteRoot() *incompleteRoot {
	return &incompleteRoot{
		sigs: make(map[string]*rootSig),
	}
}

func (r *incompleteRoot) reverify(net netmode.Magic) {
	for _, sig := range r.sigs {
		if !sig.ok {
			sig.ok = sig.pub.VerifyHashable(sig.sig, uint32(net), r.root)
		}
	}
}

func (r *incompleteRoot) addSignature(pub *keys.PublicKey, sig []byte) {
	r.sigs[string(pub.Bytes())] = &rootSig{
		pub: pub,
		ok:  r.root != nil,
		sig: sig,
	}
}

// finalize checks is either main or backup tx has sufficient number of signatures and returns
// tx and bool value indicating if it is ready to be broadcasted.
func (r *incompleteRoot) finalize(stateValidators keys.PublicKeys) (*state.MPTRoot, bool) {
	if r.root == nil {
		return nil, false
	}

	m := smartcontract.GetDefaultHonestNodeCount(len(stateValidators))
	sigs := make([][]byte, 0, m)
	for _, pub := range stateValidators {
		sig, ok := r.sigs[string(pub.Bytes())]
		if ok && sig.ok {
			sigs = append(sigs, sig.sig)
			if len(sigs) == m {
				break
			}
		}
	}
	if len(sigs) != m {
		return nil, false
	}

	w := io.NewBufBinWriter()
	for i := range sigs {
		emit.Bytes(w.BinWriter, sigs[i])
	}
	r.root.Witness = &transaction.Witness{
		InvocationScript: w.Bytes(),
	}
	return r.root, true
}
