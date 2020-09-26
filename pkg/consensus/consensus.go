package consensus

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/dbft/block"
	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/dbft/merkle"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/core"
	coreb "github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/cache"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// cacheMaxCapacity is the default cache capacity taken
// from C# implementation https://github.com/neo-project/neo/blob/master/neo/Ledger/Blockchain.cs#L64
const cacheMaxCapacity = 100

// defaultTimePerBlock is a period between blocks which is used in NEO.
const defaultTimePerBlock = 15 * time.Second

// Service represents consensus instance.
type Service interface {
	// Start initializes dBFT and starts event loop for consensus service.
	// It must be called only when sufficient amount of peers are connected.
	Start()

	// OnPayload is a callback to notify Service about new received payload.
	OnPayload(p *Payload)
	// OnTransaction is a callback to notify Service about new received transaction.
	OnTransaction(tx *transaction.Transaction)
	// GetPayload returns Payload with specified hash if it is present in the local cache.
	GetPayload(h util.Uint256) *Payload
}

type service struct {
	Config

	log *zap.Logger
	// cache is a fifo cache which stores recent payloads.
	cache *cache.HashCache
	// txx is a fifo cache which stores miner transactions.
	txx  *cache.HashCache
	dbft *dbft.DBFT
	// messages and transactions are channels needed to process
	// everything in single thread.
	messages     chan Payload
	transactions chan *transaction.Transaction
	// blockEvents is used to pass a new block event to the consensus
	// process.
	blockEvents  chan *coreb.Block
	lastProposal []util.Uint256
	wallet       *wallet.Wallet
	// started is a flag set with Start method that runs an event handling
	// goroutine.
	started *atomic.Bool
}

// Config is a configuration for consensus services.
type Config struct {
	// Logger is a logger instance.
	Logger *zap.Logger
	// Broadcast is a callback which is called to notify server
	// about new consensus payload to sent.
	Broadcast func(cache.Hashable)
	// Chain is a core.Blockchainer instance.
	Chain core.Blockchainer
	// RequestTx is a callback to which will be called
	// when a node lacks transactions present in a block.
	RequestTx func(h ...util.Uint256)
	// TimePerBlock minimal time that should pass before next block is accepted.
	TimePerBlock time.Duration
	// Wallet is a local-node wallet configuration.
	Wallet *wallet.Config
}

// NewService returns new consensus.Service instance.
func NewService(cfg Config) (Service, error) {
	if cfg.TimePerBlock <= 0 {
		cfg.TimePerBlock = defaultTimePerBlock
	}

	if cfg.Logger == nil {
		return nil, errors.New("empty logger")
	}

	srv := &service{
		Config: cfg,

		log:      cfg.Logger,
		cache:    cache.NewFIFOCache(cacheMaxCapacity),
		txx:      cache.NewFIFOCache(cacheMaxCapacity),
		messages: make(chan Payload, 100),

		transactions: make(chan *transaction.Transaction, 100),
		blockEvents:  make(chan *coreb.Block, 1),
		started:      atomic.NewBool(false),
	}

	if cfg.Wallet == nil {
		return srv, nil
	}

	var err error

	if srv.wallet, err = wallet.NewWalletFromFile(cfg.Wallet.Path); err != nil {
		return nil, err
	}

	// Check that wallet password is correct for at least one account.
	var ok bool
	for _, acc := range srv.wallet.Accounts {
		err := acc.Decrypt(srv.Config.Wallet.Password)
		if err == nil {
			ok = true
			break
		}
	}
	if !ok {
		return nil, errors.New("no account with provided password was found")
	}

	defer srv.wallet.Close()

	srv.dbft = dbft.New(
		dbft.WithLogger(srv.log),
		dbft.WithSecondsPerBlock(cfg.TimePerBlock),
		dbft.WithGetKeyPair(srv.getKeyPair),
		dbft.WithRequestTx(cfg.RequestTx),
		dbft.WithGetTx(srv.getTx),
		dbft.WithGetVerified(srv.getVerifiedTx),
		dbft.WithBroadcast(srv.broadcast),
		dbft.WithProcessBlock(srv.processBlock),
		dbft.WithVerifyBlock(srv.verifyBlock),
		dbft.WithGetBlock(srv.getBlock),
		dbft.WithWatchOnly(func() bool { return false }),
		dbft.WithNewBlockFromContext(srv.newBlockFromContext),
		dbft.WithCurrentHeight(cfg.Chain.BlockHeight),
		dbft.WithCurrentBlockHash(cfg.Chain.CurrentBlockHash),
		dbft.WithGetValidators(srv.getValidators),
		dbft.WithGetConsensusAddress(srv.getConsensusAddress),

		dbft.WithNewConsensusPayload(srv.newPayload),
		dbft.WithNewPrepareRequest(srv.newPrepareRequest),
		dbft.WithNewPrepareResponse(srv.newPrepareResponse),
		dbft.WithNewChangeView(func() payload.ChangeView { return new(changeView) }),
		dbft.WithNewCommit(srv.newCommit),
		dbft.WithNewRecoveryRequest(func() payload.RecoveryRequest { return new(recoveryRequest) }),
		dbft.WithNewRecoveryMessage(srv.newRecoveryMessage),
		dbft.WithVerifyPrepareRequest(srv.verifyRequest),
		dbft.WithVerifyPrepareResponse(srv.verifyResponse),
	)

	if srv.dbft == nil {
		return nil, errors.New("can't initialize dBFT")
	}

	return srv, nil
}

var (
	_ block.Transaction = (*transaction.Transaction)(nil)
	_ block.Block       = (*neoBlock)(nil)
)

func (s *service) Start() {
	if s.started.CAS(false, true) {
		s.dbft.Start()
		s.Chain.SubscribeForBlocks(s.blockEvents)
		go s.eventLoop()
	}
}

func (s *service) eventLoop() {
	for {
		select {
		case <-s.dbft.Timer.C():
			hv := s.dbft.Timer.HV()
			s.log.Debug("timer fired",
				zap.Uint32("height", hv.Height),
				zap.Uint("view", uint(hv.View)))
			s.dbft.OnTimeout(hv)
		case msg := <-s.messages:
			fields := []zap.Field{
				zap.Uint16("from", msg.validatorIndex),
				zap.Stringer("type", msg.Type()),
			}

			if msg.Type() == payload.RecoveryMessageType {
				rec := msg.GetRecoveryMessage().(*recoveryMessage)
				if rec.preparationHash == nil {
					req := rec.GetPrepareRequest(&msg, s.dbft.Validators, uint16(s.dbft.PrimaryIndex))
					if req != nil {
						h := req.Hash()
						rec.preparationHash = &h
					}
				}

				fields = append(fields,
					zap.Int("#preparation", len(rec.preparationPayloads)),
					zap.Int("#commit", len(rec.commitPayloads)),
					zap.Int("#changeview", len(rec.changeViewPayloads)),
					zap.Bool("#request", rec.prepareRequest != nil),
					zap.Bool("#hash", rec.preparationHash != nil))
			}

			s.log.Debug("received message", fields...)
			s.dbft.OnReceive(&msg)
		case tx := <-s.transactions:
			s.dbft.OnTransaction(tx)
		case b := <-s.blockEvents:
			// We also receive our own blocks here, so check for index.
			if b.Index >= s.dbft.BlockIndex {
				s.log.Debug("new block in the chain",
					zap.Uint32("dbft index", s.dbft.BlockIndex),
					zap.Uint32("chain index", s.Chain.BlockHeight()))
				s.dbft.InitializeConsensus(0)
			}
		}
	}
}

func (s *service) newPayload() payload.ConsensusPayload {
	return &Payload{
		message: &message{
			stateRootEnabled: s.stateRootEnabled(),
		},
	}
}

// stateRootEnabled checks if state root feature is enabled on current height.
// It should be called only from dbft callbacks and is not protected by any mutex.
func (s *service) stateRootEnabled() bool {
	return s.Chain.GetConfig().EnableStateRoot
}

func (s *service) newPrepareRequest() payload.PrepareRequest {
	res := &prepareRequest{
		stateRootEnabled: s.stateRootEnabled(),
	}
	if !s.stateRootEnabled() {
		return res
	}
	sig := s.getStateRootSig()
	if sig != nil {
		copy(res.stateRootSig[:], sig)
	}
	return res
}

func (s *service) getStateRootSig() []byte {
	var sig []byte

	sr, err := s.Chain.GetStateRoot(s.dbft.BlockIndex - 1)
	if err == nil {
		data := sr.GetSignedPart()
		sig, _ = s.dbft.Priv.Sign(data)
	}
	return sig
}

func (s *service) newCommit() payload.Commit {
	if s.stateRootEnabled() && s.dbft.Context.BlockIndex > s.Chain.GetConfig().StateRootEnableIndex {
		// This is being called when we're ready to commit, so we can safely
		// relay stateroot here.
		stateRoot, err := s.Chain.GetStateRoot(s.dbft.Context.BlockIndex - 1)
		if err != nil {
			s.log.Warn("can't get stateroot", zap.Uint32("block", s.dbft.Context.BlockIndex-1))
		}
		r := stateRoot.MPTRoot
		r.Witness = s.getWitness(func(ctx dbft.Context, i int) []byte {
			if p := ctx.PreparationPayloads[i]; p != nil && p.ViewNumber() == ctx.ViewNumber {
				if int(ctx.PrimaryIndex) == i {
					return p.GetPrepareRequest().(*prepareRequest).stateRootSig[:]
				}
				return p.GetPrepareResponse().(*prepareResponse).stateRootSig[:]
			}
			return nil
		})
		if err := s.Chain.AddStateRoot(&r); err != nil {
			s.log.Warn("errors while adding state root", zap.Error(err))
		}
		s.Broadcast(&r)
	}
	return new(commit)
}

func (s *service) newPrepareResponse() payload.PrepareResponse {
	res := &prepareResponse{
		stateRootEnabled: s.stateRootEnabled(),
	}
	if !s.stateRootEnabled() {
		return res
	}
	sig := s.getStateRootSig()
	if sig != nil {
		copy(res.stateRootSig[:], sig)
	}
	return res
}

func (s *service) newRecoveryMessage() payload.RecoveryMessage {
	return &recoveryMessage{stateRootEnabled: s.stateRootEnabled()}
}

func (s *service) validatePayload(p *Payload) bool {
	validators := s.getValidators()
	if int(p.validatorIndex) >= len(validators) {
		return false
	}

	pub := validators[p.validatorIndex]
	h := pub.(*publicKey).GetScriptHash()

	return p.Verify(h)
}

func (s *service) getKeyPair(pubs []crypto.PublicKey) (int, crypto.PrivateKey, crypto.PublicKey) {
	for i := range pubs {
		sh := pubs[i].(*publicKey).GetScriptHash()
		acc := s.wallet.GetAccount(sh)
		if acc == nil {
			continue
		}

		key, err := keys.NEP2Decrypt(acc.EncryptedWIF, s.Config.Wallet.Password)
		if err != nil {
			s.log.Fatal("can't unlock account", zap.String("address", address.Uint160ToString(sh)))
			break
		}

		return i, &privateKey{PrivateKey: key}, &publicKey{PublicKey: key.PublicKey()}
	}

	return -1, nil, nil
}

// OnPayload handles Payload receive.
func (s *service) OnPayload(cp *Payload) {
	log := s.log.With(zap.Stringer("hash", cp.Hash()))
	if !s.validatePayload(cp) {
		log.Debug("can't validate payload")
		return
	} else if s.cache.Has(cp.Hash()) {
		log.Debug("payload is already in cache")
		return
	}

	s.Config.Broadcast(cp)
	s.cache.Add(cp)

	if s.dbft == nil || !s.started.Load() {
		log.Debug("dbft is inactive or not started yet")
		return
	}

	// decode payload data into message
	if cp.message == nil {
		if err := cp.decodeData(s.stateRootEnabled()); err != nil {
			log.Debug("can't decode payload data", zap.Error(err))
			return
		}
	}

	s.messages <- *cp
}

func (s *service) OnTransaction(tx *transaction.Transaction) {
	if s.dbft != nil {
		s.transactions <- tx
	}
}

// GetPayload returns payload stored in cache.
func (s *service) GetPayload(h util.Uint256) *Payload {
	p := s.cache.Get(h)
	if p == nil {
		return (*Payload)(nil)
	}

	cp := *p.(*Payload)

	return &cp
}

func (s *service) broadcast(p payload.ConsensusPayload) {
	switch p.Type() {
	case payload.PrepareRequestType:
		pr := p.GetPrepareRequest().(*prepareRequest)
		pr.minerTx = *s.txx.Get(pr.transactionHashes[0]).(*transaction.Transaction)
	}

	if err := p.(*Payload).Sign(s.dbft.Priv.(*privateKey)); err != nil {
		s.log.Warn("can't sign consensus payload", zap.Error(err))
	}

	s.cache.Add(p)
	s.Config.Broadcast(p.(*Payload))
}

func (s *service) getTx(h util.Uint256) block.Transaction {
	if tx := s.txx.Get(h); tx != nil {
		return tx.(*transaction.Transaction)
	}

	tx, _, _ := s.Config.Chain.GetTransaction(h)

	// this is needed because in case of absent tx dBFT expects to
	// get nil interface, not a nil pointer to any concrete type
	if tx != nil {
		return tx
	}

	return nil
}

func (s *service) verifyBlock(b block.Block) bool {
	coreb := &b.(*neoBlock).Block
	for _, tx := range coreb.Transactions {
		if err := s.Chain.VerifyTx(tx, coreb); err != nil {
			s.log.Warn("invalid transaction in proposed block", zap.Stringer("hash", tx.Hash()))
			return false
		}
	}

	return true
}

func (s *service) verifyStateRootSig(index int, sig []byte) error {
	r, err := s.Chain.GetStateRoot(s.dbft.BlockIndex - 1)
	if err != nil {
		return fmt.Errorf("can't get local state root: %v", err)
	}
	validators := s.getValidators()
	if index >= len(validators) {
		return errors.New("bad validator index")
	}

	pub := validators[index]
	if pub.Verify(r.GetSignedPart(), sig) != nil {
		return errors.New("bad state root signature")
	}
	return nil
}

func (s *service) verifyRequest(p payload.ConsensusPayload) error {
	req := p.GetPrepareRequest().(*prepareRequest)
	if s.stateRootEnabled() {
		err := s.verifyStateRootSig(int(p.ValidatorIndex()), req.stateRootSig[:])
		if err != nil {
			return err
		}
	}
	// Save lastProposal for getVerified().
	s.txx.Add(&req.minerTx)
	s.lastProposal = req.transactionHashes

	return nil
}

func (s *service) verifyResponse(p payload.ConsensusPayload) error {
	if !s.stateRootEnabled() {
		return nil
	}
	resp := p.GetPrepareResponse().(*prepareResponse)
	return s.verifyStateRootSig(int(p.ValidatorIndex()), resp.stateRootSig[:])
}

func (s *service) processBlock(b block.Block) {
	bb := &b.(*neoBlock).Block
	bb.Script = *(s.getBlockWitness(bb))

	if err := s.Chain.AddBlock(bb); err != nil {
		// The block might already be added via the regular network
		// interaction.
		if _, errget := s.Chain.GetBlock(bb.Hash()); errget != nil {
			s.log.Warn("error on add block", zap.Error(err))
		}
	}
}

func (s *service) getBlockWitness(_ *coreb.Block) *transaction.Witness {
	return s.getWitness(func(ctx dbft.Context, i int) []byte {
		if p := ctx.CommitPayloads[i]; p != nil && p.ViewNumber() == ctx.ViewNumber {
			return p.GetCommit().Signature()
		}
		return nil
	})
}

func (s *service) getWitness(f func(dbft.Context, int) []byte) *transaction.Witness {
	dctx := s.dbft.Context
	pubs := convertKeys(dctx.Validators)
	sigs := make(map[*keys.PublicKey][]byte)

	for i := range pubs {
		sig := f(dctx, i)
		if sig != nil {
			sigs[pubs[i]] = sig
		}
	}

	m := s.dbft.Context.M()
	verif, err := smartcontract.CreateMultiSigRedeemScript(m, pubs)
	if err != nil {
		s.log.Warn("can't create multisig redeem script", zap.Error(err))
		return nil
	}

	sort.Sort(keys.PublicKeys(pubs))

	var invoc []byte
	for i, j := 0, 0; i < len(pubs) && j < m; i++ {
		if sig, ok := sigs[pubs[i]]; ok {
			invoc = append(invoc, byte(opcode.PUSHBYTES64))
			invoc = append(invoc, sig...)
			j++
		}
	}

	return &transaction.Witness{
		InvocationScript:   invoc,
		VerificationScript: verif,
	}
}

func (s *service) getBlock(h util.Uint256) block.Block {
	b, err := s.Chain.GetBlock(h)
	if err != nil {
		return nil
	}

	return &neoBlock{Block: *b}
}

func (s *service) getVerifiedTx() []block.Transaction {
	pool := s.Config.Chain.GetMemPool()

	var txx []mempool.TxWithFee

	if s.dbft.ViewNumber > 0 {
		txx = make([]mempool.TxWithFee, 0, len(s.lastProposal))
		for i := range s.lastProposal {
			if tx, fee, ok := pool.TryGetValue(s.lastProposal[i]); ok {
				txx = append(txx, mempool.TxWithFee{Tx: tx, Fee: fee})
			}
		}

		if len(txx) < len(s.lastProposal)/2 {
			txx = pool.GetVerifiedTransactions()
		}
	} else {
		txx = pool.GetVerifiedTransactions()
	}

	if len(txx) > 0 {
		txx = s.Config.Chain.ApplyPolicyToTxSet(txx)
	}

	res := make([]block.Transaction, len(txx)+1)
	var netFee util.Fixed8
	for i := range txx {
		res[i+1] = txx[i].Tx
		netFee += txx[i].Fee
	}

	var txOuts []transaction.Output
	if netFee != 0 {
		sh := s.wallet.GetChangeAddress()
		if sh.Equals(util.Uint160{}) {
			pk := s.dbft.Pub.(*publicKey)
			sh = pk.GetScriptHash()
		}
		txOuts = []transaction.Output{{
			AssetID:    core.UtilityTokenID(),
			Amount:     netFee,
			ScriptHash: sh,
		}}
	}
	for {
		nonce := rand.Uint32()
		res[0] = &transaction.Transaction{
			Type:       transaction.MinerType,
			Version:    0,
			Data:       &transaction.MinerTX{Nonce: nonce},
			Attributes: nil,
			Inputs:     nil,
			Outputs:    txOuts,
			Scripts:    nil,
			Trimmed:    false,
		}

		if tx, _, _ := s.Chain.GetTransaction(res[0].Hash()); tx == nil {
			break
		}
	}

	s.txx.Add(res[0])

	return res
}

func (s *service) getValidators(txx ...block.Transaction) []crypto.PublicKey {
	var (
		pKeys []*keys.PublicKey
		err   error
	)
	if len(txx) == 0 {
		pKeys, err = s.Chain.GetValidators()
	} else {
		ntxx := make([]*transaction.Transaction, len(txx))
		for i := range ntxx {
			ntxx[i] = txx[i].(*transaction.Transaction)
		}

		pKeys, err = s.Chain.GetValidators(ntxx...)
	}

	if err != nil {
		s.log.Error("error while trying to get validators", zap.Error(err))
	}

	pubs := make([]crypto.PublicKey, len(pKeys))
	for i := range pKeys {
		pubs[i] = &publicKey{PublicKey: pKeys[i]}
	}

	return pubs
}

func (s *service) getConsensusAddress(validators ...crypto.PublicKey) (h util.Uint160) {
	pubs := convertKeys(validators)

	script, err := smartcontract.CreateMultiSigRedeemScript(s.dbft.M(), pubs)
	if err != nil {
		return
	}

	return crypto.Hash160(script)
}

func convertKeys(validators []crypto.PublicKey) (pubs []*keys.PublicKey) {
	pubs = make([]*keys.PublicKey, len(validators))
	for i, k := range validators {
		pubs[i] = k.(*publicKey).PublicKey
	}

	return
}

func (s *service) newBlockFromContext(ctx *dbft.Context) block.Block {
	block := new(neoBlock)
	if len(ctx.TransactionHashes) == 0 {
		return nil
	}

	block.Block.Timestamp = uint32(ctx.Timestamp / 1000000000)
	block.Block.Index = ctx.BlockIndex
	block.Block.NextConsensus = ctx.NextConsensus
	block.Block.PrevHash = ctx.PrevHash
	block.Block.Version = ctx.Version
	block.Block.ConsensusData = ctx.Nonce

	mt := merkle.NewMerkleTree(ctx.TransactionHashes...)
	block.Block.MerkleRoot = mt.Root().Hash
	return block
}
