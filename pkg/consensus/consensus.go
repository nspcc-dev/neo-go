package consensus

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync/atomic"
	"time"

	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/dbft/timer"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	coreb "github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	npayload "github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/zap"
)

// cacheMaxCapacity is the default cache capacity taken
// from C# implementation https://github.com/neo-project/neo/blob/master/neo/Ledger/Blockchain.cs#L64
const cacheMaxCapacity = 100

// defaultTimePerBlock is a period between blocks which is used in Neo.
const defaultTimePerBlock = 15 * time.Second

// Number of nanoseconds in millisecond.
const nsInMs = 1000000

// Ledger is the interface to Blockchain sufficient for Service.
type Ledger interface {
	ApplyPolicyToTxSet([]*transaction.Transaction) []*transaction.Transaction
	GetConfig() config.Blockchain
	GetMemPool() *mempool.Pool
	GetNextBlockValidators() ([]*keys.PublicKey, error)
	GetStateRoot(height uint32) (*state.MPTRoot, error)
	GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
	ComputeNextBlockValidators() []*keys.PublicKey
	PoolTx(t *transaction.Transaction, pools ...*mempool.Pool) error
	SubscribeForBlocks(ch chan *coreb.Block)
	UnsubscribeFromBlocks(ch chan *coreb.Block)
	GetBaseExecFee() int64
	CalculateAttributesFee(tx *transaction.Transaction) int64
	interop.Ledger
	mempool.Feer
}

// BlockQueuer is an interface to the block queue manager sufficient for Service.
type BlockQueuer interface {
	PutBlock(block *coreb.Block) error
}

// Service represents a consensus instance.
type Service interface {
	// Name returns service name.
	Name() string
	// Start initializes dBFT and starts event loop for consensus service.
	// It must be called only when the sufficient amount of peers are connected.
	// The service only starts once, subsequent calls to Start are no-op.
	Start()
	// Shutdown stops dBFT event loop. It can only be called once, subsequent calls
	// to Shutdown on the same instance are no-op. The instance that was stopped can
	// not be started again by calling Start (use a new instance if needed).
	Shutdown()

	// OnPayload is a callback to notify the Service about a newly received payload.
	OnPayload(p *npayload.Extensible) error
	// OnTransaction is a callback to notify the Service about a newly received transaction.
	OnTransaction(tx *transaction.Transaction)
}

type service struct {
	Config

	log *zap.Logger
	// txx is a fifo cache which stores miner transactions.
	txx  *relayCache
	dbft *dbft.DBFT[util.Uint256]
	// messages and transactions are channels needed to process
	// everything in single thread.
	messages     chan Payload
	transactions chan *transaction.Transaction
	// blockEvents is used to pass a new block event to the consensus
	// process. It has a tiny buffer in order to avoid Blockchain blocking
	// on block addition under the high load.
	blockEvents  chan *coreb.Block
	lastProposal []util.Uint256
	wallet       *wallet.Wallet
	// started is a flag set with Start method that runs an event handling
	// goroutine.
	started  atomic.Bool
	quit     chan struct{}
	finished chan struct{}
	// lastTimestamp contains timestamp for the last processed block.
	// We can't rely on timestamp from dbft context because it is changed
	// before the block is accepted. So, in case of change view, it will contain
	// an updated value.
	lastTimestamp uint64
}

// Config is a configuration for consensus services.
type Config struct {
	// Logger is a logger instance.
	Logger *zap.Logger
	// Broadcast is a callback which is called to notify the server
	// about a new consensus payload to be sent.
	Broadcast func(p *npayload.Extensible)
	// Chain is a Ledger instance.
	Chain Ledger
	// BlockQueue is a BlockQueuer instance.
	BlockQueue BlockQueuer
	// ProtocolConfiguration contains protocol settings.
	ProtocolConfiguration config.ProtocolConfiguration
	// RequestTx is a callback to which will be called
	// when a node lacks transactions present in the block.
	RequestTx func(h ...util.Uint256)
	// StopTxFlow is a callback that is called after the consensus
	// process stops accepting incoming transactions.
	StopTxFlow func()
	// TimePerBlock is minimal time that should pass before the next block is accepted.
	TimePerBlock time.Duration
	// Wallet is a local-node wallet configuration. If the path is empty, then
	// no wallet will be initialized and the service will be in watch-only mode.
	Wallet config.Wallet
}

// NewService returns a new consensus.Service instance.
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
		txx:      newFIFOCache(cacheMaxCapacity),
		messages: make(chan Payload, 100),

		transactions: make(chan *transaction.Transaction, 100),
		blockEvents:  make(chan *coreb.Block, 1),
		quit:         make(chan struct{}),
		finished:     make(chan struct{}),
	}

	var err error

	if len(cfg.Wallet.Path) > 0 {
		if srv.wallet, err = wallet.NewWalletFromFile(cfg.Wallet.Path); err != nil {
			return nil, err
		}

		// Check that the wallet password is correct for at least one account.
		var ok = slices.ContainsFunc(srv.wallet.Accounts, func(acc *wallet.Account) bool {
			return acc.Decrypt(srv.Config.Wallet.Password, srv.wallet.Scrypt) == nil
		})
		if !ok {
			return nil, errors.New("no account with provided password was found")
		}
	}

	srv.dbft, err = dbft.New[util.Uint256](
		dbft.WithTimer[util.Uint256](timer.New()),
		dbft.WithLogger[util.Uint256](srv.log),
		dbft.WithSecondsPerBlock[util.Uint256](cfg.TimePerBlock),
		dbft.WithGetKeyPair[util.Uint256](srv.getKeyPair),
		dbft.WithRequestTx(cfg.RequestTx),
		dbft.WithStopTxFlow[util.Uint256](cfg.StopTxFlow),
		dbft.WithGetTx[util.Uint256](srv.getTx),
		dbft.WithGetVerified[util.Uint256](srv.getVerifiedTx),
		dbft.WithBroadcast[util.Uint256](srv.broadcast),
		dbft.WithProcessBlock[util.Uint256](srv.processBlock),
		dbft.WithVerifyBlock[util.Uint256](srv.verifyBlock),
		dbft.WithGetBlock[util.Uint256](srv.getBlock),
		dbft.WithWatchOnly[util.Uint256](func() bool { return false }),
		dbft.WithNewBlockFromContext[util.Uint256](srv.newBlockFromContext),
		dbft.WithCurrentHeight[util.Uint256](cfg.Chain.BlockHeight),
		dbft.WithCurrentBlockHash(cfg.Chain.CurrentBlockHash),
		dbft.WithGetValidators[util.Uint256](srv.getValidators),

		dbft.WithNewConsensusPayload[util.Uint256](srv.newPayload),
		dbft.WithNewPrepareRequest[util.Uint256](srv.newPrepareRequest),
		dbft.WithNewPrepareResponse[util.Uint256](srv.newPrepareResponse),
		dbft.WithNewChangeView[util.Uint256](srv.newChangeView),
		dbft.WithNewCommit[util.Uint256](srv.newCommit),
		dbft.WithNewRecoveryRequest[util.Uint256](srv.newRecoveryRequest),
		dbft.WithNewRecoveryMessage[util.Uint256](srv.newRecoveryMessage),
		dbft.WithVerifyPrepareRequest[util.Uint256](srv.verifyRequest),
		dbft.WithVerifyPrepareResponse[util.Uint256](srv.verifyResponse),
	)

	if err != nil {
		return nil, fmt.Errorf("can't initialize dBFT: %w", err)
	}

	return srv, nil
}

var (
	_ dbft.Transaction[util.Uint256] = (*transaction.Transaction)(nil)
	_ dbft.Block[util.Uint256]       = (*neoBlock)(nil)
)

// NewPayload creates a new consensus payload for the provided network.
func NewPayload(m netmode.Magic, stateRootEnabled bool) *Payload {
	return &Payload{
		Extensible: npayload.Extensible{
			Category: npayload.ConsensusCategory,
		},
		message: message{
			stateRootEnabled: stateRootEnabled,
		},
		network: m,
	}
}

func (s *service) newPayload(c *dbft.Context[util.Uint256], t dbft.MessageType, msg any) dbft.ConsensusPayload[util.Uint256] {
	cp := NewPayload(s.ProtocolConfiguration.Magic, s.ProtocolConfiguration.StateRootInHeader)
	cp.BlockIndex = c.BlockIndex
	cp.message.ValidatorIndex = byte(c.MyIndex)
	cp.message.ViewNumber = c.ViewNumber
	cp.message.Type = messageType(t)
	if pr, ok := msg.(*prepareRequest); ok {
		pr.prevHash = s.dbft.PrevHash
		pr.version = coreb.VersionInitial
	}
	cp.payload = msg.(io.Serializable)

	cp.Extensible.ValidBlockStart = 0
	cp.Extensible.ValidBlockEnd = c.BlockIndex
	cp.Extensible.Sender = c.Validators[c.MyIndex].(*keys.PublicKey).GetScriptHash()

	return cp
}

func (s *service) newPrepareRequest(ts uint64, nonce uint64, transactionsHashes []util.Uint256) dbft.PrepareRequest[util.Uint256] {
	r := &prepareRequest{
		timestamp:         ts / nsInMs,
		nonce:             nonce,
		transactionHashes: transactionsHashes,
	}
	if s.ProtocolConfiguration.StateRootInHeader {
		r.stateRootEnabled = true
		if sr, err := s.Chain.GetStateRoot(s.dbft.BlockIndex - 1); err == nil {
			r.stateRoot = sr.Root
		} else {
			panic(err)
		}
	}
	return r
}

func (s *service) newPrepareResponse(preparationHash util.Uint256) dbft.PrepareResponse[util.Uint256] {
	return &prepareResponse{
		preparationHash: preparationHash,
	}
}

func (s *service) newChangeView(newViewNumber byte, reason dbft.ChangeViewReason, ts uint64) dbft.ChangeView {
	return &changeView{
		newViewNumber: newViewNumber,
		timestamp:     ts / nsInMs,
		reason:        reason,
	}
}

func (s *service) newCommit(signature []byte) dbft.Commit {
	c := new(commit)
	copy(c.signature[:], signature)
	return c
}

func (s *service) newRecoveryRequest(ts uint64) dbft.RecoveryRequest {
	return &recoveryRequest{
		timestamp: ts / nsInMs,
	}
}

func (s *service) newRecoveryMessage() dbft.RecoveryMessage[util.Uint256] {
	return &recoveryMessage{
		stateRootEnabled: s.ProtocolConfiguration.StateRootInHeader,
	}
}

// Name returns service name.
func (s *service) Name() string {
	return "consensus"
}

func (s *service) Start() {
	if s.started.CompareAndSwap(false, true) {
		s.log.Info("starting consensus service")
		b, _ := s.Chain.GetBlock(s.Chain.CurrentBlockHash()) // Can't fail, we have some current block!
		s.lastTimestamp = b.Timestamp
		s.dbft.Start(s.lastTimestamp * nsInMs)
		go s.eventLoop()
	}
}

// Shutdown implements the Service interface.
func (s *service) Shutdown() {
	if s.started.CompareAndSwap(true, false) {
		s.log.Info("stopping consensus service")
		close(s.quit)
		<-s.finished
		if s.wallet != nil {
			s.wallet.Close()
		}
	}
	_ = s.log.Sync()
}

func (s *service) eventLoop() {
	s.Chain.SubscribeForBlocks(s.blockEvents)

	// Manually sync up with potentially missed fresh blocks that may be added by blockchain
	// before the subscription.
	b, _ := s.Chain.GetBlock(s.Chain.CurrentBlockHash()) // Can't fail, we have some current block!
	if b.Timestamp >= s.lastTimestamp {
		s.handleChainBlock(b)
	}
events:
	for {
		select {
		case <-s.quit:
			s.dbft.Timer.Stop()
			s.Chain.UnsubscribeFromBlocks(s.blockEvents)
			break events
		case <-s.dbft.Timer.C():
			h, v := s.dbft.Timer.Height(), s.dbft.Timer.View()
			s.log.Debug("timer fired",
				zap.Uint32("height", h),
				zap.Uint("view", uint(v)))
			s.dbft.OnTimeout(h, v)
		case msg := <-s.messages:
			fields := []zap.Field{
				zap.Uint8("from", msg.message.ValidatorIndex),
				zap.Stringer("type", msg.Type()),
			}

			if msg.Type() == dbft.RecoveryMessageType {
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
			s.handleChainBlock(b)
		}
		// Always process block event if there is any, we can add one above or external
		// services can add several blocks during message processing.
		var latestBlock *coreb.Block
	syncLoop:
		for {
			select {
			case latestBlock = <-s.blockEvents:
			default:
				break syncLoop
			}
		}
		if latestBlock != nil {
			s.handleChainBlock(latestBlock)
		}
	}
drainLoop:
	for {
		select {
		case <-s.messages:
		case <-s.transactions:
		case <-s.blockEvents:
		default:
			break drainLoop
		}
	}
	close(s.messages)
	close(s.transactions)
	close(s.blockEvents)
	close(s.finished)
}

func (s *service) handleChainBlock(b *coreb.Block) {
	// We can get our own block here, so check for index.
	if b.Index >= s.dbft.BlockIndex {
		s.log.Debug("new block in the chain",
			zap.Uint32("dbft index", s.dbft.BlockIndex),
			zap.Uint32("chain index", s.Chain.BlockHeight()))
		s.postBlock(b)
		s.dbft.Reset(b.Timestamp * nsInMs)
	}
}

func (s *service) validatePayload(p *Payload) bool {
	validators := s.getValidators()
	if int(p.message.ValidatorIndex) >= len(validators) {
		return false
	}

	pub := validators[p.message.ValidatorIndex]
	h := pub.(*keys.PublicKey).GetScriptHash()
	return p.Sender == h
}

func (s *service) getKeyPair(pubs []dbft.PublicKey) (int, dbft.PrivateKey, dbft.PublicKey) {
	if s.wallet != nil {
		for i := range pubs {
			sh := pubs[i].(*keys.PublicKey).GetScriptHash()
			acc := s.wallet.GetAccount(sh)
			if acc == nil {
				continue
			}

			if !acc.CanSign() {
				err := acc.Decrypt(s.Config.Wallet.Password, s.wallet.Scrypt)
				if err != nil {
					s.log.Fatal("can't unlock account", zap.String("address", address.Uint160ToString(sh)))
					break
				}
			}

			return i, &privateKey{PrivateKey: acc.PrivateKey()}, acc.PublicKey()
		}
	}
	return -1, nil, nil
}

func (s *service) payloadFromExtensible(ep *npayload.Extensible) *Payload {
	return &Payload{
		Extensible: *ep,
		message: message{
			stateRootEnabled: s.ProtocolConfiguration.StateRootInHeader,
		},
	}
}

// OnPayload handles Payload receive.
func (s *service) OnPayload(cp *npayload.Extensible) error {
	log := s.log.With(zap.Stringer("hash", cp.Hash()))
	p := s.payloadFromExtensible(cp)
	// decode payload data into message
	if err := p.decodeData(); err != nil {
		log.Info("can't decode payload data", zap.Error(err))
		return nil
	}

	if !s.validatePayload(p) {
		log.Info("can't validate payload")
		return nil
	}

	if s.dbft == nil || !s.started.Load() {
		log.Debug("dbft is inactive or not started yet")
		return nil
	}

	s.messages <- *p
	return nil
}

func (s *service) OnTransaction(tx *transaction.Transaction) {
	if s.dbft != nil && s.started.Load() {
		s.transactions <- tx
	}
}

func (s *service) broadcast(p dbft.ConsensusPayload[util.Uint256]) {
	if err := p.(*Payload).Sign(s.dbft.Priv.(*privateKey)); err != nil {
		s.log.Warn("can't sign consensus payload", zap.Error(err))
	}

	ep := &p.(*Payload).Extensible
	s.Config.Broadcast(ep)
}

func (s *service) getTx(h util.Uint256) dbft.Transaction[util.Uint256] {
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

func (s *service) verifyBlock(b dbft.Block[util.Uint256]) bool {
	coreb := &b.(*neoBlock).Block

	if s.Chain.BlockHeight() >= coreb.Index {
		s.log.Warn("proposed block has already outdated")
		return false
	}
	if s.lastTimestamp >= coreb.Timestamp {
		s.log.Warn("proposed block has small timestamp",
			zap.Uint64("ts", coreb.Timestamp),
			zap.Uint64("last", s.lastTimestamp))
		return false
	}

	size := coreb.GetExpectedBlockSize()
	if size > int(s.ProtocolConfiguration.MaxBlockSize) {
		s.log.Warn("proposed block size exceeds config MaxBlockSize",
			zap.Uint32("max size allowed", s.ProtocolConfiguration.MaxBlockSize),
			zap.Int("block size", size))
		return false
	}

	var fee int64
	var pool = mempool.New(len(coreb.Transactions), 0, false, nil)
	var mainPool = s.Chain.GetMemPool()
	for _, tx := range coreb.Transactions {
		var err error

		fee += tx.SystemFee
		if mainPool.ContainsKey(tx.Hash()) {
			err = pool.Add(tx, s.Chain)
			if err == nil {
				continue
			}
		} else {
			err = s.Chain.PoolTx(tx, pool)
		}
		if err != nil {
			s.log.Warn("invalid transaction in proposed block",
				zap.Stringer("hash", tx.Hash()),
				zap.Error(err))
			return false
		}
		if s.Chain.BlockHeight() >= coreb.Index {
			s.log.Warn("proposed block has already outdated")
			return false
		}
	}

	maxBlockSysFee := s.ProtocolConfiguration.MaxBlockSystemFee
	if fee > maxBlockSysFee {
		s.log.Warn("proposed block system fee exceeds config MaxBlockSystemFee",
			zap.Int("max system fee allowed", int(maxBlockSysFee)),
			zap.Int("block system fee", int(fee)))
		return false
	}

	return true
}

var (
	errInvalidPrevHash          = errors.New("invalid PrevHash")
	errInvalidVersion           = errors.New("invalid Version")
	errInvalidStateRoot         = errors.New("state root mismatch")
	errInvalidTransactionsCount = errors.New("invalid transactions count")
)

func (s *service) verifyRequest(p dbft.ConsensusPayload[util.Uint256]) error {
	req := p.GetPrepareRequest().(*prepareRequest)
	if req.prevHash != s.dbft.PrevHash {
		return errInvalidPrevHash
	}
	if req.version != coreb.VersionInitial {
		return errInvalidVersion
	}
	if s.ProtocolConfiguration.StateRootInHeader {
		sr, err := s.Chain.GetStateRoot(s.dbft.BlockIndex - 1)
		if err != nil {
			return err
		} else if sr.Root != req.stateRoot {
			return fmt.Errorf("%w: %s != %s", errInvalidStateRoot, sr.Root, req.stateRoot)
		}
	}
	if len(req.TransactionHashes()) > int(s.ProtocolConfiguration.MaxTransactionsPerBlock) {
		return fmt.Errorf("%w: max = %d, got %d", errInvalidTransactionsCount, s.ProtocolConfiguration.MaxTransactionsPerBlock, len(req.TransactionHashes()))
	}
	// Save lastProposal for getVerified().
	s.lastProposal = req.transactionHashes

	return nil
}

func (s *service) verifyResponse(p dbft.ConsensusPayload[util.Uint256]) error {
	return nil
}

func (s *service) processBlock(b dbft.Block[util.Uint256]) {
	bb := &b.(*neoBlock).Block
	bb.Script = *(s.getBlockWitness(bb))

	if err := s.BlockQueue.PutBlock(bb); err != nil {
		// The block might already be added via the regular network
		// interaction.
		if _, errget := s.Chain.GetBlock(bb.Hash()); errget != nil {
			s.log.Warn("error on enqueue block", zap.Error(err))
		}
	}
	s.postBlock(bb)
}

func (s *service) postBlock(b *coreb.Block) {
	s.lastTimestamp = max(s.lastTimestamp, b.Timestamp)
	s.lastProposal = nil
}

func (s *service) getBlockWitness(b *coreb.Block) *transaction.Witness {
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
		s.log.Warn("can't create multisig redeem script", zap.Error(err))
		return nil
	}

	sort.Sort(keys.PublicKeys(pubs))

	buf := io.NewBufBinWriter()
	for i, j := 0, 0; i < len(pubs) && j < m; i++ {
		if sig, ok := sigs[pubs[i]]; ok {
			emit.Bytes(buf.BinWriter, sig)
			j++
		}
	}

	return &transaction.Witness{
		InvocationScript:   buf.Bytes(),
		VerificationScript: verif,
	}
}

func (s *service) getBlock(h util.Uint256) dbft.Block[util.Uint256] {
	b, err := s.Chain.GetBlock(h)
	if err != nil {
		return nil
	}

	return &neoBlock{network: s.ProtocolConfiguration.Magic, Block: *b}
}

func (s *service) getVerifiedTx() []dbft.Transaction[util.Uint256] {
	pool := s.Config.Chain.GetMemPool()

	var txx []*transaction.Transaction

	if s.dbft.ViewNumber > 0 && len(s.lastProposal) > 0 {
		txx = make([]*transaction.Transaction, 0, len(s.lastProposal))
		for i := range s.lastProposal {
			if tx, ok := pool.TryGetValue(s.lastProposal[i]); ok {
				txx = append(txx, tx)
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

	res := make([]dbft.Transaction[util.Uint256], len(txx))
	for i := range txx {
		res[i] = txx[i]
	}

	return res
}

func (s *service) getValidators(txes ...dbft.Transaction[util.Uint256]) []dbft.PublicKey {
	var (
		pKeys []*keys.PublicKey
		err   error
	)
	if txes == nil {
		// getValidators with empty args is used by dbft to fill the list of
		// block's validators, thus should return validators from the current
		// epoch without recalculation.
		pKeys, err = s.Chain.GetNextBlockValidators()
	}
	// getValidators with non-empty args is used by dbft to fill block's
	// NextConsensus field, but NeoGo doesn't provide WithGetConsensusAddress
	// callback and fills NextConsensus by itself via WithNewBlockFromContext
	// callback. Thus, leave pKeys empty if txes != nil.

	if err != nil {
		s.log.Error("error while trying to get validators", zap.Error(err))
	}

	pubs := make([]dbft.PublicKey, len(pKeys))
	for i := range pKeys {
		pubs[i] = pKeys[i]
	}

	return pubs
}

func convertKeys(validators []dbft.PublicKey) (pubs []*keys.PublicKey) {
	pubs = make([]*keys.PublicKey, len(validators))
	for i, k := range validators {
		pubs[i] = k.(*keys.PublicKey)
	}

	return
}

func (s *service) newBlockFromContext(ctx *dbft.Context[util.Uint256]) dbft.Block[util.Uint256] {
	block := &neoBlock{network: s.ProtocolConfiguration.Magic}

	block.Block.Timestamp = ctx.Timestamp / nsInMs
	block.Block.Nonce = ctx.Nonce
	block.Block.Index = ctx.BlockIndex
	if s.ProtocolConfiguration.StateRootInHeader {
		sr, err := s.Chain.GetStateRoot(ctx.BlockIndex - 1)
		if err != nil {
			s.log.Fatal(fmt.Sprintf("failed to get state root: %s", err.Error()))
		}
		block.StateRootEnabled = true
		block.PrevStateRoot = sr.Root
	}

	// ComputeNextBlockValidators returns proper set of validators wrt dBFT epochs
	// boundary. I.e. for the last block in the dBFT epoch this method returns the
	// list of validators recalculated from the latest relevant information about
	// NEO votes; in this case list of validators may differ from the one returned
	// by GetNextBlockValidators. For the not-last block of dBFT epoch this method
	// returns the same list as GetNextBlockValidators. Note, that by this moment
	// we must be sure that previous block was successfully persisted to chain
	// (i.e. PostPersist was completed for native Neo contract and PostPersist
	// execution cache was persisted to s.Chain's DAO), otherwise the wrong
	// (outdated, relevant for the previous dBFT epoch) value will be returned.
	var validators = s.Chain.ComputeNextBlockValidators()
	script, err := smartcontract.CreateDefaultMultiSigRedeemScript(validators)
	if err != nil {
		s.log.Fatal(fmt.Sprintf("failed to create multisignature script: %s", err.Error()))
	}
	block.Block.NextConsensus = hash.Hash160(script)
	block.Block.PrevHash = ctx.PrevHash
	block.Block.Version = coreb.VersionInitial

	primaryIndex := byte(ctx.PrimaryIndex)
	block.Block.PrimaryIndex = primaryIndex

	// it's OK to have ctx.TransactionsHashes == nil here
	block.Block.MerkleRoot = hash.CalcMerkleRoot(slices.Clone(ctx.TransactionHashes))

	return block
}
