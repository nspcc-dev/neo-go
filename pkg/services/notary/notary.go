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
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

type (
	// Notary represents Notary module.
	Notary struct {
		Config Config

		Network netmode.Magic

		// onTransaction is a callback for completed transactions (mains or fallbacks) sending.
		onTransaction func(tx *transaction.Transaction) error

		// reqMtx protects requests list.
		reqMtx sync.RWMutex
		// requests represents the map of main transactions which needs to be completed
		// with the associated fallback transactions grouped by the main transaction hash
		requests map[util.Uint256]*request

		// accMtx protects account.
		accMtx      sync.RWMutex
		currAccount *wallet.Account
		wallet      *wallet.Wallet

		mp *mempool.Pool
		// requests channel
		reqCh    chan mempool.Event
		blocksCh chan *block.Block
		stopCh   chan struct{}
	}

	// Config represents external configuration for Notary module.
	Config struct {
		MainCfg config.P2PNotary
		Chain   blockchainer.Blockchainer
		Log     *zap.Logger
	}
)

// request represents Notary service request.
type request struct {
	typ RequestType
	// isSent indicates whether main transaction was successfully sent to the network.
	isSent bool
	main   *transaction.Transaction
	// minNotValidBefore is the minimum NVB value among fallbacks transactions.
	// We stop trying to send mainTx to the network if the chain reaches minNotValidBefore height.
	minNotValidBefore uint32
	fallbacks         []*transaction.Transaction
	// nSigs is the number of signatures to be collected.
	// nSigs == nKeys for standard signature request;
	// nSigs <= nKeys for multisignature request.
	// nSigs is 0 when all received requests were invalid, so check request.typ before access to nSigs.
	nSigs uint8
	// nSigsCollected is the number of already collected signatures
	nSigsCollected uint8

	// sigs is a map of partial multisig invocation scripts [opcode.PUSHDATA1+64+signatureBytes] grouped by public keys
	sigs map[*keys.PublicKey][]byte
}

// NewNotary returns new Notary module.
func NewNotary(cfg Config, net netmode.Magic, mp *mempool.Pool, onTransaction func(tx *transaction.Transaction) error) (*Notary, error) {
	w := cfg.MainCfg.UnlockWallet
	wallet, err := wallet.NewWalletFromFile(w.Path)
	if err != nil {
		return nil, err
	}

	haveAccount := false
	for _, acc := range wallet.Accounts {
		if err := acc.Decrypt(w.Password); err == nil {
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
		wallet:        wallet,
		onTransaction: onTransaction,
		mp:            mp,
		reqCh:         make(chan mempool.Event),
		blocksCh:      make(chan *block.Block),
		stopCh:        make(chan struct{}),
	}, nil
}

// Run runs Notary module and should be called in a separate goroutine.
func (n *Notary) Run() {
	n.Config.Chain.SubscribeForBlocks(n.blocksCh)
	n.mp.SubscribeForTransactions(n.reqCh)
	for {
		select {
		case <-n.stopCh:
			n.mp.UnsubscribeFromTransactions(n.reqCh)
			n.Config.Chain.UnsubscribeFromBlocks(n.blocksCh)
			return
		case event := <-n.reqCh:
			if req, ok := event.Data.(*payload.P2PNotaryRequest); ok {
				switch event.Type {
				case mempool.TransactionAdded:
					n.OnNewRequest(req)
				case mempool.TransactionRemoved:
					n.OnRequestRemoval(req)
				}
			}
		case <-n.blocksCh:
			// new block was added, need to check for valid fallbacks
			n.PostPersist()
		}
	}
}

// Stop shutdowns Notary module.
func (n *Notary) Stop() {
	close(n.stopCh)
}

// OnNewRequest is a callback method which is called after new notary request is added to the notary request pool.
func (n *Notary) OnNewRequest(payload *payload.P2PNotaryRequest) {
	if n.getAccount() == nil {
		return
	}

	nvbFallback := payload.FallbackTransaction.GetAttributes(transaction.NotValidBeforeT)[0].Value.(*transaction.NotValidBefore).Height
	nKeys := payload.MainTransaction.GetAttributes(transaction.NotaryAssistedT)[0].Value.(*transaction.NotaryAssisted).NKeys
	typ, nSigs, pubs, validationErr := n.verifyIncompleteWitnesses(payload.MainTransaction, nKeys)
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
		if r.typ == Unknown && validationErr == nil {
			r.typ = typ
			r.nSigs = nSigs
		}
	} else {
		r = &request{
			nSigs:             nSigs,
			main:              payload.MainTransaction,
			typ:               typ,
			minNotValidBefore: nvbFallback,
		}
		n.requests[payload.MainTransaction.Hash()] = r
	}
	r.fallbacks = append(r.fallbacks, payload.FallbackTransaction)
	if exists && r.typ != Unknown && r.nSigsCollected >= r.nSigs { // already collected sufficient number of signatures to complete main transaction
		return
	}
	if validationErr == nil {
	loop:
		for i, w := range payload.MainTransaction.Scripts {
			if payload.MainTransaction.Signers[i].Account.Equals(n.Config.Chain.GetNotaryContractScriptHash()) {
				continue
			}
			if len(w.InvocationScript) != 0 && len(w.VerificationScript) != 0 {
				switch r.typ {
				case Signature:
					if !exists {
						r.nSigsCollected++
					} else if len(r.main.Scripts[i].InvocationScript) == 0 { // need this check because signature can already be added (consider receiving the same payload multiple times)
						r.main.Scripts[i] = w
						r.nSigsCollected++
					}
					if r.nSigsCollected == r.nSigs {
						break loop
					}
				case MultiSignature:
					if r.sigs == nil {
						r.sigs = make(map[*keys.PublicKey][]byte)
					}

					hash := hash.NetSha256(uint32(n.Network), r.main).BytesBE()
					for _, pub := range pubs {
						if r.sigs[pub] != nil {
							continue // signature for this pub has already been added
						}
						if pub.Verify(w.InvocationScript[2:], hash) { // then pub is the owner of the signature
							r.sigs[pub] = w.InvocationScript
							r.nSigsCollected++
							if r.nSigsCollected == r.nSigs {
								var invScript []byte
								for j := range pubs {
									if sig, ok := r.sigs[pubs[j]]; ok {
										invScript = append(invScript, sig...)
									}
								}
								r.main.Scripts[i].InvocationScript = invScript
							}
							break loop
						}
					}
					// pubKey was not found for the signature i.e. signature is bad - we're OK with that, let the fallback TX to be added
					break loop // only one multisignature is allowed
				}
			}
		}
	}
	if r.typ != Unknown && r.nSigsCollected == nSigs && r.minNotValidBefore > n.Config.Chain.BlockHeight() {
		if err := n.finalize(r.main); err != nil {
			n.Config.Log.Error("failed to finalize main transaction", zap.Error(err))
		} else {
			r.isSent = true
		}
	}
}

// OnRequestRemoval is a callback which is called after fallback transaction is removed
// from the notary payload pool due to expiration, main tx appliance or any other reason.
func (n *Notary) OnRequestRemoval(pld *payload.P2PNotaryRequest) {
	if n.getAccount() == nil {
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

// PostPersist is a callback which is called after new block event is received.
// PostPersist must not be called under the blockchain lock, because it uses finalization function.
func (n *Notary) PostPersist() {
	if n.getAccount() == nil {
		return
	}

	n.reqMtx.Lock()
	defer n.reqMtx.Unlock()
	currHeight := n.Config.Chain.BlockHeight()
	for h, r := range n.requests {
		if !r.isSent && r.typ != Unknown && r.nSigs == r.nSigsCollected && r.minNotValidBefore > currHeight {
			if err := n.finalize(r.main); err != nil {
				n.Config.Log.Error("failed to finalize main transaction", zap.Error(err))
			} else {
				r.isSent = true
			}
			continue
		}
		if r.minNotValidBefore <= currHeight { // then at least one of the fallbacks can already be sent.
			newFallbacks := r.fallbacks[:0]
			for _, fb := range r.fallbacks {
				if nvb := fb.GetAttributes(transaction.NotValidBeforeT)[0].Value.(*transaction.NotValidBefore).Height; nvb <= currHeight {
					if err := n.finalize(fb); err != nil {
						newFallbacks = append(newFallbacks, fb) // wait for the next block to resend them
					}
				} else {
					newFallbacks = append(newFallbacks, fb)
				}
			}
			if len(newFallbacks) == 0 {
				delete(n.requests, h)
			} else {
				r.fallbacks = newFallbacks
			}
		}
	}
}

// finalize adds missing Notary witnesses to the transaction (main or fallback) and pushes it to the network.
func (n *Notary) finalize(tx *transaction.Transaction) error {
	acc := n.getAccount()
	if acc == nil {
		panic(errors.New("no available Notary account")) // unreachable code, because all callers of `finalize` check that acc != nil
	}
	notaryWitness := transaction.Witness{
		InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), 64}, acc.PrivateKey().SignHashable(uint32(n.Network), tx)...),
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

	return n.onTransaction(newTx)
}

// updateTxSize returns transaction with re-calculated size and an error.
func updateTxSize(tx *transaction.Transaction) (*transaction.Transaction, error) {
	bw := io.NewBufBinWriter()
	tx.EncodeBinary(bw.BinWriter)
	if bw.Err != nil {
		return nil, fmt.Errorf("encode binary: %w", bw.Err)
	}
	return transaction.NewTransactionFromBytes(tx.Bytes())
}

// verifyIncompleteWitnesses checks that tx either doesn't have all witnesses attached (in this case none of them
// can be multisignature), or it only has a partial multisignature. It returns the request type (sig/multisig), the
// number of signatures to be collected, sorted public keys (for multisig request only) and an error.
func (n *Notary) verifyIncompleteWitnesses(tx *transaction.Transaction, nKeys uint8) (RequestType, uint8, keys.PublicKeys, error) {
	var (
		typ         RequestType
		nSigs       int
		nKeysActual uint8
		pubsBytes   [][]byte
		pubs        keys.PublicKeys
		ok          bool
	)
	if len(tx.Signers) < 2 {
		return Unknown, 0, nil, errors.New("transaction should have at least 2 signers")
	}
	if !tx.HasSigner(n.Config.Chain.GetNotaryContractScriptHash()) {
		return Unknown, 0, nil, fmt.Errorf("P2PNotary contract should be a signer of the transaction")
	}

	for i, w := range tx.Scripts {
		// do not check witness for Notary contract -- it will be replaced by proper witness in any case.
		if tx.Signers[i].Account == n.Config.Chain.GetNotaryContractScriptHash() {
			continue
		}
		if len(w.VerificationScript) == 0 {
			// then it's a contract verification (can be combined with anything)
			continue
		}
		if !tx.Signers[i].Account.Equals(hash.Hash160(w.VerificationScript)) { // https://github.com/nspcc-dev/neo-go/pull/1658#discussion_r564265987
			return Unknown, 0, nil, fmt.Errorf("transaction should have valid verification script for signer #%d", i)
		}
		if nSigs, pubsBytes, ok = vm.ParseMultiSigContract(w.VerificationScript); ok {
			if typ == Signature || typ == MultiSignature {
				return Unknown, 0, nil, fmt.Errorf("bad type of witness #%d: only one multisignature witness is allowed", i)
			}
			typ = MultiSignature
			nKeysActual = uint8(len(pubsBytes))
			if len(w.InvocationScript) != 66 || !bytes.HasPrefix(w.InvocationScript, []byte{byte(opcode.PUSHDATA1), 64}) {
				return Unknown, 0, nil, fmt.Errorf("multisignature invocation script should have length = 66 and be of the form [PUSHDATA1, 64, signatureBytes...]")
			}
			continue
		}
		if vm.IsSignatureContract(w.VerificationScript) {
			if typ == MultiSignature {
				return Unknown, 0, nil, fmt.Errorf("bad type of witness #%d: multisignature witness can not be combined with other witnesses", i)
			}
			typ = Signature
			nSigs = int(nKeys)
			continue
		}
		return Unknown, 0, nil, fmt.Errorf("unable to define the type of witness #%d", i)
	}
	switch typ {
	case Signature:
		if len(tx.Scripts) < int(nKeys+1) {
			return Unknown, 0, nil, fmt.Errorf("transaction should comtain at least %d witnesses (1 for notary + nKeys)", nKeys+1)
		}
	case MultiSignature:
		if nKeysActual != nKeys {
			return Unknown, 0, nil, fmt.Errorf("bad m out of n partial multisignature witness: expected n = %d, got n = %d", nKeys, nKeysActual)
		}
		pubs = make(keys.PublicKeys, len(pubsBytes))
		for i, pBytes := range pubsBytes {
			pub, err := keys.NewPublicKeyFromBytes(pBytes, elliptic.P256())
			if err != nil {
				return Unknown, 0, nil, fmt.Errorf("invalid bytes of #%d public key: %s", i, hex.EncodeToString(pBytes))
			}
			pubs[i] = pub
		}
	default:
		return Unknown, 0, nil, errors.New("unexpected Notary request type")
	}
	return typ, uint8(nSigs), pubs, nil
}
