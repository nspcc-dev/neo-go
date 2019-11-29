package consensus

import (
	"errors"
	"math/rand"
	"sort"
	"time"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/wallet"
	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/dbft/block"
	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/dbft/payload"
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

	log *zap.SugaredLogger
	// cache is a fifo cache which stores recent payloads.
	cache *relayCache
	// txx is a fifo cache which stores miner transactions.
	txx  *relayCache
	dbft *dbft.DBFT
	// messages and transactions are channels needed to process
	// everything in single thread.
	messages     chan Payload
	transactions chan *transaction.Transaction
}

// Config is a configuration for consensus services.
type Config struct {
	// Broadcast is a callback which is called to notify server
	// about new consensus payload to sent.
	Broadcast func(p *Payload)
	// Chain is a core.Blockchainer instance.
	Chain core.Blockchainer
	// RequestTx is a callback to which will be called
	// when a node lacks transactions present in a block.
	RequestTx func(h ...util.Uint256)
	// TimePerBlock minimal time that should pass before next block is accepted.
	TimePerBlock time.Duration
	// Wallet is a local-node wallet configuration.
	Wallet *config.WalletConfig
}

// NewService returns new consensus.Service instance.
func NewService(cfg Config) (Service, error) {
	log, err := getLogger()
	if err != nil {
		return nil, err
	}

	if cfg.TimePerBlock <= 0 {
		cfg.TimePerBlock = defaultTimePerBlock
	}

	srv := &service{
		Config: cfg,

		log:      log.Sugar(),
		cache:    newFIFOCache(cacheMaxCapacity),
		txx:      newFIFOCache(cacheMaxCapacity),
		messages: make(chan Payload, 100),

		transactions: make(chan *transaction.Transaction, 100),
	}

	if cfg.Wallet == nil {
		return srv, nil
	}

	priv, pub := getKeyPair(cfg.Wallet)

	srv.dbft = dbft.New(
		dbft.WithLogger(srv.log.Desugar()),
		dbft.WithSecondsPerBlock(cfg.TimePerBlock),
		dbft.WithKeyPair(priv, pub),
		dbft.WithTxPerBlock(10000),
		dbft.WithRequestTx(cfg.RequestTx),
		dbft.WithGetTx(srv.getTx),
		dbft.WithGetVerified(srv.getVerifiedTx),
		dbft.WithBroadcast(srv.broadcast),
		dbft.WithProcessBlock(srv.processBlock),
		dbft.WithVerifyBlock(srv.verifyBlock),
		dbft.WithGetBlock(srv.getBlock),
		dbft.WithWatchOnly(func() bool { return false }),
		dbft.WithNewBlock(func() block.Block { return new(neoBlock) }),
		dbft.WithCurrentHeight(cfg.Chain.BlockHeight),
		dbft.WithCurrentBlockHash(cfg.Chain.CurrentBlockHash),
		dbft.WithGetValidators(srv.getValidators),
		dbft.WithGetConsensusAddress(srv.getConsensusAddress),

		dbft.WithNewConsensusPayload(func() payload.ConsensusPayload { return new(Payload) }),
		dbft.WithNewPrepareRequest(func() payload.PrepareRequest { return new(prepareRequest) }),
		dbft.WithNewPrepareResponse(func() payload.PrepareResponse { return new(prepareResponse) }),
		dbft.WithNewChangeView(func() payload.ChangeView { return new(changeView) }),
		dbft.WithNewCommit(func() payload.Commit { return new(commit) }),
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
	s.dbft.Start()

	go s.eventLoop()
}

func (s *service) eventLoop() {
	for {
		select {
		case hv := <-s.dbft.Timer.C():
			s.log.Debugf("timer fired (%d,%d)", hv.Height, hv.View)
			s.dbft.OnTimeout(hv)
		case msg := <-s.messages:
			s.log.Debugf("received message from %d", msg.validatorIndex)
			s.dbft.OnReceive(&msg)
		case tx := <-s.transactions:
			s.dbft.OnTransaction(tx)
		}
	}
}

func getKeyPair(cfg *config.WalletConfig) (crypto.PrivateKey, crypto.PublicKey) {
	acc, err := wallet.DecryptAccount(cfg.Path, cfg.Password)
	if err != nil {
		return nil, nil
	}

	key := acc.PrivateKey()
	if key == nil {
		return nil, nil
	}

	return &privateKey{PrivateKey: key}, &publicKey{PublicKey: key.PublicKey()}
}

// OnPayload handles Payload receive.
func (s *service) OnPayload(cp *Payload) {
	if s.cache.Has(cp.Hash()) {
		return
	}

	s.Config.Broadcast(cp)
	s.cache.Add(cp)

	if s.dbft == nil {
		return
	}

	// we use switch here because other payloads could be possibly added in future
	switch cp.Type() {
	case payload.PrepareRequestType:
		s.txx.Add(&cp.GetPrepareRequest().(*prepareRequest).minerTx)
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

	s.cache.Add(p)
	s.Config.Broadcast(p.(*Payload))
}

func (s *service) getTx(h util.Uint256) block.Transaction {
	if tx := s.txx.Get(h); tx != nil {
		return tx.(*transaction.Transaction)
	}

	tx, _, _ := s.Config.Chain.GetTransaction(h)

	return tx
}

func (s *service) verifyBlock(b block.Block) bool {
	coreb := &b.(*neoBlock).Block
	for _, tx := range coreb.Transactions {
		if err := s.Chain.VerifyTx(tx, coreb); err != nil {
			return false
		}
	}

	return true
}

func (s *service) processBlock(b block.Block) {
	bb := &b.(*neoBlock).Block
	bb.Script = s.getBlockWitness(bb)

	if err := s.Chain.AddBlock(bb); err != nil {
		s.log.Warnf("error on add block: %v", err)
	}
}

func (s *service) getBlockWitness(b *core.Block) *transaction.Witness {
	dctx := s.dbft.Context
	pubs := convertKeys(dctx.Validators)
	sigs := make(map[*keys.PublicKey][]byte)

	for i := range pubs {
		if p := dctx.CommitPayloads[i]; p != nil && p.ViewNumber() == dctx.ViewNumber {
			sigs[pubs[i]] = p.GetCommit().Signature()
		}
	}

	m := s.dbft.Context.M()
	verif, err := smartcontract.CreateMultiSigRedeemScript(m, pubs)
	if err != nil {
		s.log.Warnf("can't create multisig redeem script: %v", err)
		return nil
	}

	sort.Sort(keys.PublicKeys(pubs))

	var invoc []byte
	for i, j := 0, 0; i < len(pubs) && j < m; i++ {
		if sig, ok := sigs[pubs[i]]; ok {
			invoc = append(invoc, byte(vm.PUSHBYTES64))
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

func (s *service) getVerifiedTx(count int) []block.Transaction {
	pool := s.Config.Chain.GetMemPool()
	txx := pool.GetVerifiedTransactions()

	res := make([]block.Transaction, len(txx)+1)
	for i := 1; i < len(res); i++ {
		res[i] = txx[i]
	}

	for {
		nonce := rand.Uint32()
		res[0] = &transaction.Transaction{
			Type:       transaction.MinerType,
			Version:    0,
			Data:       &transaction.MinerTX{Nonce: nonce},
			Attributes: nil,
			Inputs:     nil,
			Outputs:    nil,
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
	var pKeys []*keys.PublicKey
	if len(txx) == 0 {
		pKeys, _ = s.Chain.GetValidators()
	} else {
		ntxx := make([]*transaction.Transaction, len(txx))
		for i := range ntxx {
			ntxx[i] = txx[i].(*transaction.Transaction)
		}

		pKeys, _ = s.Chain.GetValidators(ntxx...)
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
