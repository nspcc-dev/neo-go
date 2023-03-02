package notary

import (
	"bytes"
	"crypto/elliptic"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type (
	// Ledger is the interface to Blockchain sufficient for Notary.
	Ledger interface {
		BlockHeight() uint32
		GetMaxVerificationGAS() int64
		GetNotaryContractScriptHash() util.Uint160
		SubscribeForBlocks(ch chan *block.Block)
		UnsubscribeFromBlocks(ch chan *block.Block)
		VerifyWitness(util.Uint160, hash.Hashable, *transaction.Witness, int64) (int64, error)
	}

	// Notary represents a Notary module.
	Notary struct {
		Config Config

		Network netmode.Magic

		// onTransaction is a callback for completed transactions (mains or fallbacks) sending.
		onTransaction func(tx *transaction.Transaction) error
		// newTxs is a channel where new transactions are sent
		// to be processed in an `onTransaction` callback.
		newTxs chan txHashPair
		// started is a status bool to protect from double start/shutdown.
		started *atomic.Bool

		// reqMtx protects requests list.
		reqMtx sync.RWMutex
		// requests represents a map of main transactions which needs to be completed
		// with the associated fallback transactions grouped by the main transaction hash
		requests map[util.Uint256]*request

		// accMtx protects account.
		accMtx      sync.RWMutex
		currAccount *wallet.Account
		wallet      *wallet.Wallet

		mp *mempool.Pool
		// requests channel
		reqCh chan mempoolevent.Event
		// blocksCh is a channel used to receive block notifications from the
		// Blockchain. It is not buffered intentionally, as it's important to keep
		// the notary request pool in sync with the current blockchain heigh, thus,
		// it's not recommended to use a large size of notary requests pool as it may
		// slow down the block processing.
		blocksCh chan *block.Block
		stopCh   chan struct{}
		done     chan struct{}
	}

	// Config represents external configuration for Notary module.
	Config struct {
		MainCfg config.P2PNotary
		Chain   Ledger
		Log     *zap.Logger
	}
)

const defaultTxChannelCapacity = 100

type (
	// request represents Notary service request.
	request struct {
		// isSent indicates whether the main transaction was successfully sent to the network.
		isSent bool
		main   *transaction.Transaction
		// minNotValidBefore is the minimum NVB value among fallbacks transactions.
		// We stop trying to send the mainTx to the network if the chain reaches the minNotValidBefore height.
		minNotValidBefore uint32
		fallbacks         []*transaction.Transaction

		witnessInfo []witnessInfo
	}

	// witnessInfo represents information about the signer and its witness.
	witnessInfo struct {
		typ RequestType
		// nSigsLeft is the number of signatures left to collect to complete the main transaction.
		// Initial nSigsLeft value is defined as following:
		// nSigsLeft == nKeys for standard signature request;
		// nSigsLeft <= nKeys for multisignature request;
		nSigsLeft uint8

		// sigs is a map of partial multisig invocation scripts [opcode.PUSHDATA1+64+signatureBytes] grouped by public keys.
		sigs map[*keys.PublicKey][]byte
		// pubs is a set of public keys participating in the multisignature witness collection.
		pubs keys.PublicKeys
	}
)

// isMainCompleted denotes whether all signatures for the main transaction were collected.
func (r request) isMainCompleted() bool {
	if r.witnessInfo == nil {
		return false
	}
	for _, wi := range r.witnessInfo {
		if wi.nSigsLeft != 0 {
			return false
		}
	}
	return true
}

// NewNotary returns a new Notary module.
func NewNotary(cfg Config, net netmode.Magic, mp *mempool.Pool, onTransaction func(tx *transaction.Transaction) error) (*Notary, error) {
	w := cfg.MainCfg.UnlockWallet
	wallet, err := wallet.NewWalletFromFile(w.Path)
	if err != nil {
		return nil, err
	}

	haveAccount := false
	for _, acc := range wallet.Accounts {
		if err := acc.Decrypt(w.Password, wallet.Scrypt); err == nil {
			haveAccount = true
			break
		}
	}
	if !haveAccount {
		return nil, errors.New("no wallet account could be unlocked")
	}

	return &Notary{
		requests:      make(map[util.Uint256]*request),
		Config:        cfg,
		Network:       net,
		started:       atomic.NewBool(false),
		wallet:        wallet,
		onTransaction: onTransaction,
		newTxs:        make(chan txHashPair, defaultTxChannelCapacity),
		mp:            mp,
		reqCh:         make(chan mempoolevent.Event),
		blocksCh:      make(chan *block.Block),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
	}, nil
}

// Name returns service name.
func (n *Notary) Name() string {
	return "notary"
}

// Start runs a Notary module in a separate goroutine.
// The Notary only starts once, subsequent calls to Start are no-op.
func (n *Notary) Start() {
	if !n.started.CAS(false, true) {
		return
	}
	n.Config.Log.Info("starting notary service")
	n.Config.Chain.SubscribeForBlocks(n.blocksCh)
	n.mp.SubscribeForTransactions(n.reqCh)
	go n.newTxCallbackLoop()
	go n.mainLoop()
}

func (n *Notary) mainLoop() {
mainloop:
	for {
		select {
		case <-n.stopCh:
			n.mp.UnsubscribeFromTransactions(n.reqCh)
			n.Config.Chain.UnsubscribeFromBlocks(n.blocksCh)
			break mainloop
		case event := <-n.reqCh:
			if req, ok := event.Data.(*payload.P2PNotaryRequest); ok {
				switch event.Type {
				case mempoolevent.TransactionAdded:
					n.OnNewRequest(req)
				case mempoolevent.TransactionRemoved:
					n.OnRequestRemoval(req)
				}
			}
		case <-n.blocksCh:
			// a new block was added, we need to check for valid fallbacks
			n.PostPersist()
		}
	}
drainLoop:
	for {
		select {
		case <-n.blocksCh:
		case <-n.reqCh:
		default:
			break drainLoop
		}
	}
	close(n.blocksCh)
	close(n.reqCh)
	close(n.done)
}

// Shutdown stops the Notary module. It can only be called once, subsequent calls
// to Shutdown on the same instance are no-op. The instance that was stopped can
// not be started again by calling Start (use a new instance if needed).
func (n *Notary) Shutdown() {
	if !n.started.CAS(true, false) {
		return
	}
	n.Config.Log.Info("stopping notary service")
	close(n.stopCh)
	<-n.done
	n.wallet.Close()
}

// OnNewRequest is a callback method which is called after a new notary request is added to the notary request pool.
func (n *Notary) OnNewRequest(payload *payload.P2PNotaryRequest) {
	if !n.started.Load() {
		return
	}
	acc := n.getAccount()
	if acc == nil {
		return
	}

	nvbFallback := payload.FallbackTransaction.GetAttributes(transaction.NotValidBeforeT)[0].Value.(*transaction.NotValidBefore).Height
	nKeys := payload.MainTransaction.GetAttributes(transaction.NotaryAssistedT)[0].Value.(*transaction.NotaryAssisted).NKeys
	newInfo, validationErr := n.verifyIncompleteWitnesses(payload.MainTransaction, nKeys)
	if validationErr != nil {
		n.Config.Log.Info("verification of main notary transaction failed; fallback transaction will be completed",
			zap.String("main hash", payload.MainTransaction.Hash().StringLE()),
			zap.String("fallback hash", payload.FallbackTransaction.Hash().StringLE()),
			zap.String("verification error", validationErr.Error()))
	}
	n.reqMtx.Lock()
	defer n.reqMtx.Unlock()
	r, exists := n.requests[payload.MainTransaction.Hash()]
	if exists {
		for _, fb := range r.fallbacks {
			if fb.Hash().Equals(payload.FallbackTransaction.Hash()) {
				return // then we already have processed this request
			}
		}
		if nvbFallback < r.minNotValidBefore {
			r.minNotValidBefore = nvbFallback
		}
	} else {
		// Avoid changes in the main transaction witnesses got from the notary request pool to
		// keep the pooled tx valid. We will update its copy => the copy's size will be changed.
		cp := *payload.MainTransaction
		cp.Scripts = make([]transaction.Witness, len(payload.MainTransaction.Scripts))
		copy(cp.Scripts, payload.MainTransaction.Scripts)
		r = &request{
			main:              &cp,
			minNotValidBefore: nvbFallback,
		}
		n.requests[payload.MainTransaction.Hash()] = r
	}
	if r.witnessInfo == nil && validationErr == nil {
		r.witnessInfo = newInfo
	}
	// Allow modification of a fallback transaction got from the notary request pool.
	// It has dummy Notary witness attached => its size won't be changed.
	r.fallbacks = append(r.fallbacks, payload.FallbackTransaction)
	if exists && r.isMainCompleted() || validationErr != nil {
		return
	}
	mainHash := hash.NetSha256(uint32(n.Network), r.main).BytesBE()
	for i, w := range payload.MainTransaction.Scripts {
		if len(w.InvocationScript) == 0 || // check that signature for this witness was provided
			(r.witnessInfo[i].nSigsLeft == 0 && r.witnessInfo[i].typ != Contract) { // check that signature wasn't yet added (consider receiving the same payload multiple times)
			continue
		}
		switch r.witnessInfo[i].typ {
		case Contract:
			// Need to check even if r.main.Scripts[i].InvocationScript is already filled in.
			_, err := n.Config.Chain.VerifyWitness(r.main.Signers[i].Account, r.main, &w, n.Config.Chain.GetMaxVerificationGAS())
			if err != nil {
				continue
			}
			r.main.Scripts[i].InvocationScript = w.InvocationScript
		case Signature:
			if r.witnessInfo[i].pubs[0].Verify(w.InvocationScript[2:], mainHash) {
				r.main.Scripts[i] = w
				r.witnessInfo[i].nSigsLeft--
			}
		case MultiSignature:
			if r.witnessInfo[i].sigs == nil {
				r.witnessInfo[i].sigs = make(map[*keys.PublicKey][]byte)
			}

			for _, pub := range r.witnessInfo[i].pubs {
				if r.witnessInfo[i].sigs[pub] != nil {
					continue // signature for this pub has already been added
				}
				if pub.Verify(w.InvocationScript[2:], mainHash) { // then pub is the owner of the signature
					r.witnessInfo[i].sigs[pub] = w.InvocationScript
					r.witnessInfo[i].nSigsLeft--
					if r.witnessInfo[i].nSigsLeft == 0 {
						var invScript []byte
						for j := range r.witnessInfo[i].pubs {
							if sig, ok := r.witnessInfo[i].sigs[r.witnessInfo[i].pubs[j]]; ok {
								invScript = append(invScript, sig...)
							}
						}
						r.main.Scripts[i].InvocationScript = invScript
					}
					break
				}
			}
			// pubKey was not found for the signature (i.e. signature is bad) or the signature has already
			// been added - we're OK with that, let the fallback TX to be added
		}
	}
	if r.isMainCompleted() && r.minNotValidBefore > n.Config.Chain.BlockHeight() {
		if err := n.finalize(acc, r.main, payload.MainTransaction.Hash()); err != nil {
			n.Config.Log.Error("failed to finalize main transaction",
				zap.String("hash", r.main.Hash().StringLE()),
				zap.Error(err))
		}
	}
}

// OnRequestRemoval is a callback which is called after fallback transaction is removed
// from the notary payload pool due to expiration, main tx appliance or any other reason.
func (n *Notary) OnRequestRemoval(pld *payload.P2PNotaryRequest) {
	if !n.started.Load() || n.getAccount() == nil {
		return
	}

	n.reqMtx.Lock()
	defer n.reqMtx.Unlock()
	r, ok := n.requests[pld.MainTransaction.Hash()]
	if !ok {
		return
	}
	for i, fb := range r.fallbacks {
		if fb.Hash().Equals(pld.FallbackTransaction.Hash()) {
			r.fallbacks = append(r.fallbacks[:i], r.fallbacks[i+1:]...)
			break
		}
	}
	if len(r.fallbacks) == 0 {
		delete(n.requests, r.main.Hash())
	}
}

// PostPersist is a callback which is called after a new block event is received.
// PostPersist must not be called under the blockchain lock, because it uses finalization function.
func (n *Notary) PostPersist() {
	if !n.started.Load() {
		return
	}
	acc := n.getAccount()
	if acc == nil {
		return
	}

	n.reqMtx.Lock()
	defer n.reqMtx.Unlock()
	currHeight := n.Config.Chain.BlockHeight()
	for h, r := range n.requests {
		if !r.isSent && r.isMainCompleted() && r.minNotValidBefore > currHeight {
			if err := n.finalize(acc, r.main, h); err != nil {
				n.Config.Log.Error("failed to finalize main transaction", zap.Error(err))
			}
			continue
		}
		if r.minNotValidBefore <= currHeight { // then at least one of the fallbacks can already be sent.
			for _, fb := range r.fallbacks {
				if nvb := fb.GetAttributes(transaction.NotValidBeforeT)[0].Value.(*transaction.NotValidBefore).Height; nvb <= currHeight {
					// Ignore the error, wait for the next block to resend them
					_ = n.finalize(acc, fb, h)
				}
			}
		}
	}
}

// finalize adds missing Notary witnesses to the transaction (main or fallback) and pushes it to the network.
func (n *Notary) finalize(acc *wallet.Account, tx *transaction.Transaction, h util.Uint256) error {
	notaryWitness := transaction.Witness{
		InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, acc.SignHashable(n.Network, tx)...),
		VerificationScript: []byte{},
	}
	for i, signer := range tx.Signers {
		if signer.Account == n.Config.Chain.GetNotaryContractScriptHash() {
			tx.Scripts[i] = notaryWitness
			break
		}
	}
	newTx, err := updateTxSize(tx)
	if err != nil {
		return fmt.Errorf("failed to update completed transaction's size: %w", err)
	}

	n.pushNewTx(newTx, h)

	return nil
}

type txHashPair struct {
	tx       *transaction.Transaction
	mainHash util.Uint256
}

func (n *Notary) pushNewTx(tx *transaction.Transaction, h util.Uint256) {
	select {
	case n.newTxs <- txHashPair{tx, h}:
	default:
	}
}

func (n *Notary) newTxCallbackLoop() {
	for {
		select {
		case tx := <-n.newTxs:
			isMain := tx.tx.Hash() == tx.mainHash

			n.reqMtx.Lock()
			r, ok := n.requests[tx.mainHash]
			if !ok || isMain && (r.isSent || r.minNotValidBefore <= n.Config.Chain.BlockHeight()) {
				n.reqMtx.Unlock()
				continue
			}
			if !isMain {
				// Ensure that fallback was not already completed.
				var isPending bool
				for _, fb := range r.fallbacks {
					if fb.Hash() == tx.tx.Hash() {
						isPending = true
						break
					}
				}
				if !isPending {
					n.reqMtx.Unlock()
					continue
				}
			}

			n.reqMtx.Unlock()
			err := n.onTransaction(tx.tx)
			if err != nil {
				n.Config.Log.Error("new transaction callback finished with error", zap.Error(err))
				continue
			}

			n.reqMtx.Lock()
			if isMain {
				r.isSent = true
			} else {
				for i := range r.fallbacks {
					if r.fallbacks[i].Hash() == tx.tx.Hash() {
						r.fallbacks = append(r.fallbacks[:i], r.fallbacks[i+1:]...)
						break
					}
				}
				if len(r.fallbacks) == 0 {
					delete(n.requests, tx.mainHash)
				}
			}
			n.reqMtx.Unlock()
		case <-n.stopCh:
			return
		}
	}
}

// updateTxSize returns a transaction with re-calculated size and an error.
func updateTxSize(tx *transaction.Transaction) (*transaction.Transaction, error) {
	bw := io.NewBufBinWriter()
	tx.EncodeBinary(bw.BinWriter)
	if bw.Err != nil {
		return nil, fmt.Errorf("encode binary: %w", bw.Err)
	}
	return transaction.NewTransactionFromBytes(tx.Bytes())
}

// verifyIncompleteWitnesses checks that the tx either doesn't have all witnesses attached (in this case none of them
// can be multisignature) or it only has a partial multisignature. It returns the request type (sig/multisig), the
// number of signatures to be collected, sorted public keys (for multisig request only) and an error.
func (n *Notary) verifyIncompleteWitnesses(tx *transaction.Transaction, nKeysExpected uint8) ([]witnessInfo, error) {
	var nKeysActual uint8
	if len(tx.Signers) < 2 {
		return nil, errors.New("transaction should have at least 2 signers")
	}
	if !tx.HasSigner(n.Config.Chain.GetNotaryContractScriptHash()) {
		return nil, fmt.Errorf("P2PNotary contract should be a signer of the transaction")
	}
	result := make([]witnessInfo, len(tx.Signers))
	for i, w := range tx.Scripts {
		// Do not check witness for a Notary contract -- it will be replaced by proper witness in any case.
		// Also, do not check other contract-based witnesses (they can be combined with anything)
		if len(w.VerificationScript) == 0 {
			result[i] = witnessInfo{
				typ:       Contract,
				nSigsLeft: 0,
			}
			continue
		}
		if !tx.Signers[i].Account.Equals(hash.Hash160(w.VerificationScript)) { // https://github.com/nspcc-dev/neo-go/pull/1658#discussion_r564265987
			return nil, fmt.Errorf("transaction should have valid verification script for signer #%d", i)
		}
		// Each verification script is allowed to have either one signature or zero signatures. If signature is provided, then need to verify it.
		if len(w.InvocationScript) != 0 {
			if len(w.InvocationScript) != 66 || !bytes.HasPrefix(w.InvocationScript, []byte{byte(opcode.PUSHDATA1), keys.SignatureLen}) {
				return nil, fmt.Errorf("witness #%d: invocation script should have length = 66 and be of the form [PUSHDATA1, 64, signatureBytes...]", i)
			}
		}
		if nSigs, pubsBytes, ok := vm.ParseMultiSigContract(w.VerificationScript); ok {
			result[i] = witnessInfo{
				typ:       MultiSignature,
				nSigsLeft: uint8(nSigs),
				pubs:      make(keys.PublicKeys, len(pubsBytes)),
			}
			for j, pBytes := range pubsBytes {
				pub, err := keys.NewPublicKeyFromBytes(pBytes, elliptic.P256())
				if err != nil {
					return nil, fmt.Errorf("witness #%d: invalid bytes of #%d public key: %s", i, j, hex.EncodeToString(pBytes))
				}
				result[i].pubs[j] = pub
			}
			nKeysActual += uint8(len(pubsBytes))
			continue
		}
		if pBytes, ok := vm.ParseSignatureContract(w.VerificationScript); ok {
			pub, err := keys.NewPublicKeyFromBytes(pBytes, elliptic.P256())
			if err != nil {
				return nil, fmt.Errorf("witness #%d: invalid bytes of public key: %s", i, hex.EncodeToString(pBytes))
			}
			result[i] = witnessInfo{
				typ:       Signature,
				nSigsLeft: 1,
				pubs:      keys.PublicKeys{pub},
			}
			nKeysActual++
			continue
		}
		return nil, fmt.Errorf("witness #%d: unable to detect witness type, only sig/multisig/contract are supported", i)
	}
	if nKeysActual != nKeysExpected {
		return nil, fmt.Errorf("expected and actual NKeys mismatch: %d vs %d", nKeysExpected, nKeysActual)
	}
	return result, nil
}
