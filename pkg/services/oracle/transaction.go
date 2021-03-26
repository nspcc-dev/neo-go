package oracle

import (
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

type (
	incompleteTx struct {
		sync.RWMutex
		// isSent is true tx was already broadcasted.
		isSent bool
		// attempts is how many times request was processed.
		attempts int
		// time is the time when request was last processed.
		time time.Time
		// request is oracle request.
		request *state.OracleRequest
		// tx is oracle response transaction.
		tx *transaction.Transaction
		// sigs contains signature from every oracle node.
		sigs map[string]*txSignature
		// backupTx is backup transaction.
		backupTx *transaction.Transaction
		// backupSigs contains signatures of backup tx.
		backupSigs map[string]*txSignature
	}

	txSignature struct {
		// pub is cached public key.
		pub *keys.PublicKey
		// ok is true if signature was verified.
		ok bool
		// sig is tx signature.
		sig []byte
	}
)

func newIncompleteTx() *incompleteTx {
	return &incompleteTx{
		sigs:       make(map[string]*txSignature),
		backupSigs: make(map[string]*txSignature),
	}
}

func (t *incompleteTx) reverifyTx(net netmode.Magic) {
	txHash := hash.NetSha256(uint32(net), t.tx)
	backupHash := hash.NetSha256(uint32(net), t.backupTx)
	for pub, sig := range t.sigs {
		if !sig.ok {
			sig.ok = sig.pub.Verify(sig.sig, txHash.BytesBE())
			if !sig.ok && sig.pub.Verify(sig.sig, backupHash.BytesBE()) {
				t.backupSigs[pub] = &txSignature{
					pub: sig.pub,
					ok:  true,
					sig: sig.sig,
				}
			}
		}
	}
}

func (t *incompleteTx) addResponse(pub *keys.PublicKey, sig []byte, isBackup bool) {
	tx, sigs := t.tx, t.sigs
	if isBackup {
		tx, sigs = t.backupTx, t.backupSigs
	}
	sigs[string(pub.Bytes())] = &txSignature{
		pub: pub,
		ok:  tx != nil,
		sig: sig,
	}

}

// finalize checks is either main or backup tx has sufficient number of signatures and returns
// tx and bool value indicating if it is ready to be broadcasted.
func (t *incompleteTx) finalize(oracleNodes keys.PublicKeys, backupOnly bool) (*transaction.Transaction, bool) {
	if !backupOnly && finalizeTx(oracleNodes, t.tx, t.sigs) {
		return t.tx, true
	}
	return t.backupTx, finalizeTx(oracleNodes, t.backupTx, t.backupSigs)
}

func finalizeTx(oracleNodes keys.PublicKeys, tx *transaction.Transaction, txSigs map[string]*txSignature) bool {
	if tx == nil {
		return false
	}
	m := smartcontract.GetDefaultHonestNodeCount(len(oracleNodes))
	sigs := make([][]byte, 0, m)
	for _, pub := range oracleNodes {
		sig, ok := txSigs[string(pub.Bytes())]
		if ok && sig.ok {
			sigs = append(sigs, sig.sig)
			if len(sigs) == m {
				break
			}
		}
	}
	if len(sigs) != m {
		return false
	}

	w := io.NewBufBinWriter()
	for i := range sigs {
		emit.Bytes(w.BinWriter, sigs[i])
	}
	tx.Scripts[1].InvocationScript = w.Bytes()
	return true
}

func (t *incompleteTx) getRequest() *state.OracleRequest {
	t.RLock()
	defer t.RUnlock()
	return t.request
}

func (t *incompleteTx) getTime() time.Time {
	t.RLock()
	defer t.RUnlock()
	return t.time
}
