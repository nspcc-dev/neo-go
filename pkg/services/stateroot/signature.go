package stateroot

import (
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

type (
	incompleteRoot struct {
		sync.RWMutex
		// svList is a list of state validator keys for this stateroot.
		svList keys.PublicKeys
		// isSent is true if the state root was already broadcasted.
		isSent bool
		// request is an oracle request.
		root *state.MPTRoot
		// sigs contains a signature from every oracle node.
		sigs map[string]*rootSig
		// myIndex is the index of validator for this root.
		myIndex int
		// myVote is an extensible message containing node's vote.
		myVote *payload.Extensible
		// retries is a counter of send attempts.
		retries int
	}

	rootSig struct {
		// pub is a cached public key.
		pub *keys.PublicKey
		// ok is true if the signature was verified.
		ok bool
		// sig is a state root signature.
		sig []byte
	}
)

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

func (r *incompleteRoot) isSenderNow() bool {
	if r.root == nil || r.isSent || len(r.svList) == 0 {
		return false
	}
	retries := max(r.retries, 0)
	ind := (int(r.root.Index) - retries) % len(r.svList)
	if ind < 0 {
		ind += len(r.svList)
	}
	return ind == r.myIndex
}

// finalize checks if either main or backup tx has sufficient number of signatures and returns
// tx and bool value indicating if it is ready to be broadcasted.
func (r *incompleteRoot) finalize() (*state.MPTRoot, bool) {
	if r.root == nil {
		return nil, false
	}

	m := smartcontract.GetDefaultHonestNodeCount(len(r.svList))
	sigs := make([][]byte, 0, m)
	for _, pub := range r.svList {
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

	verif, err := smartcontract.CreateDefaultMultiSigRedeemScript(r.svList)
	if err != nil {
		return nil, false
	}
	w := io.NewBufBinWriter()
	for i := range sigs {
		emit.Bytes(w.BinWriter, sigs[i])
	}
	r.root.Witness = []transaction.Witness{{
		InvocationScript:   w.Bytes(),
		VerificationScript: verif,
	}}
	return r.root, true
}
