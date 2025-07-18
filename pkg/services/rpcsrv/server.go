package rpcsrv

import (
	"bytes"
	"context"
	"crypto/elliptic"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/rpcevent"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle/broadcaster"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv/params"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest/standard"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"go.uber.org/zap"
)

type (
	// Ledger abstracts away the Blockchain as used by the RPC server.
	Ledger interface {
		AddBlock(block *block.Block) error
		BlockHeight() uint32
		CalculateAttributesFee(tx *transaction.Transaction) int64
		CalculateClaimable(h util.Uint160, endHeight uint32) (*big.Int, error)
		CurrentBlockHash() util.Uint256
		FeePerByte() int64
		ForEachNEP11Transfer(acc util.Uint160, newestTimestamp uint64, f func(*state.NEP11Transfer) (bool, error)) error
		ForEachNEP17Transfer(acc util.Uint160, newestTimestamp uint64, f func(*state.NEP17Transfer) (bool, error)) error
		GetAppExecResults(util.Uint256, trigger.Type) ([]state.AppExecResult, error)
		GetBaseExecFee() int64
		GetBlock(hash util.Uint256) (*block.Block, error)
		GetCommittee() (keys.PublicKeys, error)
		GetConfig() config.Blockchain
		GetContractScriptHash(id int32) (util.Uint160, error)
		GetContractState(hash util.Uint160) *state.Contract
		GetEnrollments() ([]state.Validator, error)
		GetFakeNextBlock(nextBlockHeight uint32) (*block.Block, error)
		GetGoverningTokenBalance(acc util.Uint160) (*big.Int, uint32)
		GetHeader(hash util.Uint256) (*block.Header, error)
		GetHeaderHash(uint32) util.Uint256
		GetMaxTraceableBlocks() uint32
		GetMaxVerificationGAS() int64
		GetMemPool() *mempool.Pool
		GetNEP11Contracts() []util.Uint160
		GetNEP17Contracts() []util.Uint160
		GetNativeContractScriptHash(string) (util.Uint160, error)
		GetNatives() []state.Contract
		GetNextBlockValidators() ([]*keys.PublicKey, error)
		GetStateModule() core.StateRoot
		GetStorageItem(id int32, key []byte) state.StorageItem
		GetTestHistoricVM(t trigger.Type, tx *transaction.Transaction, nextBlockHeight uint32) (*interop.Context, error)
		GetTestVM(t trigger.Type, tx *transaction.Transaction, b *block.Block) (*interop.Context, error)
		GetTokenLastUpdated(acc util.Uint160) (map[int32]uint32, error)
		GetTransaction(util.Uint256) (*transaction.Transaction, uint32, error)
		HeaderHeight() uint32
		InitVerificationContext(ic *interop.Context, hash util.Uint160, witness *transaction.Witness) error
		GetMillisecondsPerBlock() uint32
		GetMaxValidUntilBlockIncrement() uint32
		P2PSigExtensionsEnabled() bool
		SubscribeForBlocks(ch chan *block.Block)
		SubscribeForHeadersOfAddedBlocks(ch chan *block.Header)
		SubscribeForExecutions(ch chan *state.AppExecResult)
		SubscribeForNotifications(ch chan *state.ContainedNotificationEvent)
		SubscribeForTransactions(ch chan *transaction.Transaction)
		UnsubscribeFromBlocks(ch chan *block.Block)
		UnsubscribeFromHeadersOfAddedBlocks(ch chan *block.Header)
		UnsubscribeFromExecutions(ch chan *state.AppExecResult)
		UnsubscribeFromNotifications(ch chan *state.ContainedNotificationEvent)
		UnsubscribeFromTransactions(ch chan *transaction.Transaction)
		VerifyTx(*transaction.Transaction) error
		VerifyWitness(util.Uint160, hash.Hashable, *transaction.Witness, int64) (int64, error)
		mempool.Feer // fee interface
		ContractStorageSeeker
		GetTrimmedBlock(hash util.Uint256) (*block.Block, error)
	}

	// ContractStorageSeeker is the interface `findstorage*` handlers need to be able to
	// seek over contract storage. Prefix is trimmed in the resulting pair's key.
	ContractStorageSeeker interface {
		SeekStorage(id int32, prefix []byte, cont func(k, v []byte) bool)
	}

	// OracleHandler is the interface oracle service needs to provide for the Server.
	OracleHandler interface {
		AddResponse(pub *keys.PublicKey, reqID uint64, txSig []byte)
	}

	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		http  []*http.Server
		https []*http.Server

		chain  Ledger
		config config.RPC
		// wsReadLimit represents web-socket message limit for a receiving side.
		wsReadLimit      int64
		upgrader         websocket.Upgrader
		network          netmode.Magic
		stateRootEnabled bool
		coreServer       *network.Server
		oracle           *atomic.Value
		log              *zap.Logger
		shutdown         chan struct{}
		started          atomic.Bool
		errChan          chan<- error

		sessionsLock sync.Mutex
		sessions     map[string]*session

		subsLock    sync.RWMutex
		subscribers map[*subscriber]bool

		subsCounterLock   sync.RWMutex
		blockSubs         int
		blockHeaderSubs   int
		executionSubs     int
		notificationSubs  int
		transactionSubs   int
		notaryRequestSubs int

		blockCh           chan *block.Block
		blockHeaderCh     chan *block.Header
		executionCh       chan *state.AppExecResult
		notificationCh    chan *state.ContainedNotificationEvent
		transactionCh     chan *transaction.Transaction
		notaryRequestCh   chan mempoolevent.Event
		subEventsToExitCh chan struct{}
	}

	// session holds a set of iterators got after invoke* call with corresponding
	// finalizer and session expiration timer.
	session struct {
		// iteratorsLock protects iteratorIdentifiers of the current session.
		iteratorsLock sync.Mutex
		// iteratorIdentifiers stores the set of Iterator stackitems got either from original invocation
		// or from historic MPT-based invocation. In the second case, iteratorIdentifiers are supposed
		// to be filled during the first `traverseiterator` call using corresponding params.
		iteratorIdentifiers []*iteratorIdentifier
		timer               *time.Timer
		finalize            func()
	}
	// iteratorIdentifier represents Iterator on the server side, holding iterator ID and Iterator stackitem.
	iteratorIdentifier struct {
		ID string
		// Item represents Iterator stackitem.
		Item stackitem.Item
		// Curr represents current Iterator value got after initial iterator traversal if in-place iterator
		// traversal is performed. This value must be used to prepend to the resulting items list during
		// the subsequent iterator traversal.
		Curr stackitem.Item
	}
)

const (
	// Disconnection timeout.
	wsPongLimit = 60 * time.Second

	// Ping period for connection liveness check.
	wsPingPeriod = wsPongLimit / 2

	// Write deadline.
	wsWriteLimit = wsPingPeriod / 2

	// Default maximum number of websocket clients per Server.
	defaultMaxWebSocketClients = 64

	// Maximum number of elements for get*transfers requests.
	maxTransfersLimit = 1000

	// defaultSessionPoolSize is the number of concurrently running iterator sessions.
	defaultSessionPoolSize = 20
)

var rpcHandlers = map[string]func(*Server, params.Params) (any, *neorpc.Error){
	"calculatenetworkfee":          (*Server).calculateNetworkFee,
	"findstates":                   (*Server).findStates,
	"findstorage":                  (*Server).findStorage,
	"findstoragehistoric":          (*Server).findStorageHistoric,
	"getapplicationlog":            (*Server).getApplicationLog,
	"getbestblockhash":             (*Server).getBestBlockHash,
	"getblock":                     (*Server).getBlock,
	"getblockcount":                (*Server).getBlockCount,
	"getblockhash":                 (*Server).getBlockHash,
	"getblockheader":               (*Server).getBlockHeader,
	"getblockheadercount":          (*Server).getBlockHeaderCount,
	"getblocknotifications":        (*Server).getBlockNotifications,
	"getblocksysfee":               (*Server).getBlockSysFee,
	"getcandidates":                (*Server).getCandidates,
	"getcommittee":                 (*Server).getCommittee,
	"getconnectioncount":           (*Server).getConnectionCount,
	"getcontractstate":             (*Server).getContractState,
	"getnativecontracts":           (*Server).getNativeContracts,
	"getnep11balances":             (*Server).getNEP11Balances,
	"getnep11properties":           (*Server).getNEP11Properties,
	"getnep11transfers":            (*Server).getNEP11Transfers,
	"getnep17balances":             (*Server).getNEP17Balances,
	"getnep17transfers":            (*Server).getNEP17Transfers,
	"getpeers":                     (*Server).getPeers,
	"getproof":                     (*Server).getProof,
	"getrawmempool":                (*Server).getRawMempool,
	"getrawnotarypool":             (*Server).getRawNotaryPool,
	"getrawnotarytransaction":      (*Server).getRawNotaryTransaction,
	"getrawtransaction":            (*Server).getrawtransaction,
	"getstate":                     (*Server).getState,
	"getstateheight":               (*Server).getStateHeight,
	"getstateroot":                 (*Server).getStateRoot,
	"getstorage":                   (*Server).getStorage,
	"getstoragehistoric":           (*Server).getStorageHistoric,
	"gettransactionheight":         (*Server).getTransactionHeight,
	"getunclaimedgas":              (*Server).getUnclaimedGas,
	"getnextblockvalidators":       (*Server).getNextBlockValidators,
	"getversion":                   (*Server).getVersion,
	"invokefunction":               (*Server).invokeFunction,
	"invokefunctionhistoric":       (*Server).invokeFunctionHistoric,
	"invokescript":                 (*Server).invokescript,
	"invokescripthistoric":         (*Server).invokescripthistoric,
	"invokecontainedscript":        (*Server).invokeContainedScript,
	"invokecontractverify":         (*Server).invokeContractVerify,
	"invokecontractverifyhistoric": (*Server).invokeContractVerifyHistoric,
	"sendrawtransaction":           (*Server).sendrawtransaction,
	"submitblock":                  (*Server).submitBlock,
	"submitnotaryrequest":          (*Server).submitNotaryRequest,
	"submitoracleresponse":         (*Server).submitOracleResponse,
	"terminatesession":             (*Server).terminateSession,
	"traverseiterator":             (*Server).traverseIterator,
	"validateaddress":              (*Server).validateAddress,
	"verifyproof":                  (*Server).verifyProof,
}

var rpcWsHandlers = map[string]func(*Server, params.Params, *subscriber) (any, *neorpc.Error){
	"subscribe":   (*Server).subscribe,
	"unsubscribe": (*Server).unsubscribe,
}

// New creates a new Server struct. Pay attention that orc is expected to be either
// untyped nil or non-nil structure implementing OracleHandler interface.
func New(chain Ledger, conf config.RPC, coreServer *network.Server,
	orc OracleHandler, log *zap.Logger, errChan chan<- error) *Server {
	protoCfg := chain.GetConfig().ProtocolConfiguration
	if conf.SessionEnabled {
		//nolint:staticcheck // SessionExpirationTime will not be used since version 0.113.0 release.
		if conf.SessionExpirationTime <= 0 {
			conf.SessionExpirationTime = int(max(protoCfg.TimePerBlock, 5*time.Second))
		}
		if conf.SessionLifetime <= 0 {
			//nolint:staticcheck // SessionExpirationTime will not be used since version 0.113.0 release.
			conf.SessionLifetime = time.Duration(conf.SessionExpirationTime)
			log.Info("SessionLifetime is not set or wrong, setting default value", zap.Duration("SessionLifetime", conf.SessionLifetime))
		}
		if conf.SessionPoolSize <= 0 {
			conf.SessionPoolSize = defaultSessionPoolSize
			log.Info("SessionPoolSize is not set or wrong, setting default value", zap.Int("SessionPoolSize", defaultSessionPoolSize))
		}
	}
	if conf.MaxIteratorResultItems <= 0 {
		conf.MaxIteratorResultItems = config.DefaultMaxIteratorResultItems
		log.Info("MaxIteratorResultItems is not set or wrong, setting default value", zap.Int("MaxIteratorResultItems", config.DefaultMaxIteratorResultItems))
	}
	if conf.MaxFindResultItems <= 0 {
		conf.MaxFindResultItems = config.DefaultMaxFindResultItems
		log.Info("MaxFindResultItems is not set or wrong, setting default value", zap.Int("MaxFindResultItems", config.DefaultMaxFindResultItems))
	}
	if conf.MaxFindStorageResultItems <= 0 {
		conf.MaxFindStorageResultItems = config.DefaultMaxFindStorageResultItems
		log.Info("MaxFindStorageResultItems is not set or wrong, setting default value", zap.Int("MaxFindStorageResultItems", config.DefaultMaxFindStorageResultItems))
	}
	if conf.MaxNEP11Tokens <= 0 {
		conf.MaxNEP11Tokens = config.DefaultMaxNEP11Tokens
		log.Info("MaxNEP11Tokens is not set or wrong, setting default value", zap.Int("MaxNEP11Tokens", config.DefaultMaxNEP11Tokens))
	}
	if conf.MaxRequestBodyBytes <= 0 {
		conf.MaxRequestBodyBytes = config.DefaultMaxRequestBodyBytes
		log.Info("MaxRequestBodyBytes is not set or wong, setting default value", zap.Int("MaxRequestBodyBytes", config.DefaultMaxRequestBodyBytes))
	}
	if conf.MaxRequestHeaderBytes <= 0 {
		conf.MaxRequestHeaderBytes = config.DefaultMaxRequestHeaderBytes
		log.Info("MaxRequestHeaderBytes is not set or wong, setting default value", zap.Int("MaxRequestHeaderBytes", config.DefaultMaxRequestHeaderBytes))
	}
	if conf.MaxWebSocketClients == 0 {
		conf.MaxWebSocketClients = defaultMaxWebSocketClients
		log.Info("MaxWebSocketClients is not set or wrong, setting default value", zap.Int("MaxWebSocketClients", defaultMaxWebSocketClients))
	}
	if conf.MaxWebSocketFeeds == 0 {
		conf.MaxWebSocketFeeds = defaultMaxFeeds
		log.Info("MaxWebSocketFeeds is not set or wrong, setting default value", zap.Int("MaxWebSocketFeeds", defaultMaxFeeds))
	}
	var oracleWrapped = new(atomic.Value)
	if orc != nil {
		oracleWrapped.Store(orc)
	}
	var wsOriginChecker func(*http.Request) bool
	if conf.EnableCORSWorkaround {
		wsOriginChecker = func(_ *http.Request) bool { return true }
	}

	addrs := conf.Addresses
	httpServers := make([]*http.Server, len(addrs))
	for i, addr := range addrs {
		httpServers[i] = &http.Server{
			Addr:           addr,
			MaxHeaderBytes: conf.MaxRequestHeaderBytes,
		}
	}

	var tlsServers []*http.Server
	if cfg := conf.TLSConfig; cfg.Enabled {
		addrs := cfg.Addresses
		tlsServers = make([]*http.Server, len(addrs))
		for i, addr := range addrs {
			tlsServers[i] = &http.Server{
				Addr:           addr,
				MaxHeaderBytes: conf.MaxRequestHeaderBytes,
			}
		}
	}

	return &Server{
		http:  httpServers,
		https: tlsServers,

		chain:            chain,
		config:           conf,
		wsReadLimit:      int64(protoCfg.MaxBlockSize*4)/3 + 1024, // Enough for Base64-encoded content of `submitblock` and `submitp2pnotaryrequest`.
		upgrader:         websocket.Upgrader{CheckOrigin: wsOriginChecker},
		network:          protoCfg.Magic,
		stateRootEnabled: protoCfg.StateRootInHeader,
		coreServer:       coreServer,
		log:              log,
		oracle:           oracleWrapped,
		shutdown:         make(chan struct{}),
		errChan:          errChan,

		sessions: make(map[string]*session),

		subscribers: make(map[*subscriber]bool),
		// These are NOT buffered to preserve original order of events.
		blockCh:           make(chan *block.Block),
		executionCh:       make(chan *state.AppExecResult),
		notificationCh:    make(chan *state.ContainedNotificationEvent),
		transactionCh:     make(chan *transaction.Transaction),
		notaryRequestCh:   make(chan mempoolevent.Event),
		blockHeaderCh:     make(chan *block.Header),
		subEventsToExitCh: make(chan struct{}),
	}
}

// Name returns service name.
func (s *Server) Name() string {
	return "rpc"
}

// Start creates a new JSON-RPC server listening on the configured port. It creates
// goroutines needed internally and it returns its errors via errChan passed to New().
// The Server only starts once, subsequent calls to Start are no-op.
func (s *Server) Start() {
	if !s.config.Enabled {
		s.log.Info("RPC server is not enabled")
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		s.log.Info("RPC server already started")
		return
	}

	go s.handleSubEvents()

	for _, srv := range s.http {
		srv.Handler = http.HandlerFunc(s.handleHTTPRequest)
		s.log.Info("starting rpc-server", zap.String("endpoint", srv.Addr))

		ln, err := net.Listen("tcp", srv.Addr)
		if err != nil {
			s.errChan <- fmt.Errorf("failed to listen on %s: %w", srv.Addr, err)
			return
		}
		srv.Addr = ln.Addr().String() // set Addr to the actual address
		go func(server *http.Server) {
			err = server.Serve(ln)
			if !errors.Is(err, http.ErrServerClosed) {
				s.log.Error("failed to start RPC server", zap.Error(err))
				s.errChan <- err
			}
		}(srv)
	}

	if cfg := s.config.TLSConfig; cfg.Enabled {
		for _, srv := range s.https {
			srv.Handler = http.HandlerFunc(s.handleHTTPRequest)
			s.log.Info("starting rpc-server (https)", zap.String("endpoint", srv.Addr))

			ln, err := net.Listen("tcp", srv.Addr)
			if err != nil {
				s.errChan <- err
				return
			}
			srv.Addr = ln.Addr().String()

			go func(srv *http.Server) {
				err = srv.ServeTLS(ln, cfg.CertFile, cfg.KeyFile)
				if !errors.Is(err, http.ErrServerClosed) {
					s.log.Error("failed to start TLS RPC server",
						zap.String("endpoint", srv.Addr), zap.Error(err))
					s.errChan <- err
				}
			}(srv)
		}
	}
}

// Shutdown stops the RPC server if it's running. It can only be called once,
// subsequent calls to Shutdown on the same instance are no-op. The instance
// that was stopped can not be started again by calling Start (use a new
// instance if needed).
func (s *Server) Shutdown() {
	if !s.started.CompareAndSwap(true, false) {
		return
	}
	// Signal to websocket writer routines and handleSubEvents.
	close(s.shutdown)

	if s.config.TLSConfig.Enabled {
		for _, srv := range s.https {
			s.log.Info("shutting down RPC server (https)", zap.String("endpoint", srv.Addr))
			err := srv.Shutdown(context.Background())
			if err != nil {
				s.log.Warn("error during RPC (https) server shutdown",
					zap.String("endpoint", srv.Addr), zap.Error(err))
			}
		}
	}

	for _, srv := range s.http {
		s.log.Info("shutting down RPC server", zap.String("endpoint", srv.Addr))
		err := srv.Shutdown(context.Background())
		if err != nil {
			s.log.Warn("error during RPC (http) server shutdown",
				zap.String("endpoint", srv.Addr), zap.Error(err))
		}
	}

	// Perform sessions finalisation.
	if s.config.SessionEnabled {
		s.sessionsLock.Lock()
		for _, session := range s.sessions {
			// Concurrent iterator traversal may still be in process, thus need to protect iteratorIdentifiers access.
			session.iteratorsLock.Lock()
			session.timer.Stop() // cancel session's finalizer.
			session.finalize()
			session.iteratorsLock.Unlock()
		}
		s.sessions = nil
		s.sessionsLock.Unlock()
	}

	// Wait for handleSubEvents to finish.
	<-s.subEventsToExitCh
	_ = s.log.Sync()
}

// SetOracleHandler allows to update oracle handler used by the Server.
func (s *Server) SetOracleHandler(orc OracleHandler) {
	s.oracle.Store(orc)
}

func (s *Server) handleHTTPRequest(w http.ResponseWriter, httpRequest *http.Request) {
	// Restrict request body before further processing.
	httpRequest.Body = http.MaxBytesReader(w, httpRequest.Body, int64(s.config.MaxRequestBodyBytes))
	req := params.NewRequest()

	if httpRequest.URL.Path == "/ws" && httpRequest.Method == "GET" {
		// Technically there is a race between this check and
		// s.subscribers modification 20 lines below, but it's tiny
		// and not really critical to bother with it. Some additional
		// clients may sneak in, no big deal.
		s.subsLock.RLock()
		numOfSubs := len(s.subscribers)
		s.subsLock.RUnlock()
		if numOfSubs >= s.config.MaxWebSocketClients {
			s.writeHTTPErrorResponse(
				params.NewIn(),
				w,
				neorpc.NewInternalServerError("websocket users limit reached"),
			)
			return
		}
		ws, err := s.upgrader.Upgrade(w, httpRequest, nil)
		if err != nil {
			s.log.Info("websocket connection upgrade failed", zap.Error(err))
			return
		}
		resChan := make(chan abstractResult) // response.abstract or response.abstractBatch
		subChan := make(chan intEvent, notificationBufSize)
		subscr := &subscriber{writer: subChan, feeds: make([]feed, s.config.MaxWebSocketFeeds)}
		s.subsLock.Lock()
		s.subscribers[subscr] = true
		s.subsLock.Unlock()
		go s.handleWsWrites(ws, resChan, subChan)
		s.handleWsReads(ws, resChan, subscr)
		return
	}

	if httpRequest.Method == "OPTIONS" && s.config.EnableCORSWorkaround { // Preflight CORS.
		setCORSOriginHeaders(w.Header())
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST") // GET for websockets.
		w.Header().Set("Access-Control-Max-Age", "21600")           // 6 hours.
		return
	}

	if httpRequest.Method != "POST" {
		s.writeHTTPErrorResponse(
			params.NewIn(),
			w,
			neorpc.NewInvalidParamsError(fmt.Sprintf("invalid method '%s', please retry with 'POST'", httpRequest.Method)),
		)
		return
	}

	err := req.DecodeData(httpRequest.Body)
	if err != nil {
		s.writeHTTPErrorResponse(params.NewIn(), w, neorpc.NewParseError(err.Error()))
		return
	}

	resp := s.handleRequest(req, nil)
	s.writeHTTPServerResponse(req, w, resp)
}

// RegisterLocal performs local client registration. Events channel will be
// closed by server once context is done or server is closed.
func (s *Server) RegisterLocal(ctx context.Context, events chan<- neorpc.Notification) func(*neorpc.Request) (*neorpc.Response, error) {
	subChan := make(chan intEvent, notificationBufSize)
	subscr := &subscriber{writer: subChan, feeds: make([]feed, s.config.MaxWebSocketFeeds)}
	s.subsLock.Lock()
	s.subscribers[subscr] = true
	s.subsLock.Unlock()
	go s.handleLocalNotifications(ctx, events, subChan, subscr)
	return func(req *neorpc.Request) (*neorpc.Response, error) {
		return s.handleInternal(req, subscr)
	}
}

func (s *Server) handleRequest(req *params.Request, sub *subscriber) abstractResult {
	if req.In != nil {
		req.In.Method = escapeForLog(req.In.Method) // No valid method name will be changed by it.
		return s.handleIn(req.In, sub)
	}
	resp := make(abstractBatch, len(req.Batch))
	for i, in := range req.Batch {
		in.Method = escapeForLog(in.Method) // No valid method name will be changed by it.
		resp[i] = s.handleIn(&in, sub)
	}
	return resp
}

// handleInternal is an experimental interface to handle client requests directly.
func (s *Server) handleInternal(req *neorpc.Request, sub *subscriber) (*neorpc.Response, error) {
	var (
		res    any
		rpcRes = &neorpc.Response{
			HeaderAndError: neorpc.HeaderAndError{
				Header: neorpc.Header{
					JSONRPC: req.JSONRPC,
					ID:      json.RawMessage(strconv.FormatUint(req.ID, 10)),
				},
			},
		}
	)
	reqParams, err := params.FromAny(req.Params)
	if err != nil {
		return nil, err
	}

	s.log.Debug("processing local rpc request",
		zap.String("method", req.Method),
		zap.Stringer("params", reqParams))

	start := time.Now()
	defer func() { addReqTimeMetric(req.Method, time.Since(start)) }()

	rpcRes.Error = neorpc.NewMethodNotFoundError(fmt.Sprintf("method %q not supported", req.Method))
	handler, ok := rpcHandlers[req.Method]
	if ok {
		res, rpcRes.Error = handler(s, reqParams)
	} else if sub != nil {
		handler, ok := rpcWsHandlers[req.Method]
		if ok {
			res, rpcRes.Error = handler(s, reqParams, sub)
		}
	}
	if res != nil {
		b, err := json.Marshal(res)
		if err != nil {
			return nil, fmt.Errorf("response can't be JSONized: %w", err)
		}
		rpcRes.Result = json.RawMessage(b)
	}
	return rpcRes, nil
}

func (s *Server) handleIn(req *params.In, sub *subscriber) abstract {
	var res any
	var resErr *neorpc.Error
	if req.JSONRPC != neorpc.JSONRPCVersion {
		return s.packResponse(req, nil, neorpc.NewInvalidParamsError(fmt.Sprintf("problem parsing JSON: invalid version, expected 2.0 got '%s'", req.JSONRPC)))
	}

	reqParams := params.Params(req.RawParams)

	s.log.Debug("processing rpc request",
		zap.String("method", req.Method),
		zap.Stringer("params", reqParams))

	start := time.Now()
	defer func() { addReqTimeMetric(req.Method, time.Since(start)) }()

	resErr = neorpc.NewMethodNotFoundError(fmt.Sprintf("method %q not supported", req.Method))
	handler, ok := rpcHandlers[req.Method]
	if ok {
		res, resErr = handler(s, reqParams)
	} else if sub != nil {
		handler, ok := rpcWsHandlers[req.Method]
		if ok {
			res, resErr = handler(s, reqParams, sub)
		}
	}
	return s.packResponse(req, res, resErr)
}

func (s *Server) handleLocalNotifications(ctx context.Context, events chan<- neorpc.Notification, subChan <-chan intEvent, subscr *subscriber) {
eventloop:
	for {
		select {
		case <-s.shutdown:
			break eventloop
		case <-ctx.Done():
			break eventloop
		case ev := <-subChan:
			events <- *ev.ntf // Make a copy.
		}
	}
	close(events)
	s.dropSubscriber(subscr)
drainloop:
	for {
		select {
		case <-subChan:
		default:
			break drainloop
		}
	}
}

func (s *Server) handleWsWrites(ws *websocket.Conn, resChan <-chan abstractResult, subChan <-chan intEvent) {
	pingTicker := time.NewTicker(wsPingPeriod)
eventloop:
	for {
		select {
		case <-s.shutdown:
			break eventloop
		case event, ok := <-subChan:
			if !ok {
				break eventloop
			}
			if err := ws.SetWriteDeadline(time.Now().Add(wsWriteLimit)); err != nil {
				break eventloop
			}
			if err := ws.WritePreparedMessage(event.msg); err != nil {
				break eventloop
			}
		case res, ok := <-resChan:
			if !ok {
				break eventloop
			}
			if err := ws.SetWriteDeadline(time.Now().Add(wsWriteLimit)); err != nil {
				break eventloop
			}
			if err := ws.WriteJSON(res); err != nil {
				break eventloop
			}
		case <-pingTicker.C:
			if err := ws.SetWriteDeadline(time.Now().Add(wsWriteLimit)); err != nil {
				break eventloop
			}
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				break eventloop
			}
		}
	}
	ws.Close()
	// Drain notification channel as there might be some goroutines blocked
	// on it.
drainloop:
	for {
		select {
		case _, ok := <-subChan:
			if !ok {
				break drainloop
			}
		default:
			break drainloop
		}
	}
}

func (s *Server) handleWsReads(ws *websocket.Conn, resChan chan<- abstractResult, subscr *subscriber) {
	ws.SetReadLimit(s.wsReadLimit)
	err := ws.SetReadDeadline(time.Now().Add(wsPongLimit))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(wsPongLimit)) })
requestloop:
	for err == nil {
		req := params.NewRequest()
		err := ws.ReadJSON(req)
		if err != nil {
			break
		}
		res := s.handleRequest(req, subscr)
		res.RunForErrors(func(jsonErr *neorpc.Error) {
			s.logRequestError(req, jsonErr)
		})
		select {
		case <-s.shutdown:
			break requestloop
		case resChan <- res:
		}
	}
	s.dropSubscriber(subscr)
	close(resChan)
	ws.Close()
}

func (s *Server) dropSubscriber(subscr *subscriber) {
	s.subsLock.Lock()
	delete(s.subscribers, subscr)
	s.subsLock.Unlock()
	s.subsCounterLock.Lock()
	for _, e := range subscr.feeds {
		if e.event != neorpc.InvalidEventID {
			s.unsubscribeFromChannel(e.event)
		}
	}
	s.subsCounterLock.Unlock()
}

func (s *Server) getBestBlockHash(_ params.Params) (any, *neorpc.Error) {
	return "0x" + s.chain.CurrentBlockHash().StringLE(), nil
}

func (s *Server) getBlockCount(_ params.Params) (any, *neorpc.Error) {
	return s.chain.BlockHeight() + 1, nil
}

func (s *Server) getBlockHeaderCount(_ params.Params) (any, *neorpc.Error) {
	return s.chain.HeaderHeight() + 1, nil
}

func (s *Server) getConnectionCount(_ params.Params) (any, *neorpc.Error) {
	return s.coreServer.PeerCount(), nil
}

func (s *Server) blockHashFromParam(param *params.Param) (util.Uint256, *neorpc.Error) {
	var (
		hash util.Uint256
		err  error
	)
	if param == nil {
		return hash, neorpc.ErrInvalidParams
	}

	if hash, err = param.GetUint256(); err != nil {
		num, respErr := s.blockHeightFromParam(param)
		if respErr != nil {
			return hash, respErr
		}
		hash = s.chain.GetHeaderHash(num)
	}
	return hash, nil
}

func (s *Server) fillBlockMetadata(obj io.Serializable, h *block.Header) result.BlockMetadata {
	res := result.BlockMetadata{
		Size:          io.GetVarSize(obj), // obj can be a Block or a Header.
		Confirmations: s.chain.BlockHeight() - h.Index + 1,
	}

	hash := s.chain.GetHeaderHash(h.Index + 1)
	if !hash.Equals(util.Uint256{}) {
		res.NextBlockHash = &hash
	}
	return res
}

func (s *Server) getBlock(reqParams params.Params) (any, *neorpc.Error) {
	param := reqParams.Value(0)
	hash, respErr := s.blockHashFromParam(param)
	if respErr != nil {
		return nil, respErr
	}

	block, err := s.chain.GetBlock(hash)
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrUnknownBlock, err.Error())
	}

	if v, _ := reqParams.Value(1).GetBoolean(); v {
		res := result.Block{
			Block:         *block,
			BlockMetadata: s.fillBlockMetadata(block, &block.Header),
		}
		return res, nil
	}
	writer := io.NewBufBinWriter()
	block.EncodeBinary(writer.BinWriter)
	return writer.Bytes(), nil
}

func (s *Server) getBlockHash(reqParams params.Params) (any, *neorpc.Error) {
	num, err := s.blockHeightFromParam(reqParams.Value(0))
	if err != nil {
		return nil, err
	}

	return s.chain.GetHeaderHash(num), nil
}

func (s *Server) getVersion(_ params.Params) (any, *neorpc.Error) {
	port, err := s.coreServer.Port(nil) // any port will suite
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("cannot fetch tcp port: %s", err))
	}

	cfg := s.chain.GetConfig()
	hfs := make(map[config.Hardfork]uint32, len(cfg.Hardforks))
	for _, cfgHf := range config.Hardforks {
		height, ok := cfg.Hardforks[cfgHf.String()]
		if !ok {
			continue
		}
		hfs[cfgHf] = height
	}
	standbyCommittee, err := keys.NewPublicKeysFromStrings(cfg.StandbyCommittee)
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("cannot fetch standbyCommittee: %s", err))
	}
	return &result.Version{
		TCPPort:   port,
		Nonce:     s.coreServer.ID(),
		UserAgent: s.coreServer.UserAgent,
		RPC: result.RPC{
			MaxIteratorResultItems:  s.config.MaxIteratorResultItems,
			SessionEnabled:          s.config.SessionEnabled,
			SessionExpansionEnabled: s.config.SessionExpansionEnabled,
		},
		Protocol: result.Protocol{
			AddressVersion:              address.NEO3Prefix,
			Network:                     cfg.Magic,
			MillisecondsPerBlock:        int(s.chain.GetMillisecondsPerBlock()),
			MaxTraceableBlocks:          s.chain.GetMaxTraceableBlocks(),
			MaxValidUntilBlockIncrement: s.chain.GetMaxValidUntilBlockIncrement(),
			MaxTransactionsPerBlock:     cfg.MaxTransactionsPerBlock,
			MemoryPoolMaxTransactions:   cfg.MemPoolSize,
			ValidatorsCount:             byte(cfg.GetNumOfCNs(s.chain.BlockHeight())),
			InitialGasDistribution:      cfg.InitialGASSupply,
			Hardforks:                   hfs,
			StandbyCommittee:            standbyCommittee,
			SeedList:                    cfg.SeedList,

			CommitteeHistory:  cfg.CommitteeHistory,
			P2PSigExtensions:  cfg.P2PSigExtensions,
			StateRootInHeader: cfg.StateRootInHeader,
			ValidatorsHistory: cfg.ValidatorsHistory,
		},
	}, nil
}

func (s *Server) getPeers(_ params.Params) (any, *neorpc.Error) {
	peers := result.NewGetPeers()
	peers.AddUnconnected(s.coreServer.UnconnectedPeers())
	peers.AddConnected(s.coreServer.ConnectedPeers())
	peers.AddBad(s.coreServer.BadPeers())
	return peers, nil
}

func (s *Server) getRawMempool(reqParams params.Params) (any, *neorpc.Error) {
	verbose, _ := reqParams.Value(0).GetBoolean()
	mp := s.chain.GetMemPool()
	txes := mp.GetVerifiedTransactions()
	hashList := make([]util.Uint256, len(txes))
	for i := range txes {
		hashList[i] = txes[i].Hash()
	}
	if !verbose {
		return hashList, nil
	}
	return result.RawMempool{
		Height:     s.chain.BlockHeight(),
		Verified:   hashList,
		Unverified: []util.Uint256{}, // avoid `null` result
	}, nil
}

func (s *Server) validateAddress(reqParams params.Params) (any, *neorpc.Error) {
	param, err := reqParams.Value(0).GetString()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}

	return result.ValidateAddress{
		Address: reqParams.Value(0),
		IsValid: validateAddress(param),
	}, nil
}

// calculateNetworkFee calculates network fee for the transaction.
func (s *Server) calculateNetworkFee(reqParams params.Params) (any, *neorpc.Error) {
	if len(reqParams) < 1 {
		return 0, neorpc.ErrInvalidParams
	}
	byteTx, err := reqParams[0].GetBytesBase64()
	if err != nil {
		return 0, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
	}
	tx, err := transaction.NewTransactionFromBytes(byteTx)
	if err != nil {
		return 0, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
	}
	hashablePart, err := tx.EncodeHashableFields()
	if err != nil {
		return 0, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("failed to compute tx size: %s", err))
	}
	size := len(hashablePart) + io.GetVarSize(len(tx.Signers))
	var (
		netFee int64
		// Verification GAS cost can't exceed chin policy, but RPC config can limit it further.
		gasLimit = min(s.chain.GetMaxVerificationGAS(), int64(s.config.MaxGasInvoke))
	)
	for i, signer := range tx.Signers {
		w := tx.Scripts[i]
		if len(w.InvocationScript) == 0 { // No invocation provided, try to infer one.
			var paramz []manifest.Parameter
			if len(w.VerificationScript) == 0 { // Contract-based verification
				cs := s.chain.GetContractState(signer.Account)
				if cs == nil {
					return 0, neorpc.WrapErrorWithData(neorpc.ErrInvalidVerificationFunction, fmt.Sprintf("signer %d has no verification script and no deployed contract", i))
				}
				md := cs.Manifest.ABI.GetMethod(manifest.MethodVerify, -1)
				if md == nil || md.ReturnType != smartcontract.BoolType {
					return 0, neorpc.WrapErrorWithData(neorpc.ErrInvalidVerificationFunction, fmt.Sprintf("signer %d has no verify method in deployed contract", i))
				}
				paramz = md.Parameters // Might as well have none params and it's OK.
			} else { // Regular signature verification.
				if vm.IsSignatureContract(w.VerificationScript) {
					paramz = []manifest.Parameter{{Type: smartcontract.SignatureType}}
				} else if nSigs, _, ok := vm.ParseMultiSigContract(w.VerificationScript); ok {
					paramz = make([]manifest.Parameter, nSigs)
					for j := range paramz {
						paramz[j] = manifest.Parameter{Type: smartcontract.SignatureType}
					}
				}
			}
			inv := io.NewBufBinWriter()
			for _, p := range paramz {
				p.Type.EncodeDefaultValue(inv.BinWriter)
			}
			if inv.Err != nil {
				return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to create dummy invocation script (signer %d): %s", i, inv.Err.Error()))
			}
			w.InvocationScript = inv.Bytes()
		}
		gasConsumed, err := s.chain.VerifyWitness(signer.Account, tx, &w, gasLimit)
		if err != nil && !errors.Is(err, core.ErrInvalidSignature) {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidSignature, fmt.Sprintf("witness %d: %s", i, err))
		}
		gasLimit -= gasConsumed
		netFee += gasConsumed
		size += io.GetVarSize(w.VerificationScript) + io.GetVarSize(w.InvocationScript)
	}
	netFee += int64(size)*s.chain.FeePerByte() + s.chain.CalculateAttributesFee(tx)
	return result.NetworkFee{Value: netFee}, nil
}

// getApplicationLog returns the contract log based on the specified txid or blockid.
func (s *Server) getApplicationLog(reqParams params.Params) (any, *neorpc.Error) {
	hash, err := reqParams.Value(0).GetUint256()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}

	trig := trigger.All
	if len(reqParams) > 1 {
		trigString, err := reqParams.Value(1).GetString()
		if err != nil {
			return nil, neorpc.ErrInvalidParams
		}
		trig, err = trigger.FromString(trigString)
		if err != nil {
			return nil, neorpc.ErrInvalidParams
		}
	}

	appExecResults, err := s.chain.GetAppExecResults(hash, trigger.All)
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrUnknownScriptContainer, fmt.Sprintf("failed to locate application log: %s", err))
	}
	return result.NewApplicationLog(hash, appExecResults, trig), nil
}

func (s *Server) getNEP11Tokens(h util.Uint160, acc util.Uint160, bw *io.BufBinWriter) ([]stackitem.Item, string, int, error) {
	items, finalize, err := s.invokeReadOnlyMulti(bw, h, []string{"tokensOf", "symbol", "decimals"}, [][]any{{acc}, nil, nil})
	if err != nil {
		return nil, "", 0, err
	}
	defer finalize()
	if (items[0].Type() != stackitem.InteropT) || !iterator.IsIterator(items[0]) {
		return nil, "", 0, fmt.Errorf("invalid `tokensOf` result type %s", items[0].String())
	}
	vals := iterator.Values(items[0], s.config.MaxNEP11Tokens)
	sym, err := stackitem.ToString(items[1])
	if err != nil {
		return nil, "", 0, fmt.Errorf("`symbol` return value error: %w", err)
	}
	dec, err := items[2].TryInteger()
	if err != nil {
		return nil, "", 0, fmt.Errorf("`decimals` return value error: %w", err)
	}
	if !dec.IsInt64() || dec.Sign() == -1 || dec.Int64() > math.MaxInt32 {
		return nil, "", 0, errors.New("`decimals` returned a bad integer")
	}
	return vals, sym, int(dec.Int64()), nil
}

func (s *Server) getNEP11Balances(ps params.Params) (any, *neorpc.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}

	bs := &result.NEP11Balances{
		Address:  address.Uint160ToString(u),
		Balances: []result.NEP11AssetBalance{},
	}
	lastUpdated, err := s.chain.GetTokenLastUpdated(u)
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("Failed to get NEP-11 last updated block: %s", err.Error()))
	}
	var count int
	stateSyncPoint := lastUpdated[math.MinInt32]
	bw := io.NewBufBinWriter()
contract_loop:
	for _, h := range s.chain.GetNEP11Contracts() {
		toks, sym, dec, err := s.getNEP11Tokens(h, u, bw)
		if err != nil {
			continue
		}
		if len(toks) == 0 {
			continue
		}
		cs := s.chain.GetContractState(h)
		if cs == nil {
			continue
		}
		isDivisible := (standard.ComplyABI(&cs.Manifest, standard.Nep11Divisible) == nil)
		lub, ok := lastUpdated[cs.ID]
		if !ok {
			cfg := s.chain.GetConfig()
			if !cfg.P2PStateExchangeExtensions && cfg.Ledger.RemoveUntraceableBlocks {
				return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to get LastUpdatedBlock for balance of %s token: internal database inconsistency", cs.Hash.StringLE()))
			}
			lub = stateSyncPoint
		}
		bs.Balances = append(bs.Balances, result.NEP11AssetBalance{
			Asset:    h,
			Decimals: dec,
			Name:     cs.Manifest.Name,
			Symbol:   sym,
			Tokens:   make([]result.NEP11TokenBalance, 0, len(toks)),
		})
		curAsset := &bs.Balances[len(bs.Balances)-1]
		for i := range toks {
			id, err := toks[i].TryBytes()
			if err != nil || len(id) > limits.MaxStorageKeyLen {
				continue
			}
			var amount = "1"
			if isDivisible {
				balance, err := s.getNEP11DTokenBalance(h, u, id, bw)
				if err != nil {
					continue
				}
				if balance.Sign() == 0 {
					continue
				}
				amount = balance.String()
			}
			count++
			curAsset.Tokens = append(curAsset.Tokens, result.NEP11TokenBalance{
				ID:          hex.EncodeToString(id),
				Amount:      amount,
				LastUpdated: lub,
			})
			if count >= s.config.MaxNEP11Tokens {
				break contract_loop
			}
		}
	}
	return bs, nil
}

func (s *Server) invokeNEP11Properties(h util.Uint160, id []byte, bw *io.BufBinWriter) ([]stackitem.MapElement, error) {
	item, finalize, err := s.invokeReadOnly(bw, h, "properties", id)
	if err != nil {
		return nil, err
	}
	defer finalize()
	if item.Type() != stackitem.MapT {
		return nil, fmt.Errorf("invalid `properties` result type %s", item.String())
	}
	return item.Value().([]stackitem.MapElement), nil
}

func (s *Server) getNEP11Properties(ps params.Params) (any, *neorpc.Error) {
	asset, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	token, err := ps.Value(1).GetBytesHex()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	props, err := s.invokeNEP11Properties(asset, token, nil)
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrExecutionFailed, fmt.Sprintf("Failed to get NEP-11 properties: %s", err.Error()))
	}
	res := make(map[string]any)
	for _, kv := range props {
		key, err := kv.Key.TryBytes()
		if err != nil {
			continue
		}
		var val any
		if result.KnownNEP11Properties[string(key)] || kv.Value.Type() != stackitem.AnyT {
			v, err := kv.Value.TryBytes()
			if err != nil {
				continue
			}
			if result.KnownNEP11Properties[string(key)] {
				val = string(v)
			} else {
				val = v
			}
		}
		res[string(key)] = val
	}
	return res, nil
}

func (s *Server) getNEP17Balances(ps params.Params) (any, *neorpc.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}

	bs := &result.NEP17Balances{
		Address:  address.Uint160ToString(u),
		Balances: []result.NEP17Balance{},
	}
	lastUpdated, err := s.chain.GetTokenLastUpdated(u)
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("Failed to get NEP-17 last updated block: %s", err.Error()))
	}
	stateSyncPoint := lastUpdated[math.MinInt32]
	bw := io.NewBufBinWriter()
	for _, h := range s.chain.GetNEP17Contracts() {
		balance, sym, dec, err := s.getNEP17TokenBalance(h, u, bw)
		if err != nil {
			continue
		}
		if balance.Sign() == 0 {
			continue
		}
		cs := s.chain.GetContractState(h)
		if cs == nil {
			continue
		}
		lub, ok := lastUpdated[cs.ID]
		if !ok {
			cfg := s.chain.GetConfig()
			if !cfg.P2PStateExchangeExtensions && cfg.Ledger.RemoveUntraceableBlocks {
				return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to get LastUpdatedBlock for balance of %s token: internal database inconsistency", cs.Hash.StringLE()))
			}
			lub = stateSyncPoint
		}
		bs.Balances = append(bs.Balances, result.NEP17Balance{
			Asset:       h,
			Amount:      balance.String(),
			Decimals:    dec,
			LastUpdated: lub,
			Name:        cs.Manifest.Name,
			Symbol:      sym,
		})
	}
	return bs, nil
}

func (s *Server) invokeReadOnly(bw *io.BufBinWriter, h util.Uint160, method string, params ...any) (stackitem.Item, func(), error) {
	r, f, err := s.invokeReadOnlyMulti(bw, h, []string{method}, [][]any{params})
	if err != nil {
		return nil, nil, err
	}
	return r[0], f, nil
}

func (s *Server) invokeReadOnlyMulti(bw *io.BufBinWriter, h util.Uint160, methods []string, params [][]any) ([]stackitem.Item, func(), error) {
	if bw == nil {
		bw = io.NewBufBinWriter()
	} else {
		bw.Reset()
	}
	if len(methods) != len(params) {
		return nil, nil, fmt.Errorf("asymmetric parameters")
	}
	for i := range methods {
		emit.AppCall(bw.BinWriter, h, methods[i], callflag.ReadStates|callflag.AllowCall, params[i]...)
		if bw.Err != nil {
			return nil, nil, fmt.Errorf("failed to create `%s` invocation script: %w", methods[i], bw.Err)
		}
	}
	script := bw.Bytes()
	tx := &transaction.Transaction{Script: script}
	ic, err := s.chain.GetTestVM(trigger.Application, tx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("faile to prepare test VM: %w", err)
	}
	ic.VM.GasLimit = core.HeaderVerificationGasLimit
	ic.VM.LoadScriptWithFlags(script, callflag.All)
	err = ic.VM.Run()
	if err != nil {
		ic.Finalize()
		return nil, nil, fmt.Errorf("failed to run %d methods of %s: %w", len(methods), h.StringLE(), err)
	}
	estack := ic.VM.Estack()
	if estack.Len() != len(methods) {
		ic.Finalize()
		return nil, nil, fmt.Errorf("invalid return values count: expected %d, got %d", len(methods), estack.Len())
	}
	return estack.ToArray(), ic.Finalize, nil
}

func (s *Server) getNEP17TokenBalance(h util.Uint160, acc util.Uint160, bw *io.BufBinWriter) (*big.Int, string, int, error) {
	items, finalize, err := s.invokeReadOnlyMulti(bw, h, []string{"balanceOf", "symbol", "decimals"}, [][]any{{acc}, nil, nil})
	if err != nil {
		return nil, "", 0, err
	}
	finalize()
	res, err := items[0].TryInteger()
	if err != nil {
		return nil, "", 0, fmt.Errorf("unexpected `balanceOf` result type: %w", err)
	}
	sym, err := stackitem.ToString(items[1])
	if err != nil {
		return nil, "", 0, fmt.Errorf("`symbol` return value error: %w", err)
	}
	dec, err := items[2].TryInteger()
	if err != nil {
		return nil, "", 0, fmt.Errorf("`decimals` return value error: %w", err)
	}
	if !dec.IsInt64() || dec.Sign() == -1 || dec.Int64() > math.MaxInt32 {
		return nil, "", 0, errors.New("`decimals` returned a bad integer")
	}
	return res, sym, int(dec.Int64()), nil
}

func (s *Server) getNEP11DTokenBalance(h util.Uint160, acc util.Uint160, id []byte, bw *io.BufBinWriter) (*big.Int, error) {
	item, finalize, err := s.invokeReadOnly(bw, h, "balanceOf", acc, id)
	if err != nil {
		return nil, err
	}
	finalize()
	res, err := item.TryInteger()
	if err != nil {
		return nil, fmt.Errorf("unexpected `balanceOf` result type: %w", err)
	}
	return res, nil
}

func getTimestampsAndLimit(ps params.Params, index int) (uint64, uint64, int, int, error) {
	var start, end uint64
	var limit, page int

	limit = maxTransfersLimit
	pStart, pEnd, pLimit, pPage := ps.Value(index), ps.Value(index+1), ps.Value(index+2), ps.Value(index+3)
	if pPage != nil {
		p, err := pPage.GetInt()
		if err != nil {
			return 0, 0, 0, 0, err
		}
		if p < 0 {
			return 0, 0, 0, 0, errors.New("can't use negative page")
		}
		page = p
	}
	if pLimit != nil {
		l, err := pLimit.GetInt()
		if err != nil {
			return 0, 0, 0, 0, err
		}
		if l <= 0 {
			return 0, 0, 0, 0, errors.New("can't use negative or zero limit")
		}
		if l > maxTransfersLimit {
			return 0, 0, 0, 0, errors.New("too big limit requested")
		}
		limit = l
	}
	if pEnd != nil {
		val, err := pEnd.GetInt()
		if err != nil {
			return 0, 0, 0, 0, err
		}
		end = uint64(val)
	} else {
		end = uint64(time.Now().Unix() * 1000)
	}
	if pStart != nil {
		val, err := pStart.GetInt()
		if err != nil {
			return 0, 0, 0, 0, err
		}
		start = uint64(val)
	} else {
		start = uint64(time.Now().Add(-time.Hour*24*7).Unix() * 1000)
	}
	return start, end, limit, page, nil
}

func (s *Server) getNEP11Transfers(ps params.Params) (any, *neorpc.Error) {
	return s.getTokenTransfers(ps, true)
}

func (s *Server) getNEP17Transfers(ps params.Params) (any, *neorpc.Error) {
	return s.getTokenTransfers(ps, false)
}

func (s *Server) getTokenTransfers(ps params.Params, isNEP11 bool) (any, *neorpc.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}

	start, end, limit, page, err := getTimestampsAndLimit(ps, 1)
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("malformed timestamps/limit: %s", err))
	}

	bs := &tokenTransfers{
		Address:  address.Uint160ToString(u),
		Received: []any{},
		Sent:     []any{},
	}
	cache := make(map[int32]util.Uint160)
	var resCount, frameCount int
	// handleTransfer returns items to be added into the received and sent arrays
	// along with a continue flag and error.
	var handleTransfer = func(tr *state.NEP17Transfer) (*result.NEP17Transfer, *result.NEP17Transfer, bool, error) {
		var received, sent *result.NEP17Transfer

		// Iterating from the newest to the oldest, not yet reached required
		// time frame, continue looping.
		if tr.Timestamp > end {
			return nil, nil, true, nil
		}
		// Iterating from the newest to the oldest, moved past required
		// time frame, stop looping.
		if tr.Timestamp < start {
			return nil, nil, false, nil
		}
		frameCount++
		// Using limits, not yet reached required page.
		if limit != 0 && page*limit >= frameCount {
			return nil, nil, true, nil
		}

		h, err := s.getHash(tr.Asset, cache)
		if err != nil {
			return nil, nil, false, err
		}

		transfer := result.NEP17Transfer{
			Timestamp: tr.Timestamp,
			Asset:     h,
			Index:     tr.Block,
			TxHash:    tr.Tx,
		}
		if !tr.Counterparty.Equals(util.Uint160{}) {
			transfer.Address = address.Uint160ToString(tr.Counterparty)
		}
		if tr.Amount.Sign() > 0 { // token was received
			transfer.Amount = tr.Amount.String()
			received = &result.NEP17Transfer{}
			*received = transfer // Make a copy, transfer is to be modified below.
		} else {
			transfer.Amount = new(big.Int).Neg(tr.Amount).String()
			sent = &result.NEP17Transfer{}
			*sent = transfer
		}

		resCount++
		// Check limits for continue flag.
		return received, sent, !(limit != 0 && resCount >= limit), nil
	}
	if !isNEP11 {
		err = s.chain.ForEachNEP17Transfer(u, end, func(tr *state.NEP17Transfer) (bool, error) {
			r, s, res, err := handleTransfer(tr)
			if err == nil {
				if r != nil {
					bs.Received = append(bs.Received, r)
				}
				if s != nil {
					bs.Sent = append(bs.Sent, s)
				}
			}
			return res, err
		})
	} else {
		err = s.chain.ForEachNEP11Transfer(u, end, func(tr *state.NEP11Transfer) (bool, error) {
			r, s, res, err := handleTransfer(&tr.NEP17Transfer)
			if err == nil {
				id := hex.EncodeToString(tr.ID)
				if r != nil {
					bs.Received = append(bs.Received, nep17TransferToNEP11(r, id))
				}
				if s != nil {
					bs.Sent = append(bs.Sent, nep17TransferToNEP11(s, id))
				}
			}
			return res, err
		})
	}
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("invalid transfer log: %s", err))
	}
	return bs, nil
}

// getHash returns the hash of the contract by its ID using cache.
func (s *Server) getHash(contractID int32, cache map[int32]util.Uint160) (util.Uint160, error) {
	if d, ok := cache[contractID]; ok {
		return d, nil
	}
	h, err := s.chain.GetContractScriptHash(contractID)
	if err != nil {
		return util.Uint160{}, err
	}
	cache[contractID] = h
	return h, nil
}

func (s *Server) contractIDFromParam(param *params.Param, root ...util.Uint256) (int32, *neorpc.Error) {
	var result int32
	if param == nil {
		return 0, neorpc.ErrInvalidParams
	}
	if scriptHash, err := param.GetUint160FromHex(); err == nil {
		if len(root) == 0 {
			cs := s.chain.GetContractState(scriptHash)
			if cs == nil {
				return 0, neorpc.ErrUnknownContract
			}
			result = cs.ID
		} else {
			cs, respErr := s.getHistoricalContractState(root[0], scriptHash)
			if respErr != nil {
				return 0, respErr
			}
			result = cs.ID
		}
	} else {
		id, err := param.GetInt()
		if err != nil {
			return 0, neorpc.ErrInvalidParams
		}
		if err := checkInt32(id); err != nil {
			return 0, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
		}
		result = int32(id)
	}
	return result, nil
}

// contractScriptHashFromParam returns the contract script hash by hex contract hash, address, id or native contract name.
func (s *Server) contractScriptHashFromParam(param *params.Param) (util.Uint160, *neorpc.Error) {
	var result util.Uint160
	if param == nil {
		return result, neorpc.ErrInvalidParams
	}
	nameOrHashOrIndex, err := param.GetString()
	if err != nil {
		return result, neorpc.ErrInvalidParams
	}
	result, err = param.GetUint160FromAddressOrHex()
	if err == nil {
		return result, nil
	}
	result, err = s.chain.GetNativeContractScriptHash(nameOrHashOrIndex)
	if err == nil {
		return result, nil
	}
	id, err := strconv.Atoi(nameOrHashOrIndex)
	if err != nil {
		return result, neorpc.NewInvalidParamsError(fmt.Sprintf("Invalid contract identifier (name/hash/index is expected) : %s", err.Error()))
	}
	if err := checkInt32(id); err != nil {
		return result, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
	}
	result, err = s.chain.GetContractScriptHash(int32(id))
	if err != nil {
		return result, neorpc.ErrUnknownContract
	}
	return result, nil
}

func makeStorageKey(id int32, key []byte) []byte {
	skey := make([]byte, 4+len(key))
	binary.LittleEndian.PutUint32(skey, uint32(id))
	copy(skey[4:], key)
	return skey
}

var errKeepOnlyLatestState = errors.New("'KeepOnlyLatestState' setting is enabled")

func (s *Server) getProof(ps params.Params) (any, *neorpc.Error) {
	if s.chain.GetConfig().Ledger.KeepOnlyLatestState {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrUnsupportedState, fmt.Sprintf("'getproof' is not supported: %s", errKeepOnlyLatestState))
	}
	root, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	sc, err := ps.Value(1).GetUint160FromHex()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	key, err := ps.Value(2).GetBytesBase64()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	cs, respErr := s.getHistoricalContractState(root, sc)
	if respErr != nil {
		return nil, respErr
	}
	skey := makeStorageKey(cs.ID, key)
	proof, err := s.chain.GetStateModule().GetStateProof(root, skey)
	if err != nil {
		if errors.Is(err, mpt.ErrNotFound) {
			return nil, neorpc.ErrUnknownStorageItem
		}
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to get proof: %s", err))
	}
	return &result.ProofWithKey{
		Key:   skey,
		Proof: proof,
	}, nil
}

func (s *Server) verifyProof(ps params.Params) (any, *neorpc.Error) {
	if s.chain.GetConfig().Ledger.KeepOnlyLatestState {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrUnsupportedState, fmt.Sprintf("'verifyproof' is not supported: %s", errKeepOnlyLatestState))
	}
	root, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	proofStr, err := ps.Value(1).GetString()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	var p result.ProofWithKey
	if err := p.FromString(proofStr); err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	vp := new(result.VerifyProof)
	val, ok := mpt.VerifyProof(root, p.Key, p.Proof)
	if !ok {
		return nil, neorpc.ErrInvalidProof
	}
	vp.Value = val
	return vp, nil
}

func (s *Server) getState(ps params.Params) (any, *neorpc.Error) {
	root, respErr := s.getStateRootFromParam(ps.Value(0))
	if respErr != nil {
		return nil, respErr
	}
	csHash, err := ps.Value(1).GetUint160FromHex()
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, "invalid contract hash")
	}
	key, err := ps.Value(2).GetBytesBase64()
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, "invalid key")
	}
	cs, respErr := s.getHistoricalContractState(root, csHash)
	if respErr != nil {
		return nil, respErr
	}
	sKey := makeStorageKey(cs.ID, key)
	res, err := s.chain.GetStateModule().GetState(root, sKey)
	if err != nil {
		if errors.Is(err, mpt.ErrNotFound) {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid key: %s", err.Error()))
		}
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("Failed to get historical item state: %s", err.Error()))
	}
	return res, nil
}

func (s *Server) findStates(ps params.Params) (any, *neorpc.Error) {
	root, respErr := s.getStateRootFromParam(ps.Value(0))
	if respErr != nil {
		return nil, respErr
	}
	csHash, err := ps.Value(1).GetUint160FromHex()
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid contract hash: %s", err))
	}
	prefix, err := ps.Value(2).GetBytesBase64()
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid prefix: %s", err))
	}
	var (
		key   []byte
		count = s.config.MaxFindResultItems
	)
	if len(ps) > 3 {
		key, err = ps.Value(3).GetBytesBase64()
		if err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid key: %s", err))
		}
		if len(key) > 0 {
			if !bytes.HasPrefix(key, prefix) {
				return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, "key doesn't match prefix")
			}
			key = key[len(prefix):]
		} else {
			// empty ("") key shouldn't exclude item matching prefix from the result
			key = nil
		}
	}
	if len(ps) > 4 {
		count, err = ps.Value(4).GetInt()
		if err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid count: %s", err))
		}
		count = min(count, s.config.MaxFindResultItems)
	}
	cs, respErr := s.getHistoricalContractState(root, csHash)
	if respErr != nil {
		return nil, respErr
	}
	pKey := makeStorageKey(cs.ID, prefix)
	kvs, err := s.chain.GetStateModule().FindStates(root, pKey, key, count+1) // +1 to define result truncation
	if err != nil && !errors.Is(err, mpt.ErrNotFound) {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to find state items: %s", err))
	}
	res := result.FindStates{}
	if len(kvs) == count+1 {
		res.Truncated = true
		kvs = kvs[:len(kvs)-1]
	}
	if len(kvs) > 0 {
		proof, err := s.chain.GetStateModule().GetStateProof(root, kvs[0].Key)
		if err != nil {
			return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to get first proof: %s", err))
		}
		res.FirstProof = &result.ProofWithKey{
			Key:   kvs[0].Key,
			Proof: proof,
		}
	}
	if len(kvs) > 1 {
		proof, err := s.chain.GetStateModule().GetStateProof(root, kvs[len(kvs)-1].Key)
		if err != nil {
			return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to get last proof: %s", err))
		}
		res.LastProof = &result.ProofWithKey{
			Key:   kvs[len(kvs)-1].Key,
			Proof: proof,
		}
	}
	res.Results = make([]result.KeyValue, len(kvs))
	for i, kv := range kvs {
		res.Results[i] = result.KeyValue{
			Key:   kv.Key[4:], // cut contract ID as it is done in C#
			Value: kv.Value,
		}
	}
	return res, nil
}

// getStateRootFromParam retrieves state root hash from the provided parameter
// (only util.Uint256 serialized representation is allowed) and checks whether
// MPT states are supported for the old stateroot.
func (s *Server) getStateRootFromParam(p *params.Param) (util.Uint256, *neorpc.Error) {
	root, err := p.GetUint256()
	if err != nil {
		return util.Uint256{}, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, "invalid stateroot")
	}
	if s.chain.GetConfig().Ledger.KeepOnlyLatestState {
		curr, err := s.chain.GetStateModule().GetStateRoot(s.chain.BlockHeight())
		if err != nil {
			return util.Uint256{}, neorpc.NewInternalServerError(fmt.Sprintf("failed to get current stateroot: %s", err))
		}
		if !curr.Root.Equals(root) {
			return util.Uint256{}, neorpc.WrapErrorWithData(neorpc.ErrUnsupportedState, fmt.Sprintf("state-based methods are not supported for old states: %s", errKeepOnlyLatestState))
		}
	}
	return root, nil
}

func (s *Server) findStorage(reqParams params.Params) (any, *neorpc.Error) {
	id, prefix, start, take, respErr := s.getFindStorageParams(reqParams)
	if respErr != nil {
		return nil, respErr
	}
	return s.findStorageInternal(id, prefix, start, take, s.chain)
}

func (s *Server) findStorageInternal(id int32, prefix []byte, start, take int, seeker ContractStorageSeeker) (any, *neorpc.Error) {
	var (
		i   int
		end = start + take
		// Result is an empty list if a contract state is not found as it is in C# implementation.
		res = &result.FindStorage{Results: make([]result.KeyValue, 0)}
	)
	seeker.SeekStorage(id, prefix, func(k, v []byte) bool {
		if i < start {
			i++
			return true
		}
		if i < end {
			res.Results = append(res.Results, result.KeyValue{
				Key:   bytes.Clone(append(prefix, k...)), // Don't strip prefix, as it is done in C#.
				Value: v,
			})
			i++
			return true
		}
		res.Truncated = true
		return false
	})
	res.Next = i
	return res, nil
}

func (s *Server) findStorageHistoric(reqParams params.Params) (any, *neorpc.Error) {
	root, respErr := s.getStateRootFromParam(reqParams.Value(0))
	if respErr != nil {
		return nil, respErr
	}
	if len(reqParams) < 2 {
		return nil, neorpc.ErrInvalidParams
	}
	id, prefix, start, take, respErr := s.getFindStorageParams(reqParams[1:], root)
	if respErr != nil {
		return nil, respErr
	}

	return s.findStorageInternal(id, prefix, start, take, mptStorageSeeker{
		root:   root,
		module: s.chain.GetStateModule(),
	})
}

// mptStorageSeeker is an auxiliary structure that implements ContractStorageSeeker interface.
type mptStorageSeeker struct {
	root   util.Uint256
	module core.StateRoot
}

func (s mptStorageSeeker) SeekStorage(id int32, prefix []byte, cont func(k, v []byte) bool) {
	key := makeStorageKey(id, prefix)
	s.module.SeekStates(s.root, key, cont)
}

func (s *Server) getFindStorageParams(reqParams params.Params, root ...util.Uint256) (int32, []byte, int, int, *neorpc.Error) {
	if len(reqParams) < 2 {
		return 0, nil, 0, 0, neorpc.ErrInvalidParams
	}
	id, respErr := s.contractIDFromParam(reqParams.Value(0), root...)
	if respErr != nil {
		return 0, nil, 0, 0, respErr
	}

	prefix, err := reqParams.Value(1).GetBytesBase64()
	if err != nil {
		return 0, nil, 0, 0, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid prefix: %s", err))
	}

	var skip int
	if len(reqParams) > 2 {
		skip, err = reqParams.Value(2).GetInt()
		if err != nil {
			return 0, nil, 0, 0, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid start: %s", err))
		}
	}
	return id, prefix, skip, s.config.MaxFindStorageResultItems, nil
}

func (s *Server) getHistoricalContractState(root util.Uint256, csHash util.Uint160) (*state.Contract, *neorpc.Error) {
	csKey := makeStorageKey(native.ManagementContractID, native.MakeContractKey(csHash))
	csBytes, err := s.chain.GetStateModule().GetState(root, csKey)
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrUnknownContract, fmt.Sprintf("Failed to get historical contract state: %s", err.Error()))
	}
	contract := new(state.Contract)
	err = stackitem.DeserializeConvertible(csBytes, contract)
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to deserialize historical contract state: %s", err))
	}
	return contract, nil
}

func (s *Server) getStateHeight(_ params.Params) (any, *neorpc.Error) {
	var height = s.chain.BlockHeight()
	var stateHeight = s.chain.GetStateModule().CurrentValidatedHeight()
	if s.chain.GetConfig().StateRootInHeader {
		stateHeight = height - 1
	}
	return &result.StateHeight{
		Local:     height,
		Validated: stateHeight,
	}, nil
}

func (s *Server) getStateRoot(ps params.Params) (any, *neorpc.Error) {
	p := ps.Value(0)
	if p == nil {
		return nil, neorpc.NewInvalidParamsError("missing stateroot identifier")
	}
	var rt *state.MPTRoot
	var h util.Uint256
	height, err := p.GetIntStrict()
	if err == nil {
		if err := checkUint32(height); err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
		}
		rt, err = s.chain.GetStateModule().GetStateRoot(uint32(height))
	} else if h, err = p.GetUint256(); err == nil {
		var hdr *block.Header
		hdr, err = s.chain.GetHeader(h)
		if err == nil {
			rt, err = s.chain.GetStateModule().GetStateRoot(hdr.Index)
		}
	}
	if err != nil {
		return nil, neorpc.ErrUnknownStateRoot
	}
	return rt, nil
}

func (s *Server) getStorage(ps params.Params) (any, *neorpc.Error) {
	id, rErr := s.contractIDFromParam(ps.Value(0))
	if rErr != nil {
		return nil, rErr
	}

	key, err := ps.Value(1).GetBytesBase64()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}

	item := s.chain.GetStorageItem(id, key)
	if item == nil {
		return "", neorpc.ErrUnknownStorageItem
	}

	return []byte(item), nil
}

func (s *Server) getStorageHistoric(ps params.Params) (any, *neorpc.Error) {
	root, respErr := s.getStateRootFromParam(ps.Value(0))
	if respErr != nil {
		return nil, respErr
	}
	if len(ps) < 2 {
		return nil, neorpc.ErrInvalidParams
	}

	id, rErr := s.contractIDFromParam(ps.Value(1), root)
	if rErr != nil {
		return nil, rErr
	}
	key, err := ps.Value(2).GetBytesBase64()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	pKey := makeStorageKey(id, key)

	v, err := s.chain.GetStateModule().GetState(root, pKey)
	if err != nil && !errors.Is(err, mpt.ErrNotFound) {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to get state item: %s", err))
	}
	if v == nil {
		return "", neorpc.ErrUnknownStorageItem
	}

	return v, nil
}

func (s *Server) getrawtransaction(reqParams params.Params) (any, *neorpc.Error) {
	txHash, err := reqParams.Value(0).GetUint256()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	tx, height, err := s.chain.GetTransaction(txHash)
	if err != nil {
		return nil, neorpc.ErrUnknownTransaction
	}
	if v, _ := reqParams.Value(1).GetBoolean(); v {
		res := result.TransactionOutputRaw{
			Transaction: *tx,
		}
		if height == math.MaxUint32 { // Mempooled transaction.
			return res, nil
		}
		_header := s.chain.GetHeaderHash(height)
		header, err := s.chain.GetHeader(_header)
		if err != nil {
			return nil, neorpc.NewInternalServerError(fmt.Sprintf("Failed to get header for the transaction: %s", err.Error()))
		}
		aers, err := s.chain.GetAppExecResults(txHash, trigger.Application)
		if err != nil {
			return nil, neorpc.NewInternalServerError(fmt.Sprintf("Failed to get application log for the transaction: %s", err.Error()))
		}
		if len(aers) == 0 {
			return nil, neorpc.NewInternalServerError("Inconsistent application log: application log for the transaction is empty")
		}
		res.TransactionMetadata = result.TransactionMetadata{
			Blockhash:     header.Hash(),
			Confirmations: int(s.chain.BlockHeight() - header.Index + 1),
			Timestamp:     header.Timestamp,
			VMState:       aers[0].VMState.String(),
		}
		return res, nil
	}
	return tx.Bytes(), nil
}

func (s *Server) getTransactionHeight(ps params.Params) (any, *neorpc.Error) {
	h, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}

	_, height, err := s.chain.GetTransaction(h)
	if err != nil || height == math.MaxUint32 {
		return nil, neorpc.ErrUnknownTransaction
	}

	return height, nil
}

// getContractState returns contract state (contract information, according to the contract script hash,
// contract id or native contract name).
func (s *Server) getContractState(reqParams params.Params) (any, *neorpc.Error) {
	scriptHash, err := s.contractScriptHashFromParam(reqParams.Value(0))
	if err != nil {
		return nil, err
	}
	cs := s.chain.GetContractState(scriptHash)
	if cs == nil {
		return nil, neorpc.ErrUnknownContract
	}
	return cs, nil
}

func (s *Server) getNativeContracts(_ params.Params) (any, *neorpc.Error) {
	return s.chain.GetNatives(), nil
}

// getBlockSysFee returns the system fees of the block, based on the specified index.
func (s *Server) getBlockSysFee(reqParams params.Params) (any, *neorpc.Error) {
	num, err := s.blockHeightFromParam(reqParams.Value(0))
	if err != nil {
		return 0, neorpc.WrapErrorWithData(err, fmt.Sprintf("invalid block height: %s", err.Data))
	}

	headerHash := s.chain.GetHeaderHash(num)
	block, errBlock := s.chain.GetBlock(headerHash)
	if errBlock != nil {
		return 0, neorpc.ErrUnknownBlock
	}

	var blockSysFee int64
	for _, tx := range block.Transactions {
		blockSysFee += tx.SystemFee
	}

	return blockSysFee, nil
}

// getBlockHeader returns the corresponding block header information according to the specified script hash.
func (s *Server) getBlockHeader(reqParams params.Params) (any, *neorpc.Error) {
	param := reqParams.Value(0)
	hash, respErr := s.blockHashFromParam(param)
	if respErr != nil {
		return nil, respErr
	}

	verbose, _ := reqParams.Value(1).GetBoolean()
	h, err := s.chain.GetHeader(hash)
	if err != nil {
		return nil, neorpc.ErrUnknownBlock
	}

	if verbose {
		res := result.Header{
			Header:        *h,
			BlockMetadata: s.fillBlockMetadata(h, h),
		}
		return res, nil
	}

	buf := io.NewBufBinWriter()
	h.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("encoding error: %s", buf.Err))
	}
	return buf.Bytes(), nil
}

// getUnclaimedGas returns unclaimed GAS amount of the specified address.
func (s *Server) getUnclaimedGas(ps params.Params) (any, *neorpc.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}

	neo, _ := s.chain.GetGoverningTokenBalance(u)
	if neo.Sign() == 0 {
		return result.UnclaimedGas{
			Address: u,
		}, nil
	}
	gas, err := s.chain.CalculateClaimable(u, s.chain.BlockHeight()+1) // +1 as in C#, for the next block.
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("Can't calculate claimable: %s", err.Error()))
	}
	return result.UnclaimedGas{
		Address:   u,
		Unclaimed: *gas,
	}, nil
}

// getCandidates returns the current list of candidates with their active/inactive voting status.
func (s *Server) getCandidates(_ params.Params) (any, *neorpc.Error) {
	var validators keys.PublicKeys

	validators, err := s.chain.GetNextBlockValidators()
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("Can't get next block validators: %s", err.Error()))
	}
	enrollments, err := s.chain.GetEnrollments()
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("Can't get enrollments: %s", err.Error()))
	}
	var res = make([]result.Candidate, 0, len(enrollments))
	for _, v := range enrollments {
		res = append(res, result.Candidate{
			PublicKey: *v.Key,
			Votes:     v.Votes.Int64(),
			Active:    validators.Contains(v.Key),
		})
	}
	return res, nil
}

// getNextBlockValidators returns validators for the next block with voting status.
func (s *Server) getNextBlockValidators(_ params.Params) (any, *neorpc.Error) {
	var validators keys.PublicKeys

	validators, err := s.chain.GetNextBlockValidators()
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("Can't get next block validators: %s", err.Error()))
	}
	enrollments, err := s.chain.GetEnrollments()
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("Can't get enrollments: %s", err.Error()))
	}
	var res = make([]result.Validator, 0, len(validators))
	for _, v := range enrollments {
		if !validators.Contains(v.Key) {
			continue
		}
		res = append(res, result.Validator{
			PublicKey: *v.Key,
			Votes:     v.Votes.Int64(),
		})
	}
	return res, nil
}

// getCommittee returns the current list of NEO committee members.
func (s *Server) getCommittee(_ params.Params) (any, *neorpc.Error) {
	keys, err := s.chain.GetCommittee()
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("can't get committee members: %s", err))
	}
	return keys, nil
}

// invokeFunction implements the `invokeFunction` RPC call.
func (s *Server) invokeFunction(reqParams params.Params) (any, *neorpc.Error) {
	tx, verbose, respErr := s.getInvokeFunctionParams(reqParams)
	if respErr != nil {
		return nil, respErr
	}
	return s.runScriptInVM(trigger.Application, tx.Script, util.Uint160{}, tx, nil, nil, verbose)
}

// invokeFunctionHistoric implements the `invokeFunctionHistoric` RPC call.
func (s *Server) invokeFunctionHistoric(reqParams params.Params) (any, *neorpc.Error) {
	nextH, respErr := s.getHistoricParams(reqParams)
	if respErr != nil {
		return nil, respErr
	}
	if len(reqParams) < 2 {
		return nil, neorpc.ErrInvalidParams
	}
	tx, verbose, respErr := s.getInvokeFunctionParams(reqParams[1:])
	if respErr != nil {
		return nil, respErr
	}
	return s.runScriptInVM(trigger.Application, tx.Script, util.Uint160{}, tx, nil, &nextH, verbose)
}

func (s *Server) getInvokeFunctionParams(reqParams params.Params) (*transaction.Transaction, bool, *neorpc.Error) {
	if len(reqParams) < 2 {
		return nil, false, neorpc.ErrInvalidParams
	}
	scriptHash, responseErr := s.contractScriptHashFromParam(reqParams.Value(0))
	if responseErr != nil {
		return nil, false, responseErr
	}
	method, err := reqParams[1].GetString()
	if err != nil {
		return nil, false, neorpc.ErrInvalidParams
	}
	var invparams *params.Param
	if len(reqParams) > 2 {
		invparams = &reqParams[2]
	}
	tx := &transaction.Transaction{}
	if len(reqParams) > 3 {
		signers, _, err := reqParams[3].GetSignersWithWitnesses()
		if err != nil {
			return nil, false, neorpc.ErrInvalidParams
		}
		tx.Signers = signers
	}
	var verbose bool
	if len(reqParams) > 4 {
		verbose, err = reqParams[4].GetBoolean()
		if err != nil {
			return nil, false, neorpc.ErrInvalidParams
		}
	}
	if len(tx.Signers) == 0 {
		tx.Signers = []transaction.Signer{{Account: util.Uint160{}, Scopes: transaction.None}}
	}
	script, err := params.CreateFunctionInvocationScript(scriptHash, method, invparams)
	if err != nil {
		return nil, false, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("can't create invocation script: %s", err))
	}
	tx.Script = script
	return tx, verbose, nil
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokescript(reqParams params.Params) (any, *neorpc.Error) {
	tx, verbose, respErr := s.getInvokeScriptParams(reqParams)
	if respErr != nil {
		return nil, respErr
	}
	return s.runScriptInVM(trigger.Application, tx.Script, util.Uint160{}, tx, nil, nil, verbose)
}

// invokescripthistoric implements the `invokescripthistoric` RPC call.
func (s *Server) invokescripthistoric(reqParams params.Params) (any, *neorpc.Error) {
	nextH, respErr := s.getHistoricParams(reqParams)
	if respErr != nil {
		return nil, respErr
	}
	if len(reqParams) < 2 {
		return nil, neorpc.ErrInvalidParams
	}
	tx, verbose, respErr := s.getInvokeScriptParams(reqParams[1:])
	if respErr != nil {
		return nil, respErr
	}
	return s.runScriptInVM(trigger.Application, tx.Script, util.Uint160{}, tx, nil, &nextH, verbose)
}

func (s *Server) getInvokeScriptParams(reqParams params.Params) (*transaction.Transaction, bool, *neorpc.Error) {
	script, err := reqParams.Value(0).GetBytesBase64()
	if err != nil {
		return nil, false, neorpc.ErrInvalidParams
	}

	tx := &transaction.Transaction{}
	if len(reqParams) > 1 {
		signers, witnesses, err := reqParams[1].GetSignersWithWitnesses()
		if err != nil {
			return nil, false, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
		}
		tx.Signers = signers
		tx.Scripts = witnesses
	}
	var verbose bool
	if len(reqParams) > 2 {
		verbose, err = reqParams[2].GetBoolean()
		if err != nil {
			return nil, false, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
		}
	}
	if len(tx.Signers) == 0 {
		tx.Signers = []transaction.Signer{{Account: util.Uint160{}, Scopes: transaction.None}}
	}
	tx.Script = script
	return tx, verbose, nil
}

func (s *Server) fakeTxFromParam(p *params.Param) (*transaction.Transaction, *neorpc.Error) {
	tx, err := p.GetFakeTx()
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("failed to unmarshal fake transaction: %s", err))
	}
	if len(tx.Script) == 0 {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, "transaction script is mandatory")
	}
	return tx, nil
}

func (s *Server) fakeBlockFromParam(p *params.Param) (*block.Block, *neorpc.Error) {
	base, err := s.chain.GetFakeNextBlock(s.chain.BlockHeight() + 1)
	if err != nil {
		return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to mock block execution container: %s", err))
	}
	if p != nil && !p.IsNull() {
		h, err := p.GetFakeHeader()
		if err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("failed to unmarshal fake block: %s", err))
		}

		// Override allowed base parameters with some sanity checks.
		if h.Version != 0 {
			base.Version = h.Version
		}
		if !h.PrevHash.Equals(util.Uint256{}) {
			base.PrevHash = h.PrevHash
		}
		if !h.MerkleRoot.Equals(util.Uint256{}) {
			base.MerkleRoot = h.MerkleRoot
		}
		if h.Timestamp != 0 {
			base.Timestamp = h.Timestamp
		}
		if h.Nonce != 0 {
			base.Nonce = h.Nonce
		}
		if h.Index != 0 {
			base.Index = h.Index
		}
		base.PrimaryIndex = h.PrimaryIndex
		if !h.NextConsensus.Equals(util.Uint160{}) {
			base.NextConsensus = h.NextConsensus
		}
		if base.StateRootEnabled && !h.PrevStateRoot.Equals(util.Uint256{}) {
			base.PrevStateRoot = h.PrevStateRoot
		}
	}

	return base, nil
}

// invokeContainedScript implements the `invokecontainedscript` RPC call.
func (s *Server) invokeContainedScript(reqParams params.Params) (any, *neorpc.Error) {
	if len(reqParams) < 1 {
		return nil, neorpc.ErrInvalidParams
	}
	tx, respErr := s.fakeTxFromParam(reqParams.Value(0))
	if respErr != nil {
		return nil, respErr
	}
	b, respErr := s.fakeBlockFromParam(reqParams.Value(1))
	if respErr != nil {
		return nil, respErr
	}
	var (
		trig    = trigger.Application
		verbose bool
	)
	if len(reqParams) > 2 {
		t, err := reqParams[2].GetString()
		if err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid trigger parameter: %s", err))
		}
		trig, err = trigger.FromString(t)
		if err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("failed to parse trigger: %s", err))
		}
		if trig != trigger.Application && trig != trigger.Verification {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("unsupported trigger: expected %s or %s, got %s", trigger.Application, trigger.Verification, trig))
		}
	}
	if len(reqParams) > 3 {
		var err error
		verbose, err = reqParams[3].GetBoolean()
		if err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid verbose parameter: %s", err))
		}
	}

	return s.runScriptInVM(trig, tx.Script, util.Uint160{}, tx, b, nil, verbose)
}

// invokeContractVerify implements the `invokecontractverify` RPC call.
func (s *Server) invokeContractVerify(reqParams params.Params) (any, *neorpc.Error) {
	scriptHash, tx, invocationScript, respErr := s.getInvokeContractVerifyParams(reqParams)
	if respErr != nil {
		return nil, respErr
	}
	return s.runScriptInVM(trigger.Verification, invocationScript, scriptHash, tx, nil, nil, false)
}

// invokeContractVerifyHistoric implements the `invokecontractverifyhistoric` RPC call.
func (s *Server) invokeContractVerifyHistoric(reqParams params.Params) (any, *neorpc.Error) {
	nextH, respErr := s.getHistoricParams(reqParams)
	if respErr != nil {
		return nil, respErr
	}
	if len(reqParams) < 2 {
		return nil, neorpc.ErrInvalidParams
	}
	scriptHash, tx, invocationScript, respErr := s.getInvokeContractVerifyParams(reqParams[1:])
	if respErr != nil {
		return nil, respErr
	}
	return s.runScriptInVM(trigger.Verification, invocationScript, scriptHash, tx, nil, &nextH, false)
}

func (s *Server) getInvokeContractVerifyParams(reqParams params.Params) (util.Uint160, *transaction.Transaction, []byte, *neorpc.Error) {
	scriptHash, responseErr := s.contractScriptHashFromParam(reqParams.Value(0))
	if responseErr != nil {
		return util.Uint160{}, nil, nil, responseErr
	}

	bw := io.NewBufBinWriter()
	if len(reqParams) > 1 {
		args, err := reqParams[1].GetArray() // second `invokecontractverify` parameter is an array of arguments for `verify` method
		if err != nil {
			return util.Uint160{}, nil, nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
		}
		if len(args) > 0 {
			err := params.ExpandArrayIntoScript(bw.BinWriter, args)
			if err != nil {
				return util.Uint160{}, nil, nil, neorpc.NewInternalServerError(fmt.Sprintf("can't create witness invocation script: %s", err))
			}
		}
	}
	invocationScript := bw.Bytes()

	tx := &transaction.Transaction{Script: []byte{byte(opcode.RET)}} // need something in script
	if len(reqParams) > 2 {
		signers, witnesses, err := reqParams[2].GetSignersWithWitnesses()
		if err != nil {
			return util.Uint160{}, nil, nil, neorpc.ErrInvalidParams
		}
		tx.Signers = signers
		tx.Scripts = witnesses
	} else { // fill the only known signer - the contract with `verify` method
		tx.Signers = []transaction.Signer{{Account: scriptHash}}
		tx.Scripts = []transaction.Witness{{InvocationScript: invocationScript, VerificationScript: []byte{}}}
	}
	return scriptHash, tx, invocationScript, nil
}

// getHistoricParams checks that historic calls are supported and returns index of
// a fake next block to perform the historic call. It also checks that
// specified stateroot is stored at the specified height for further request
// handling consistency.
func (s *Server) getHistoricParams(reqParams params.Params) (uint32, *neorpc.Error) {
	if s.chain.GetConfig().Ledger.KeepOnlyLatestState {
		return 0, neorpc.WrapErrorWithData(neorpc.ErrUnsupportedState, fmt.Sprintf("only latest state is supported: %s", errKeepOnlyLatestState))
	}
	if len(reqParams) < 1 {
		return 0, neorpc.ErrInvalidParams
	}
	height, respErr := s.blockHeightFromParam(reqParams.Value(0))
	if respErr != nil {
		hash, err := reqParams.Value(0).GetUint256()
		if err != nil {
			return 0, neorpc.NewInvalidParamsError(fmt.Sprintf("invalid block hash or index or stateroot hash: %s", err))
		}
		b, err := s.chain.GetBlock(hash)
		if err != nil {
			stateH, err := s.chain.GetStateModule().GetLatestStateHeight(hash)
			if err != nil {
				return 0, neorpc.NewInvalidParamsError(fmt.Sprintf("unknown block or stateroot: %s", err))
			}
			height = stateH
		} else {
			height = b.Index
		}
	}
	return height + 1, nil
}

func (s *Server) prepareInvocationContext(t trigger.Type, script []byte, contractScriptHash util.Uint160, tx *transaction.Transaction, b *block.Block, nextH *uint32, verbose bool) (*interop.Context, *neorpc.Error) {
	var (
		err error
		ic  *interop.Context
	)
	if nextH == nil {
		ic, err = s.chain.GetTestVM(t, tx, b)
		if err != nil {
			return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to create test VM: %s", err))
		}
	} else {
		ic, err = s.chain.GetTestHistoricVM(t, tx, *nextH)
		if err != nil {
			return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to create historic VM: %s", err))
		}
	}
	if verbose {
		ic.VM.EnableInvocationTree()
	}
	ic.VM.GasLimit = int64(s.config.MaxGasInvoke)
	if t == trigger.Verification {
		// We need this special case because witnesses verification is not the simple System.Contract.Call,
		// and we need to define exactly the amount of gas consumed for a contract witness verification.
		ic.VM.GasLimit = min(ic.VM.GasLimit, s.chain.GetMaxVerificationGAS())
	}

	if t == trigger.Verification && b == nil { // don't initialize verification context for contained invocations.
		err = s.chain.InitVerificationContext(ic, contractScriptHash, &transaction.Witness{InvocationScript: script, VerificationScript: []byte{}})
		if err != nil {
			switch {
			case errors.Is(err, core.ErrUnknownVerificationContract):
				return nil, neorpc.WrapErrorWithData(neorpc.ErrUnknownContract, err.Error())
			case errors.Is(err, core.ErrInvalidVerificationContract):
				return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidVerificationFunction, err.Error())
			default:
				return nil, neorpc.NewInternalServerError(fmt.Sprintf("can't prepare verification VM: %s", err))
			}
		}
	} else {
		ic.VM.LoadScriptWithFlags(script, callflag.All)
	}
	return ic, nil
}

// runScriptInVM runs the given script in a new test VM and returns the invocation
// result. The script is either a simple transaction's script in case of
// `application` trigger or witness invocation script in case of `verification`
// trigger (it pushes `verify` arguments on stack before verification). If block
// is specified, it will be used to set up execution container parameters. In case
// of contract verification contractScriptHash should be specified.
func (s *Server) runScriptInVM(t trigger.Type, script []byte, contractScriptHash util.Uint160, tx *transaction.Transaction, b *block.Block, nextH *uint32, verbose bool) (*result.Invoke, *neorpc.Error) {
	ic, respErr := s.prepareInvocationContext(t, script, contractScriptHash, tx, b, nextH, verbose)
	if respErr != nil {
		return nil, respErr
	}
	err := ic.VM.Run()
	var faultException string
	if err != nil {
		faultException = err.Error()
	}
	items := ic.VM.Estack().ToArray()
	sess := s.postProcessExecStack(items)
	var id uuid.UUID

	if sess != nil {
		// nextH == nil only when we're not using MPT-backed storage, therefore
		// the second attempt won't stop here.
		if s.config.SessionBackedByMPT && nextH == nil {
			ic.Finalize()
			// Rerun with MPT-backed storage.
			return s.runScriptInVM(t, script, contractScriptHash, tx, b, &ic.Block.Index, verbose)
		}
		id = uuid.New()
		sessionID := id.String()
		sess.finalize = ic.Finalize
		sess.timer = time.AfterFunc(s.config.SessionLifetime, func() {
			s.sessionsLock.Lock()
			defer s.sessionsLock.Unlock()
			if len(s.sessions) == 0 {
				return
			}
			sess, ok := s.sessions[sessionID]
			if !ok {
				return
			}
			sess.iteratorsLock.Lock()
			sess.finalize()
			delete(s.sessions, sessionID)
			sess.iteratorsLock.Unlock()
		})
		s.sessionsLock.Lock()
		if len(s.sessions) >= s.config.SessionPoolSize {
			ic.Finalize()
			s.sessionsLock.Unlock()
			return nil, neorpc.NewInternalServerError("max session capacity reached")
		}
		s.sessions[sessionID] = sess
		s.sessionsLock.Unlock()
	} else {
		ic.Finalize()
	}
	var diag *result.InvokeDiag
	tree := ic.VM.GetInvocationTree()
	if tree != nil {
		diag = &result.InvokeDiag{
			Invocations: tree.Calls,
			Changes:     storage.BatchToOperations(ic.DAO.GetBatch()),
		}
	}
	notifications := ic.Notifications
	if notifications == nil {
		notifications = make([]state.NotificationEvent, 0)
	}
	res := &result.Invoke{
		State:          ic.VM.State().String(),
		GasConsumed:    ic.VM.GasConsumed(),
		Script:         script,
		Stack:          items,
		FaultException: faultException,
		Notifications:  notifications,
		Diagnostics:    diag,
		Session:        id,
	}

	return res, nil
}

// postProcessExecStack changes iterator interop items according to the server configuration.
// It does modifications in-place, but it returns a session if any iterator was registered.
func (s *Server) postProcessExecStack(stack []stackitem.Item) *session {
	var sess session

	for i, v := range stack {
		var (
			id   uuid.UUID
			curr stackitem.Item
		)

		stack[i], id, curr = s.registerOrDumpIterator(v)
		if id != (uuid.UUID{}) {
			sess.iteratorIdentifiers = append(sess.iteratorIdentifiers, &iteratorIdentifier{
				ID:   id.String(),
				Item: v,
				Curr: curr,
			})
		}
	}
	if len(sess.iteratorIdentifiers) != 0 {
		return &sess
	}
	return nil
}

// registerOrDumpIterator changes iterator interop stack items into result.Iterator
// interop stack items and returns a uuid for it if sessions are enabled. All the other stack
// items are not changed. The third return value is the current iterator value if it's not nil.
func (s *Server) registerOrDumpIterator(item stackitem.Item) (stackitem.Item, uuid.UUID, stackitem.Item) {
	var iterID uuid.UUID

	if (item.Type() != stackitem.InteropT) || !iterator.IsIterator(item) {
		return item, iterID, nil
	}

	var (
		resIterator result.Iterator
		curr        stackitem.Item
	)

	if s.config.SessionEnabled {
		iterID = uuid.New()
		resIterator.ID = &iterID
		if s.config.SessionExpansionEnabled {
			resIterator.Values, resIterator.Truncated, curr = iterator.ValuesTruncated(item, s.config.MaxIteratorResultItems)
		}
	} else {
		resIterator.Values, resIterator.Truncated, curr = iterator.ValuesTruncated(item, s.config.MaxIteratorResultItems)
	}
	return stackitem.NewInterop(resIterator), iterID, curr
}

func (s *Server) traverseIterator(reqParams params.Params) (any, *neorpc.Error) {
	if !s.config.SessionEnabled {
		return nil, neorpc.ErrSessionsDisabled
	}
	sID, err := reqParams.Value(0).GetUUID()
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("invalid session ID: %s", err))
	}
	iID, err := reqParams.Value(1).GetUUID()
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("invalid iterator ID: %s", err))
	}
	count, err := reqParams.Value(2).GetInt()
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("invalid iterator items count: %s", err))
	}
	if err := checkInt32(count); err != nil {
		return nil, neorpc.NewInvalidParamsError("invalid iterator items count: not an int32")
	}
	if count > s.config.MaxIteratorResultItems {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("iterator items count (%d) is out of range (%d at max)", count, s.config.MaxIteratorResultItems))
	}

	s.sessionsLock.Lock()
	session, ok := s.sessions[sID.String()]
	if !ok {
		s.sessionsLock.Unlock()
		return nil, neorpc.ErrUnknownSession
	}
	session.iteratorsLock.Lock()
	// Perform `till` update only after session.iteratorsLock is taken in order to have more
	// precise session lifetime.
	session.timer.Reset(s.config.SessionLifetime)
	s.sessionsLock.Unlock()

	var (
		iIDStr = iID.String()
		iVals  []stackitem.Item
		found  bool
	)
	for _, it := range session.iteratorIdentifiers {
		if iIDStr == it.ID {
			found = true
			if it.Curr != nil {
				if count <= 0 {
					break
				}
				iVals = append(iVals, it.Curr)
				it.Curr = nil
				count--
			}
			iVals = append(iVals, iterator.Values(it.Item, count)...)
			break
		}
	}
	session.iteratorsLock.Unlock()
	if !found {
		return nil, neorpc.ErrUnknownIterator
	}

	result := make([]json.RawMessage, len(iVals))
	for j := range iVals {
		result[j], err = stackitem.ToJSONWithTypes(iVals[j])
		if err != nil {
			return nil, neorpc.NewInternalServerError(fmt.Sprintf("failed to marshal iterator value: %s", err))
		}
	}
	return result, nil
}

func (s *Server) terminateSession(reqParams params.Params) (any, *neorpc.Error) {
	if !s.config.SessionEnabled {
		return nil, neorpc.ErrSessionsDisabled
	}
	sID, err := reqParams.Value(0).GetUUID()
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("invalid session ID: %s", err))
	}
	strSID := sID.String()
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()
	session, ok := s.sessions[strSID]
	if !ok {
		return nil, neorpc.ErrUnknownSession
	}
	// Iterators access Seek channel under the hood; finalizer closes this channel, thus,
	// we need to perform finalisation under iteratorsLock.
	session.iteratorsLock.Lock()
	session.timer.Stop() // cancel session's finalizer.
	session.finalize()
	delete(s.sessions, strSID)
	session.iteratorsLock.Unlock()
	return ok, nil
}

// submitBlock broadcasts a raw block over the Neo network.
func (s *Server) submitBlock(reqParams params.Params) (any, *neorpc.Error) {
	blockBytes, err := reqParams.Value(0).GetBytesBase64()
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("missing parameter or not a base64: %s", err))
	}
	b := block.New(s.stateRootEnabled)
	r := io.NewBinReaderFromBuf(blockBytes)
	b.DecodeBinary(r)
	if r.Err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("can't decode block: %s", r.Err))
	}
	return getRelayResult(s.chain.AddBlock(b), b.Hash())
}

// submitNotaryRequest broadcasts P2PNotaryRequest over the Neo network.
func (s *Server) submitNotaryRequest(ps params.Params) (any, *neorpc.Error) {
	if !s.chain.P2PSigExtensionsEnabled() {
		return nil, neorpc.NewInternalServerError("P2PSignatureExtensions are disabled")
	}

	bytePayload, err := ps.Value(0).GetBytesBase64()
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("not a base64: %s", err))
	}
	r, err := payload.NewP2PNotaryRequestFromBytes(bytePayload)
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("can't decode notary payload: %s", err))
	}
	return getRelayResult(s.coreServer.RelayP2PNotaryRequest(r), r.FallbackTransaction.Hash())
}

// getRelayResult returns successful relay result or an error.
func getRelayResult(err error, hash util.Uint256) (any, *neorpc.Error) {
	switch {
	case err == nil:
		return result.RelayResult{
			Hash: hash,
		}, nil
	case errors.Is(err, core.ErrTxExpired):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrExpiredTransaction, err.Error())
	case errors.Is(err, core.ErrAlreadyExists):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrAlreadyExists, err.Error())
	case errors.Is(err, core.ErrAlreadyInPool):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrAlreadyInPool, err.Error())
	case errors.Is(err, core.ErrOOM):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrMempoolCapReached, err.Error())
	case errors.Is(err, core.ErrPolicy):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrPolicyFailed, err.Error())
	case errors.Is(err, core.ErrInvalidScript):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidScript, err.Error())
	case errors.Is(err, core.ErrTxTooBig):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidSize, err.Error())
	case errors.Is(err, core.ErrTxSmallNetworkFee):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInsufficientNetworkFee, err.Error())
	case errors.Is(err, core.ErrInvalidAttribute):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidAttribute, err.Error())
	case errors.Is(err, core.ErrInsufficientFunds), errors.Is(err, core.ErrMemPoolConflict):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInsufficientFunds, err.Error())
	case errors.Is(err, core.ErrInvalidSignature):
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidSignature, err.Error())
	default:
		return nil, neorpc.WrapErrorWithData(neorpc.ErrVerificationFailed, err.Error())
	}
}

func (s *Server) submitOracleResponse(ps params.Params) (any, *neorpc.Error) {
	oraclePtr := s.oracle.Load()
	if oraclePtr == nil {
		return nil, neorpc.ErrOracleDisabled
	}
	oracle := oraclePtr.(OracleHandler)
	var pub *keys.PublicKey
	pubBytes, err := ps.Value(0).GetBytesBase64()
	if err == nil {
		pub, err = keys.NewPublicKeyFromBytes(pubBytes, elliptic.P256())
	}
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("public key is missing: %s", err))
	}
	reqID, err := ps.Value(1).GetInt()
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("request ID is missing: %s", err))
	}
	txSig, err := ps.Value(2).GetBytesBase64()
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("tx signature is missing: %s", err))
	}
	msgSig, err := ps.Value(3).GetBytesBase64()
	if err != nil {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("msg signature is missing: %s", err))
	}
	data := broadcaster.GetMessage(pubBytes, uint64(reqID), txSig)
	if !pub.Verify(msgSig, hash.Sha256(data).BytesBE()) {
		return nil, neorpc.ErrInvalidSignature
	}
	oracle.AddResponse(pub, uint64(reqID), txSig)
	return json.RawMessage([]byte("{}")), nil
}

func (s *Server) sendrawtransaction(reqParams params.Params) (any, *neorpc.Error) {
	if len(reqParams) < 1 {
		return nil, neorpc.NewInvalidParamsError("not enough parameters")
	}
	byteTx, err := reqParams[0].GetBytesBase64()
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("not a base64: %s", err))
	}
	tx, err := transaction.NewTransactionFromBytes(byteTx)
	if err != nil {
		return nil, neorpc.NewInvalidParamsError(fmt.Sprintf("can't decode transaction: %s", err))
	}
	return getRelayResult(s.coreServer.RelayTxn(tx), tx.Hash())
}

// subscribe handles subscription requests from websocket clients.
func (s *Server) subscribe(reqParams params.Params, sub *subscriber) (any, *neorpc.Error) {
	streamName, err := reqParams.Value(0).GetString()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	event, err := neorpc.GetEventIDFromString(streamName)
	if err != nil || event == neorpc.MissedEventID {
		return nil, neorpc.ErrInvalidParams
	}
	if event == neorpc.NotaryRequestEventID && !s.chain.P2PSigExtensionsEnabled() {
		return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, "P2PSigExtensions are disabled")
	}
	// Optional filter.
	var filter neorpc.SubscriptionFilter
	if p := reqParams.Value(1); p != nil {
		param := *p
		jd := json.NewDecoder(bytes.NewReader(param.RawMessage))
		jd.DisallowUnknownFields()
		switch event {
		case neorpc.BlockEventID, neorpc.HeaderOfAddedBlockEventID:
			flt := new(neorpc.BlockFilter)
			err = jd.Decode(flt)
			filter = *flt
		case neorpc.TransactionEventID:
			flt := new(neorpc.TxFilter)
			err = jd.Decode(flt)
			filter = *flt
		case neorpc.NotaryRequestEventID:
			flt := new(neorpc.NotaryRequestFilter)
			err = jd.Decode(flt)
			filter = *flt
		case neorpc.NotificationEventID:
			flt := new(neorpc.NotificationFilter)
			err = jd.Decode(flt)
			filter = *flt
		case neorpc.ExecutionEventID:
			flt := new(neorpc.ExecutionFilter)
			err = jd.Decode(flt)
			filter = *flt
		default:
		}
		if err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
		}
	}
	if filter != nil {
		err = filter.IsValid()
		if err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
		}
	}

	s.subsLock.Lock()
	var id int
	for ; id < len(sub.feeds); id++ {
		if sub.feeds[id].event == neorpc.InvalidEventID {
			break
		}
	}
	if id == len(sub.feeds) {
		s.subsLock.Unlock()
		return nil, neorpc.NewInternalServerError("maximum number of subscriptions is reached")
	}
	sub.feeds[id].event = event
	sub.feeds[id].filter = filter
	s.subsLock.Unlock()

	s.subsCounterLock.Lock()
	select {
	case <-s.shutdown:
		s.subsCounterLock.Unlock()
		return nil, neorpc.NewInternalServerError("server is shutting down")
	default:
	}
	s.subscribeToChannel(event)
	s.subsCounterLock.Unlock()
	return strconv.FormatInt(int64(id), 10), nil
}

// subscribeToChannel subscribes RPC server to appropriate chain events if
// it's not yet subscribed for them. It's supposed to be called with s.subsCounterLock
// taken by the caller.
func (s *Server) subscribeToChannel(event neorpc.EventID) {
	switch event {
	case neorpc.BlockEventID:
		if s.blockSubs == 0 {
			s.chain.SubscribeForBlocks(s.blockCh)
		}
		s.blockSubs++
	case neorpc.TransactionEventID:
		if s.transactionSubs == 0 {
			s.chain.SubscribeForTransactions(s.transactionCh)
		}
		s.transactionSubs++
	case neorpc.NotificationEventID:
		if s.notificationSubs == 0 {
			s.chain.SubscribeForNotifications(s.notificationCh)
		}
		s.notificationSubs++
	case neorpc.ExecutionEventID:
		if s.executionSubs == 0 {
			s.chain.SubscribeForExecutions(s.executionCh)
		}
		s.executionSubs++
	case neorpc.NotaryRequestEventID:
		if s.notaryRequestSubs == 0 {
			s.coreServer.SubscribeForNotaryRequests(s.notaryRequestCh)
		}
		s.notaryRequestSubs++
	case neorpc.HeaderOfAddedBlockEventID:
		if s.blockHeaderSubs == 0 {
			s.chain.SubscribeForHeadersOfAddedBlocks(s.blockHeaderCh)
		}
		s.blockHeaderSubs++
	default:
	}
}

// unsubscribe handles unsubscription requests from websocket clients.
func (s *Server) unsubscribe(reqParams params.Params, sub *subscriber) (any, *neorpc.Error) {
	id, err := reqParams.Value(0).GetInt()
	if err != nil || id < 0 {
		return nil, neorpc.ErrInvalidParams
	}
	s.subsLock.Lock()
	if len(sub.feeds) <= id || sub.feeds[id].event == neorpc.InvalidEventID {
		s.subsLock.Unlock()
		return nil, neorpc.ErrInvalidParams
	}
	event := sub.feeds[id].event
	sub.feeds[id].event = neorpc.InvalidEventID
	sub.feeds[id].filter = nil
	s.subsLock.Unlock()

	s.subsCounterLock.Lock()
	s.unsubscribeFromChannel(event)
	s.subsCounterLock.Unlock()
	return true, nil
}

// unsubscribeFromChannel unsubscribes RPC server from appropriate chain events
// if there are no other subscribers for it. It must be called with s.subsConutersLock
// holding by the caller.
func (s *Server) unsubscribeFromChannel(event neorpc.EventID) {
	switch event {
	case neorpc.BlockEventID:
		s.blockSubs--
		if s.blockSubs == 0 {
			s.chain.UnsubscribeFromBlocks(s.blockCh)
		}
	case neorpc.TransactionEventID:
		s.transactionSubs--
		if s.transactionSubs == 0 {
			s.chain.UnsubscribeFromTransactions(s.transactionCh)
		}
	case neorpc.NotificationEventID:
		s.notificationSubs--
		if s.notificationSubs == 0 {
			s.chain.UnsubscribeFromNotifications(s.notificationCh)
		}
	case neorpc.ExecutionEventID:
		s.executionSubs--
		if s.executionSubs == 0 {
			s.chain.UnsubscribeFromExecutions(s.executionCh)
		}
	case neorpc.NotaryRequestEventID:
		s.notaryRequestSubs--
		if s.notaryRequestSubs == 0 {
			s.coreServer.UnsubscribeFromNotaryRequests(s.notaryRequestCh)
		}
	case neorpc.HeaderOfAddedBlockEventID:
		s.blockHeaderSubs--
		if s.blockHeaderSubs == 0 {
			s.chain.UnsubscribeFromHeadersOfAddedBlocks(s.blockHeaderCh)
		}
	default:
	}
}

// handleSubEvents processes Server subscriptions until Shutdown. Upon
// completion signals to subEventCh channel.
func (s *Server) handleSubEvents() {
	var overflowEvent = neorpc.Notification{
		JSONRPC: neorpc.JSONRPCVersion,
		Event:   neorpc.MissedEventID,
		Payload: make([]any, 0),
	}
	b, err := json.Marshal(overflowEvent)
	if err != nil {
		s.log.Error("fatal: failed to marshal overflow event", zap.Error(err))
		return
	}
	overflowMsg, err := websocket.NewPreparedMessage(websocket.TextMessage, b)
	if err != nil {
		s.log.Error("fatal: failed to prepare overflow message", zap.Error(err))
		return
	}
chloop:
	for {
		var resp = neorpc.Notification{
			JSONRPC: neorpc.JSONRPCVersion,
			Payload: make([]any, 1),
		}
		var msg *websocket.PreparedMessage
		select {
		case <-s.shutdown:
			break chloop
		case b := <-s.blockCh:
			resp.Event = neorpc.BlockEventID
			resp.Payload[0] = b
		case execution := <-s.executionCh:
			resp.Event = neorpc.ExecutionEventID
			resp.Payload[0] = execution
		case notification := <-s.notificationCh:
			resp.Event = neorpc.NotificationEventID
			resp.Payload[0] = notification
		case tx := <-s.transactionCh:
			resp.Event = neorpc.TransactionEventID
			resp.Payload[0] = tx
		case e := <-s.notaryRequestCh:
			resp.Event = neorpc.NotaryRequestEventID
			resp.Payload[0] = &result.NotaryRequestEvent{
				Type:          e.Type,
				NotaryRequest: e.Data.(*payload.P2PNotaryRequest),
			}
		case header := <-s.blockHeaderCh:
			resp.Event = neorpc.HeaderOfAddedBlockEventID
			resp.Payload[0] = header
		}
		s.subsLock.RLock()
	subloop:
		for sub := range s.subscribers {
			if sub.overflown.Load() {
				continue
			}
			for i := range sub.feeds {
				if rpcevent.Matches(sub.feeds[i], &resp) {
					if msg == nil {
						b, err = json.Marshal(resp)
						if err != nil {
							s.log.Error("failed to marshal notification",
								zap.Error(err),
								zap.Stringer("type", resp.Event))
							break subloop
						}
						msg, err = websocket.NewPreparedMessage(websocket.TextMessage, b)
						if err != nil {
							s.log.Error("failed to prepare notification message",
								zap.Error(err),
								zap.Stringer("type", resp.Event))
							break subloop
						}
					}
					select {
					case sub.writer <- intEvent{msg, &resp}:
					default:
						sub.overflown.Store(true)
						// MissedEvent is to be delivered eventually.
						go func(sub *subscriber) {
							sub.writer <- intEvent{overflowMsg, &overflowEvent}
							sub.overflown.Store(false)
						}(sub)
					}
					// The message is sent only once per subscriber.
					break
				}
			}
		}
		s.subsLock.RUnlock()
	}
	// It's important to do it with subsCounterLock held because no subscription routine
	// should be running concurrently to this one. And even if one is to run
	// after unlock, it'll see closed s.shutdown and won't subscribe.
	s.subsCounterLock.Lock()
	// There might be no subscription in reality, but it's not a problem as
	// core.Blockchain allows unsubscribing non-subscribed channels.
	s.chain.UnsubscribeFromBlocks(s.blockCh)
	s.chain.UnsubscribeFromTransactions(s.transactionCh)
	s.chain.UnsubscribeFromNotifications(s.notificationCh)
	s.chain.UnsubscribeFromExecutions(s.executionCh)
	s.chain.UnsubscribeFromHeadersOfAddedBlocks(s.blockHeaderCh)
	if s.chain.P2PSigExtensionsEnabled() {
		s.coreServer.UnsubscribeFromNotaryRequests(s.notaryRequestCh)
	}
	s.subsCounterLock.Unlock()
drainloop:
	for {
		select {
		case <-s.blockCh:
		case <-s.executionCh:
		case <-s.notificationCh:
		case <-s.transactionCh:
		case <-s.notaryRequestCh:
		case <-s.blockHeaderCh:
		default:
			break drainloop
		}
	}
	// It's not required closing these, but since they're drained already
	// this is safe.
	close(s.blockCh)
	close(s.transactionCh)
	close(s.notificationCh)
	close(s.executionCh)
	close(s.notaryRequestCh)
	close(s.blockHeaderCh)
	// notify Shutdown routine
	close(s.subEventsToExitCh)
}

func (s *Server) blockHeightFromParam(param *params.Param) (uint32, *neorpc.Error) {
	num, err := param.GetInt()
	if err != nil {
		return 0, neorpc.ErrInvalidParams
	}

	if num < 0 || int64(num) > int64(s.chain.BlockHeight()) {
		return 0, neorpc.WrapErrorWithData(neorpc.ErrUnknownHeight, fmt.Sprintf("param at index %d should be greater than or equal to 0 and less than or equal to current block height, got: %d", 0, num))
	}
	return uint32(num), nil
}

func (s *Server) packResponse(r *params.In, result any, respErr *neorpc.Error) abstract {
	resp := abstract{
		Header: neorpc.Header{
			JSONRPC: r.JSONRPC,
			ID:      r.RawID,
		},
	}
	if respErr != nil {
		resp.Error = respErr
	} else {
		resp.Result = result
	}
	return resp
}

// logRequestError is a request error logger.
func (s *Server) logRequestError(r *params.Request, jsonErr *neorpc.Error) {
	logFields := []zap.Field{
		zap.Int64("code", jsonErr.Code),
	}
	if len(jsonErr.Data) != 0 {
		logFields = append(logFields, zap.String("cause", jsonErr.Data))
	}

	if r.In != nil {
		logFields = append(logFields, zap.String("method", r.In.Method))
		params := params.Params(r.In.RawParams)
		logFields = append(logFields, zap.Any("params", params))
	}

	logText := "Error encountered with rpc request"
	switch jsonErr.Code {
	case neorpc.InternalServerErrorCode:
		s.log.Error(logText, logFields...)
	default:
		s.log.Info(logText, logFields...)
	}
}

// writeHTTPErrorResponse writes an error response to the ResponseWriter.
func (s *Server) writeHTTPErrorResponse(r *params.In, w http.ResponseWriter, jsonErr *neorpc.Error) {
	resp := s.packResponse(r, nil, jsonErr)
	s.writeHTTPServerResponse(&params.Request{In: r}, w, resp)
}

func setCORSOriginHeaders(h http.Header) {
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
}

func (s *Server) writeHTTPServerResponse(r *params.Request, w http.ResponseWriter, resp abstractResult) {
	// Errors can happen in many places and we can only catch ALL of them here.
	resp.RunForErrors(func(jsonErr *neorpc.Error) {
		s.logRequestError(r, jsonErr)
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if s.config.EnableCORSWorkaround {
		setCORSOriginHeaders(w.Header())
	}

	encoder := json.NewEncoder(w)
	err := encoder.Encode(resp)

	if err != nil {
		switch {
		case r.In != nil:
			s.log.Error("Error encountered while encoding response",
				zap.String("err", err.Error()),
				zap.String("method", r.In.Method))
		case r.Batch != nil:
			s.log.Error("Error encountered while encoding batch response",
				zap.String("err", err.Error()))
		}
	}
}

// validateAddress verifies that the address is a correct Neo address
// see https://docs.neo.org/en-us/node/cli/2.9.4/api/validateaddress.html
func validateAddress(addr any) bool {
	if addr, ok := addr.(string); ok {
		_, err := address.StringToUint160(addr)
		return err == nil
	}
	return false
}

func escapeForLog(in string) string {
	return strings.Map(func(c rune) rune {
		if !strconv.IsGraphic(c) {
			return -1
		}
		return c
	}, in)
}

// Addresses returns the list of addresses RPC server is listening to in the form of
// address:port.
func (s *Server) Addresses() []string {
	res := make([]string, len(s.http))
	for i, srv := range s.http {
		res[i] = srv.Addr
	}
	return res
}

func (s *Server) getRawNotaryPool(_ params.Params) (any, *neorpc.Error) {
	if !s.chain.P2PSigExtensionsEnabled() {
		return nil, neorpc.NewInternalServerError("P2PSignatureExtensions are disabled")
	}
	nrp := s.coreServer.GetNotaryPool()
	res := &result.RawNotaryPool{Hashes: make(map[util.Uint256][]util.Uint256)}
	nrp.IterateVerifiedTransactions(func(tx *transaction.Transaction, data any) bool {
		if data != nil {
			d := data.(*payload.P2PNotaryRequest)
			mainHash := d.MainTransaction.Hash()
			fallbackHash := d.FallbackTransaction.Hash()
			res.Hashes[mainHash] = append(res.Hashes[mainHash], fallbackHash)
		}
		return true
	})
	return res, nil
}

func (s *Server) getRawNotaryTransaction(reqParams params.Params) (any, *neorpc.Error) {
	if !s.chain.P2PSigExtensionsEnabled() {
		return nil, neorpc.NewInternalServerError("P2PSignatureExtensions are disabled")
	}

	txHash, err := reqParams.Value(0).GetUint256()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	nrp := s.coreServer.GetNotaryPool()
	// Try to find fallback transaction.
	tx, ok := nrp.TryGetValue(txHash)
	if !ok {
		// Try to find main transaction.
		nrp.IterateVerifiedTransactions(func(t *transaction.Transaction, data any) bool {
			if data != nil && data.(*payload.P2PNotaryRequest).MainTransaction.Hash().Equals(txHash) {
				tx = data.(*payload.P2PNotaryRequest).MainTransaction
				return false
			}
			return true
		})
		// The transaction was not found.
		if tx == nil {
			return nil, neorpc.ErrUnknownTransaction
		}
	}

	if v, _ := reqParams.Value(1).GetBoolean(); v {
		return tx, nil
	}
	return tx.Bytes(), nil
}

// getBlockNotifications returns notifications from a specific block with optional filtering.
func (s *Server) getBlockNotifications(reqParams params.Params) (any, *neorpc.Error) {
	param := reqParams.Value(0)
	hash, respErr := s.blockHashFromParam(param)
	if respErr != nil {
		return nil, respErr
	}

	var filter *neorpc.NotificationFilter
	if len(reqParams) > 1 {
		var (
			reader  = bytes.NewBuffer([]byte(reqParams[1].RawMessage))
			decoder = json.NewDecoder(reader)
		)
		decoder.DisallowUnknownFields()
		filter = new(neorpc.NotificationFilter)

		err := decoder.Decode(filter)
		if err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid filter: %s", err))
		}
		if err := filter.IsValid(); err != nil {
			return nil, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, fmt.Sprintf("invalid filter: %s", err))
		}
	}

	block, err := s.chain.GetTrimmedBlock(hash)
	if err != nil {
		return nil, neorpc.ErrUnknownBlock
	}

	notifications := &result.BlockNotifications{}

	aers, err := s.chain.GetAppExecResults(block.Hash(), trigger.OnPersist)
	if err != nil {
		return nil, neorpc.NewInternalServerError("failed to get app exec results for onpersist")
	}
	notifications.OnPersist = processAppExecResults([]state.AppExecResult{aers[0]}, filter)

	for _, txHash := range block.Transactions {
		aers, err := s.chain.GetAppExecResults(txHash.Hash(), trigger.Application)
		if err != nil {
			return nil, neorpc.NewInternalServerError("failed to get app exec results")
		}
		notifications.Application = append(notifications.Application, processAppExecResults(aers, filter)...)
	}

	aers, err = s.chain.GetAppExecResults(block.Hash(), trigger.PostPersist)
	if err != nil {
		return nil, neorpc.NewInternalServerError("failed to get app exec results for postpersist")
	}
	notifications.PostPersist = processAppExecResults([]state.AppExecResult{aers[0]}, filter)

	return notifications, nil
}
