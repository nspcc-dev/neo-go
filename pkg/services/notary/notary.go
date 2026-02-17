package notary

import (
	"bytes"
	"crypto/elliptic"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/scparser"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

type (
	// Ledger is the interface to Blockchain sufficient for Notary.
	Ledger interface {
		BlockHeight() uint32
		GetMaxVerificationGAS() int64
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
		started atomic.Bool

		// reqMtx protects the request list from concurrent requests addition/removal.
		// Use per-request locks instead of this one to perform request-changing operations.
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
		lock sync.RWMutex
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
		// nSigsLeft is the number of verified signatures left to be collected to
		// complete the main transaction. Zero means that all required signatures
		// are collected and verified. Initial nSigsLeft value is defined as
		// following:
		// nSigsLeft == nKeys for standard [Signature] request;
		// nSigsLeft <= nKeys for [MultiSignature] request;
		// nSigsLeft == 0 for [Contract] witness request;
		// nSigsLeft <= nKeys for AppCall request.
		nSigsLeft uint8

		// sigs is a map of partial multisig invocation scripts [opcode.PUSHDATA1+64+signatureBytes] grouped by public keys.
		sigs map[*keys.PublicKey][]byte
		// pubs is a set of public keys participating in the multisignature witness collection.
		pubs keys.PublicKeys

		// args is a list of invocation script parts for AppCall witness. Duplicates are allowed. No partial verification
		// is supported. No sorting is performed.
		args [][]byte
	}
)

// isMainCompleted denotes whether all signatures for the main transaction were collected.
// The caller must hold the request RLock.
func (r *request) isMainCompleted() bool {
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
	wall, err := wallet.NewWalletFromFile(w.Path)
	if err != nil {
		return nil, err
	}

	var haveAccount = slices.ContainsFunc(wall.Accounts, func(acc *wallet.Account) bool {
		return acc.Decrypt(w.Password, wall.Scrypt) == nil
	})
	if !haveAccount {
		return nil, errors.New("no wallet account could be unlocked")
	}

	return &Notary{
		requests:      make(map[util.Uint256]*request),
		Config:        cfg,
		Network:       net,
		wallet:        wall,
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
	if !n.started.CompareAndSwap(false, true) {
		return
	}
	n.Config.Log.Info("starting notary service")
	go n.newTxCallbackLoop()
	go n.mainLoop()
}

func (n *Notary) mainLoop() {
	n.Config.Chain.SubscribeForBlocks(n.blocksCh)
	n.mp.SubscribeForTransactions(n.reqCh)
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
	if !n.started.CompareAndSwap(true, false) {
		return
	}
	n.Config.Log.Info("stopping notary service")
	close(n.stopCh)
	<-n.done
	n.wallet.Close()
	_ = n.Config.Log.Sync()
}

// IsAuthorized returns whether Notary service currently is authorized to collect
// signatures. It returnes true iff designated Notary node's account provided to
// the Notary service in decrypted state.
func (n *Notary) IsAuthorized() bool {
	return n.getAccount() != nil
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
	newInfo, validationErr := verifyIncompleteWitnesses(payload.MainTransaction, nKeys)
	if validationErr != nil {
		n.Config.Log.Info("verification of main notary transaction failed; fallback transaction will be completed",
			zap.String("main hash", payload.MainTransaction.Hash().StringLE()),
			zap.String("fallback hash", payload.FallbackTransaction.Hash().StringLE()),
			zap.String("verification error", validationErr.Error()))
	}
	n.reqMtx.Lock()
	r, exists := n.requests[payload.MainTransaction.Hash()]
	if exists {
		r.lock.Lock() // RLock doesn't fit here since we modify r.minNotValidBefore below.
		defer r.lock.Unlock()
		if slices.ContainsFunc(r.fallbacks, func(fb *transaction.Transaction) bool {
			return fb.Hash().Equals(payload.FallbackTransaction.Hash())
		}) {
			n.reqMtx.Unlock()
			return // then we already have processed this request
		}
		r.minNotValidBefore = min(r.minNotValidBefore, nvbFallback)
	} else {
		// Avoid changes in the main transaction witnesses got from the notary request pool to
		// keep the pooled tx valid. We will update its copy => the copy's size will be changed.
		r = &request{
			main:              payload.MainTransaction.Copy(),
			minNotValidBefore: nvbFallback,
		}
		r.lock.Lock()
		defer r.lock.Unlock()
		n.requests[payload.MainTransaction.Hash()] = r
	}
	n.reqMtx.Unlock()
	if r.witnessInfo == nil && validationErr == nil {
		r.witnessInfo = newInfo
	}
	// Disallow modification of a fallback transaction got from the notary
	// request pool. Even though it has dummy Notary witness attached and its
	// size won't be changed after finalisation, the witness bytes changes may
	// affect the other users of notary pool and cause race. Avoid this by making
	// the copy.
	r.fallbacks = append(r.fallbacks, payload.FallbackTransaction.Copy())
	if exists && r.isMainCompleted() || validationErr != nil {
		return
	}
	mainHash := hash.NetSha256(uint32(n.Network), r.main).BytesBE()
	for i, w := range payload.MainTransaction.Scripts {
		// Check that request provides signature for this witness. For contract
		// accounts only empty invocation scripts are supported with no
		// verification implied.
		if (len(w.InvocationScript) == 0 && r.witnessInfo[i].typ != Contract) ||
			// Check if *valid* signature was already collected (consider receiving
			// malicious request with extra intentionally wrong witness or receiving
			// extra valid signatures for M out of N multisignature request).
			r.witnessInfo[i].nSigsLeft == 0 {
			continue
		}
		switch r.witnessInfo[i].typ {
		case Contract:
			// Will support non-empty invocation scripts in the future.
			continue
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
		case AppCall:
			if r.witnessInfo[i].args == nil {
				r.witnessInfo[i].args = make([][]byte, 0, r.witnessInfo[i].nSigsLeft)
			}
			// No verification for AppCall is supported - technically, anyone is allowed to mess up main tx,
			// but the service is not able to verify parts of invocation script for custom appcall.
			r.witnessInfo[i].args = append(r.witnessInfo[i].args, w.InvocationScript)

			args, err := scparser.ParseSomething(w.InvocationScript, true)
			if err != nil {
				continue
			}
			if len(args) > int(r.witnessInfo[i].nSigsLeft) {
				continue
			}
			r.witnessInfo[i].nSigsLeft -= byte(len(args))
			if r.witnessInfo[i].nSigsLeft == 0 {
				var invScript []byte
				for _, part := range r.witnessInfo[i].args {
					invScript = append(invScript, part...)
				}
				r.main.Scripts[i].InvocationScript = invScript
			}
		}
	}
	if r.isMainCompleted() && r.minNotValidBefore > n.Config.Chain.BlockHeight() {
		if err := n.finalize(acc, r.main, payload.MainTransaction.Hash()); err != nil {
			n.Config.Log.Error("failed to finalize main transaction, waiting for the next block to retry",
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

	n.reqMtx.RLock()
	r, ok := n.requests[pld.MainTransaction.Hash()]
	n.reqMtx.RUnlock()
	if !ok {
		return
	}

	r.lock.Lock()
	for i, fb := range r.fallbacks {
		if fb.Hash().Equals(pld.FallbackTransaction.Hash()) {
			r.fallbacks = append(r.fallbacks[:i], r.fallbacks[i+1:]...)
			break
		}
	}
	r.lock.Unlock()
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
		r.lock.Lock()
		if len(r.fallbacks) == 0 {
			delete(n.requests, r.main.Hash())
			r.lock.Unlock()
			continue
		}
		if !r.isSent && r.isMainCompleted() && r.minNotValidBefore > currHeight {
			if err := n.finalize(acc, r.main, h); err != nil {
				n.Config.Log.Error("failed to finalize main transaction after PostPersist, waiting for the next block to retry",
					zap.String("hash", r.main.Hash().StringLE()),
					zap.Error(err))
			}
			r.lock.Unlock()
			continue
		}
		if r.minNotValidBefore <= currHeight { // then at least one of the fallbacks can already be sent.
			for _, fb := range r.fallbacks {
				if nvb := fb.GetAttributes(transaction.NotValidBeforeT)[0].Value.(*transaction.NotValidBefore).Height; nvb <= currHeight {
					// Ignore the error, wait for the next block to resend them
					err := n.finalize(acc, fb, h)
					if err != nil {
						n.Config.Log.Error("failed to finalize fallback transaction, waiting for the next block to retry",
							zap.String("hash", fb.Hash().StringLE()),
							zap.Error(err))
					}
				}
			}
		}
		r.lock.Unlock()
	}
}

// finalize adds missing Notary witnesses to the transaction (main or fallback) and pushes it to the network.
func (n *Notary) finalize(acc *wallet.Account, tx *transaction.Transaction, h util.Uint256) error {
	notaryWitness := transaction.Witness{
		InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), keys.SignatureLen}, acc.SignHashable(n.Network, tx)...),
		VerificationScript: []byte{},
	}
	for i, signer := range tx.Signers {
		if signer.Account == nativehashes.Notary {
			tx.Scripts[i] = notaryWitness
			break
		}
	}
	newTx, err := updateTxSize(tx)
	if err != nil {
		return fmt.Errorf("failed to update completed transaction's size: %w", err)
	}

	err = n.pushNewTx(newTx, h)
	if err != nil {
		return fmt.Errorf("failed to enqueue completed transaction: %w", err)
	}

	return nil
}

type txHashPair struct {
	tx       *transaction.Transaction
	mainHash util.Uint256
}

func (n *Notary) pushNewTx(tx *transaction.Transaction, h util.Uint256) error {
	select {
	case n.newTxs <- txHashPair{tx, h}:
	default:
		return errors.New("transaction queue is full")
	}
	return nil
}

func (n *Notary) newTxCallbackLoop() {
	for {
		select {
		case tx := <-n.newTxs:
			isMain := tx.tx.Hash() == tx.mainHash

			n.reqMtx.RLock()
			r, ok := n.requests[tx.mainHash]
			n.reqMtx.RUnlock()
			if !ok {
				continue
			}
			r.lock.RLock()
			if isMain && (r.isSent || r.minNotValidBefore <= n.Config.Chain.BlockHeight()) {
				r.lock.RUnlock()
				continue
			}
			if !isMain {
				// Ensure that fallback was not already completed.
				var isPending = slices.ContainsFunc(r.fallbacks, func(fb *transaction.Transaction) bool {
					return fb.Hash() == tx.tx.Hash()
				})
				if !isPending {
					r.lock.RUnlock()
					continue
				}
			}

			// Do not take lock over r during onTransaction processing; it may cause
			// a deadlock on attempt to mempool finalized transaction, ref. #2064,
			// ref. https://github.com/nspcc-dev/neo-go/pull/4093#issuecomment-3682809371.
			r.lock.RUnlock()
			err := n.onTransaction(tx.tx)
			if err != nil {
				n.Config.Log.Error("new transaction callback finished with error",
					zap.Error(err),
					zap.Bool("is main", isMain))
				continue
			}

			r.lock.Lock()
			if isMain {
				r.isSent = true
			} else {
				for i := range r.fallbacks {
					if r.fallbacks[i].Hash() == tx.tx.Hash() {
						r.fallbacks = append(r.fallbacks[:i], r.fallbacks[i+1:]...)
						break
					}
				}
			}
			r.lock.Unlock()
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
func verifyIncompleteWitnesses(tx *transaction.Transaction, nKeysExpected uint8) ([]witnessInfo, error) {
	var nKeysActual uint8
	if len(tx.Signers) < 2 {
		return nil, errors.New("transaction should have at least 2 signers")
	}
	if !tx.HasSigner(nativehashes.Notary) {
		return nil, fmt.Errorf("P2PNotary contract should be a signer of the transaction")
	}
	result := make([]witnessInfo, len(tx.Signers))
	for i, w := range tx.Scripts {
		// Do not check (and count) the contract witness -- Notary one will be replaced by proper witness in any case,
		// other contract witnesses must have empty verification scripts since they are not included into NKeys.
		if len(w.VerificationScript) == 0 {
			if len(w.InvocationScript) != 0 {
				return nil, fmt.Errorf("witness #%d: only empty invocation scripts are supported for contract accounts", i)
			}
			result[i] = witnessInfo{
				typ:       Contract,
				nSigsLeft: 0,
			}
			continue
		}
		if !tx.Signers[i].Account.Equals(hash.Hash160(w.VerificationScript)) { // https://github.com/nspcc-dev/neo-go/pull/1658#discussion_r564265987
			return nil, fmt.Errorf("transaction should have valid verification script for signer #%d", i)
		}
		if nSigs, pubsBytes, ok := scparser.ParseMultiSigContract(w.VerificationScript); ok {
			err := verifyIncompleteStandardInvocationScript(w.InvocationScript)
			if err != nil {
				return nil, fmt.Errorf("witness #%d: %w", i, err)
			}
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
		if pBytes, ok := scparser.ParseSignatureContract(w.VerificationScript); ok {
			err := verifyIncompleteStandardInvocationScript(w.InvocationScript)
			if err != nil {
				return nil, fmt.Errorf("witness #%d: %w", i, err)
			}
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
		n, m, err := ParseAppCallContract(w.VerificationScript)
		left := uint8(n - m)
		if err == nil {
			result[i] = witnessInfo{
				typ:       AppCall,
				nSigsLeft: left,
			}
			nKeysActual += left
			continue
		}
		return nil, fmt.Errorf("witness #%d: unable to detect witness type, only sig/multisig/contract are supported, custom AppCall parsing failed: %w", i, err)
	}
	if nKeysActual != nKeysExpected {
		return nil, fmt.Errorf("expected and actual NKeys mismatch: %d vs %d", nKeysExpected, nKeysActual)
	}
	return result, nil
}

// verifyIncompleteStandardInvocationScript verifies verification script for standard signature or multisignature contract,
// (it is allowed to have either one signature or zero signatures). If signature is provided, then it will be verified later.
func verifyIncompleteStandardInvocationScript(inv []byte) error {
	if len(inv) != 0 && (len(inv) != 66 || !bytes.HasPrefix(inv, []byte{byte(opcode.PUSHDATA1), keys.SignatureLen})) {
		return errors.New("invocation script should have length = 66 and be of the form [PUSHDATA1, 64, signatureBytes...]")
	}
	return nil
}

// ParseAppCallContract tries to parse System.Contract.Call with an arbitrary
// (>= 0) number of contract call arguments missing. It follows
// [scparser.ParseAppCall] rules. If successful, it returns N - the overall
// number of contract call arguments (defined by the PACK's parameter) and M -
// the number of those which are already present in the given script.
func ParseAppCallContract(script []byte) (int, int, error) {
	_, _, _, args, err := scparser.ParseAppCallNonStrict(script)
	if err != nil {
		return 0, 0, err
	}

	var (
		n = len(args)
		m int
	)
	for _, arg := range args {
		if !arg.IsEmpty() {
			m++
		}
	}

	return n, m, nil
}
