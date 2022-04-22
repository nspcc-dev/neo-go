package server

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
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
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
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/rpc"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result/subscriptions"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle/broadcaster"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		*http.Server
		chain            blockchainer.Blockchainer
		config           rpc.Config
		network          netmode.Magic
		stateRootEnabled bool
		coreServer       *network.Server
		oracle           *oracle.Oracle
		log              *zap.Logger
		https            *http.Server
		shutdown         chan struct{}
		started          *atomic.Bool
		errChan          chan error

		subsLock          sync.RWMutex
		subscribers       map[*subscriber]bool
		blockSubs         int
		executionSubs     int
		notificationSubs  int
		transactionSubs   int
		notaryRequestSubs int
		blockCh           chan *block.Block
		executionCh       chan *state.AppExecResult
		notificationCh    chan *subscriptions.NotificationEvent
		transactionCh     chan *transaction.Transaction
		notaryRequestCh   chan mempoolevent.Event
	}
)

const (
	// Message limit for receiving side.
	wsReadLimit = 4096

	// Disconnection timeout.
	wsPongLimit = 60 * time.Second

	// Ping period for connection liveness check.
	wsPingPeriod = wsPongLimit / 2

	// Write deadline.
	wsWriteLimit = wsPingPeriod / 2

	// Maximum number of subscribers per Server. Each websocket client is
	// treated like subscriber, so technically it's a limit on websocket
	// connections.
	maxSubscribers = 64

	// Maximum number of elements for get*transfers requests.
	maxTransfersLimit = 1000
)

var rpcHandlers = map[string]func(*Server, request.Params) (interface{}, *response.Error){
	"calculatenetworkfee":    (*Server).calculateNetworkFee,
	"findstates":             (*Server).findStates,
	"getapplicationlog":      (*Server).getApplicationLog,
	"getbestblockhash":       (*Server).getBestBlockHash,
	"getblock":               (*Server).getBlock,
	"getblockcount":          (*Server).getBlockCount,
	"getblockhash":           (*Server).getBlockHash,
	"getblockheader":         (*Server).getBlockHeader,
	"getblockheadercount":    (*Server).getBlockHeaderCount,
	"getblocksysfee":         (*Server).getBlockSysFee,
	"getcommittee":           (*Server).getCommittee,
	"getconnectioncount":     (*Server).getConnectionCount,
	"getcontractstate":       (*Server).getContractState,
	"getnativecontracts":     (*Server).getNativeContracts,
	"getnep11balances":       (*Server).getNEP11Balances,
	"getnep11properties":     (*Server).getNEP11Properties,
	"getnep11transfers":      (*Server).getNEP11Transfers,
	"getnep17balances":       (*Server).getNEP17Balances,
	"getnep17transfers":      (*Server).getNEP17Transfers,
	"getpeers":               (*Server).getPeers,
	"getproof":               (*Server).getProof,
	"getrawmempool":          (*Server).getRawMempool,
	"getrawtransaction":      (*Server).getrawtransaction,
	"getstate":               (*Server).getState,
	"getstateheight":         (*Server).getStateHeight,
	"getstateroot":           (*Server).getStateRoot,
	"getstorage":             (*Server).getStorage,
	"gettransactionheight":   (*Server).getTransactionHeight,
	"getunclaimedgas":        (*Server).getUnclaimedGas,
	"getnextblockvalidators": (*Server).getNextBlockValidators,
	"getversion":             (*Server).getVersion,
	"invokefunction":         (*Server).invokeFunction,
	"invokescript":           (*Server).invokescript,
	"invokecontractverify":   (*Server).invokeContractVerify,
	"sendrawtransaction":     (*Server).sendrawtransaction,
	"submitblock":            (*Server).submitBlock,
	"submitnotaryrequest":    (*Server).submitNotaryRequest,
	"submitoracleresponse":   (*Server).submitOracleResponse,
	"validateaddress":        (*Server).validateAddress,
	"verifyproof":            (*Server).verifyProof,
}

var rpcWsHandlers = map[string]func(*Server, request.Params, *subscriber) (interface{}, *response.Error){
	"subscribe":   (*Server).subscribe,
	"unsubscribe": (*Server).unsubscribe,
}

var invalidBlockHeightError = func(index int, height int) *response.Error {
	return response.NewRPCError(fmt.Sprintf("Param at index %d should be greater than or equal to 0 and less then or equal to current block height, got: %d", index, height), "", nil)
}

// upgrader is a no-op websocket.Upgrader that reuses HTTP server buffers and
// doesn't set any Error function.
var upgrader = websocket.Upgrader{}

// New creates a new Server struct.
func New(chain blockchainer.Blockchainer, conf rpc.Config, coreServer *network.Server,
	orc *oracle.Oracle, log *zap.Logger, errChan chan error) Server {
	httpServer := &http.Server{
		Addr: conf.Address + ":" + strconv.FormatUint(uint64(conf.Port), 10),
	}

	var tlsServer *http.Server
	if cfg := conf.TLSConfig; cfg.Enabled {
		tlsServer = &http.Server{
			Addr: net.JoinHostPort(cfg.Address, strconv.FormatUint(uint64(cfg.Port), 10)),
		}
	}

	if orc != nil {
		orc.SetBroadcaster(broadcaster.New(orc.MainCfg, log))
	}
	return Server{
		Server:           httpServer,
		chain:            chain,
		config:           conf,
		network:          chain.GetConfig().Magic,
		stateRootEnabled: chain.GetConfig().StateRootInHeader,
		coreServer:       coreServer,
		log:              log,
		oracle:           orc,
		https:            tlsServer,
		shutdown:         make(chan struct{}),
		started:          atomic.NewBool(false),
		errChan:          errChan,

		subscribers: make(map[*subscriber]bool),
		// These are NOT buffered to preserve original order of events.
		blockCh:         make(chan *block.Block),
		executionCh:     make(chan *state.AppExecResult),
		notificationCh:  make(chan *subscriptions.NotificationEvent),
		transactionCh:   make(chan *transaction.Transaction),
		notaryRequestCh: make(chan mempoolevent.Event),
	}
}

// Name returns service name.
func (s *Server) Name() string {
	return "rpc"
}

// Start creates a new JSON-RPC server listening on the configured port. It creates
// goroutines needed internally and it returns its errors via errChan passed to New().
func (s *Server) Start() {
	if !s.config.Enabled {
		s.log.Info("RPC server is not enabled")
		return
	}
	if !s.started.CAS(false, true) {
		s.log.Info("RPC server already started")
		return
	}
	s.Handler = http.HandlerFunc(s.handleHTTPRequest)
	s.log.Info("starting rpc-server", zap.String("endpoint", s.Addr))

	go s.handleSubEvents()
	if cfg := s.config.TLSConfig; cfg.Enabled {
		s.https.Handler = http.HandlerFunc(s.handleHTTPRequest)
		s.log.Info("starting rpc-server (https)", zap.String("endpoint", s.https.Addr))
		go func() {
			ln, err := net.Listen("tcp", s.https.Addr)
			if err != nil {
				s.errChan <- err
				return
			}
			s.https.Addr = ln.Addr().String()
			err = s.https.ServeTLS(ln, cfg.CertFile, cfg.KeyFile)
			if err != http.ErrServerClosed {
				s.log.Error("failed to start TLS RPC server", zap.Error(err))
				s.errChan <- err
			}
		}()
	}
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		s.errChan <- err
		return
	}
	s.Addr = ln.Addr().String() // set Addr to the actual address
	go func() {
		err = s.Serve(ln)
		if err != http.ErrServerClosed {
			s.log.Error("failed to start RPC server", zap.Error(err))
			s.errChan <- err
		}
	}()
}

// Shutdown stops the RPC server. It can only be called once.
func (s *Server) Shutdown() {
	var httpsErr error

	// Signal to websocket writer routines and handleSubEvents.
	close(s.shutdown)

	if s.config.TLSConfig.Enabled {
		s.log.Info("shutting down RPC server (https)", zap.String("endpoint", s.https.Addr))
		httpsErr = s.https.Shutdown(context.Background())
	}

	s.log.Info("shutting down RPC server", zap.String("endpoint", s.Addr))
	err := s.Server.Shutdown(context.Background())

	// Wait for handleSubEvents to finish.
	<-s.executionCh

	if httpsErr != nil {
		s.errChan <- httpsErr
	}
	if err != nil {
		s.errChan <- err
	}
}

func (s *Server) handleHTTPRequest(w http.ResponseWriter, httpRequest *http.Request) {
	req := request.NewRequest()

	if httpRequest.URL.Path == "/ws" && httpRequest.Method == "GET" {
		// Technically there is a race between this check and
		// s.subscribers modification 20 lines below, but it's tiny
		// and not really critical to bother with it. Some additional
		// clients may sneak in, no big deal.
		s.subsLock.RLock()
		numOfSubs := len(s.subscribers)
		s.subsLock.RUnlock()
		if numOfSubs >= maxSubscribers {
			s.writeHTTPErrorResponse(
				request.NewIn(),
				w,
				response.NewInternalServerError("websocket users limit reached", nil),
			)
			return
		}
		ws, err := upgrader.Upgrade(w, httpRequest, nil)
		if err != nil {
			s.log.Info("websocket connection upgrade failed", zap.Error(err))
			return
		}
		resChan := make(chan response.AbstractResult) // response.Abstract or response.AbstractBatch
		subChan := make(chan *websocket.PreparedMessage, notificationBufSize)
		subscr := &subscriber{writer: subChan, ws: ws}
		s.subsLock.Lock()
		s.subscribers[subscr] = true
		s.subsLock.Unlock()
		go s.handleWsWrites(ws, resChan, subChan)
		s.handleWsReads(ws, resChan, subscr)
		return
	}

	if httpRequest.Method != "POST" {
		s.writeHTTPErrorResponse(
			request.NewIn(),
			w,
			response.NewInvalidParamsError(
				fmt.Sprintf("Invalid method '%s', please retry with 'POST'", httpRequest.Method), nil,
			),
		)
		return
	}

	err := req.DecodeData(httpRequest.Body)
	if err != nil {
		s.writeHTTPErrorResponse(request.NewIn(), w, response.NewParseError("Problem parsing JSON-RPC request body", err))
		return
	}

	resp := s.handleRequest(req, nil)
	s.writeHTTPServerResponse(req, w, resp)
}

func (s *Server) handleRequest(req *request.Request, sub *subscriber) response.AbstractResult {
	if req.In != nil {
		req.In.Method = escapeForLog(req.In.Method) // No valid method name will be changed by it.
		return s.handleIn(req.In, sub)
	}
	resp := make(response.AbstractBatch, len(req.Batch))
	for i, in := range req.Batch {
		in.Method = escapeForLog(in.Method) // No valid method name will be changed by it.
		resp[i] = s.handleIn(&in, sub)
	}
	return resp
}

func (s *Server) handleIn(req *request.In, sub *subscriber) response.Abstract {
	var res interface{}
	var resErr *response.Error
	if req.JSONRPC != request.JSONRPCVersion {
		return s.packResponse(req, nil, response.NewInvalidParamsError("Problem parsing JSON", fmt.Errorf("invalid version, expected 2.0 got: '%s'", req.JSONRPC)))
	}

	reqParams := request.Params(req.RawParams)

	s.log.Debug("processing rpc request",
		zap.String("method", req.Method),
		zap.Stringer("params", reqParams))

	incCounter(req.Method)

	resErr = response.NewMethodNotFoundError(fmt.Sprintf("Method %q not supported", req.Method), nil)
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

func (s *Server) handleWsWrites(ws *websocket.Conn, resChan <-chan response.AbstractResult, subChan <-chan *websocket.PreparedMessage) {
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
			if err := ws.WritePreparedMessage(event); err != nil {
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
	pingTicker.Stop()
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

func (s *Server) handleWsReads(ws *websocket.Conn, resChan chan<- response.AbstractResult, subscr *subscriber) {
	ws.SetReadLimit(wsReadLimit)
	err := ws.SetReadDeadline(time.Now().Add(wsPongLimit))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(wsPongLimit)) })
requestloop:
	for err == nil {
		req := request.NewRequest()
		err := ws.ReadJSON(req)
		if err != nil {
			break
		}
		res := s.handleRequest(req, subscr)
		res.RunForErrors(func(jsonErr *response.Error) {
			s.logRequestError(req, jsonErr)
		})
		select {
		case <-s.shutdown:
			break requestloop
		case resChan <- res:
		}
	}
	s.subsLock.Lock()
	delete(s.subscribers, subscr)
	for _, e := range subscr.feeds {
		if e.event != response.InvalidEventID {
			s.unsubscribeFromChannel(e.event)
		}
	}
	s.subsLock.Unlock()
	close(resChan)
	ws.Close()
}

func (s *Server) getBestBlockHash(_ request.Params) (interface{}, *response.Error) {
	return "0x" + s.chain.CurrentBlockHash().StringLE(), nil
}

func (s *Server) getBlockCount(_ request.Params) (interface{}, *response.Error) {
	return s.chain.BlockHeight() + 1, nil
}

func (s *Server) getBlockHeaderCount(_ request.Params) (interface{}, *response.Error) {
	return s.chain.HeaderHeight() + 1, nil
}

func (s *Server) getConnectionCount(_ request.Params) (interface{}, *response.Error) {
	return s.coreServer.PeerCount(), nil
}

func (s *Server) blockHashFromParam(param *request.Param) (util.Uint256, *response.Error) {
	var (
		hash util.Uint256
		err  error
	)
	if param == nil {
		return hash, response.ErrInvalidParams
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

func (s *Server) getBlock(reqParams request.Params) (interface{}, *response.Error) {
	param := reqParams.Value(0)
	hash, respErr := s.blockHashFromParam(param)
	if respErr != nil {
		return nil, respErr
	}

	block, err := s.chain.GetBlock(hash)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
	}

	if v, _ := reqParams.Value(1).GetBoolean(); v {
		return result.NewBlock(block, s.chain), nil
	}
	writer := io.NewBufBinWriter()
	block.EncodeBinary(writer.BinWriter)
	return writer.Bytes(), nil
}

func (s *Server) getBlockHash(reqParams request.Params) (interface{}, *response.Error) {
	num, err := s.blockHeightFromParam(reqParams.Value(0))
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	return s.chain.GetHeaderHash(num), nil
}

func (s *Server) getVersion(_ request.Params) (interface{}, *response.Error) {
	port, err := s.coreServer.Port()
	if err != nil {
		return nil, response.NewInternalServerError("Cannot fetch tcp port", err)
	}

	cfg := s.chain.GetConfig()
	return result.Version{
		Magic:             s.network,
		TCPPort:           port,
		Nonce:             s.coreServer.ID(),
		UserAgent:         s.coreServer.UserAgent,
		StateRootInHeader: cfg.StateRootInHeader,
		Protocol: result.Protocol{
			AddressVersion:              address.NEO3Prefix,
			Network:                     cfg.Magic,
			MillisecondsPerBlock:        cfg.SecondsPerBlock * 1000,
			MaxTraceableBlocks:          cfg.MaxTraceableBlocks,
			MaxValidUntilBlockIncrement: cfg.MaxValidUntilBlockIncrement,
			MaxTransactionsPerBlock:     cfg.MaxTransactionsPerBlock,
			MemoryPoolMaxTransactions:   cfg.MemPoolSize,
			ValidatorsCount:             byte(cfg.GetNumOfCNs(s.chain.BlockHeight())),
			InitialGasDistribution:      int64(cfg.InitialGASSupply),
			StateRootInHeader:           cfg.StateRootInHeader,
		},
	}, nil
}

func (s *Server) getPeers(_ request.Params) (interface{}, *response.Error) {
	peers := result.NewGetPeers()
	peers.AddUnconnected(s.coreServer.UnconnectedPeers())
	peers.AddConnected(s.coreServer.ConnectedPeers())
	peers.AddBad(s.coreServer.BadPeers())
	return peers, nil
}

func (s *Server) getRawMempool(reqParams request.Params) (interface{}, *response.Error) {
	verbose, _ := reqParams.Value(0).GetBoolean()
	mp := s.chain.GetMemPool()
	hashList := make([]util.Uint256, 0)
	for _, item := range mp.GetVerifiedTransactions() {
		hashList = append(hashList, item.Hash())
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

func (s *Server) validateAddress(reqParams request.Params) (interface{}, *response.Error) {
	param, err := reqParams.Value(0).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	return result.ValidateAddress{
		Address: reqParams.Value(0),
		IsValid: validateAddress(param),
	}, nil
}

// calculateNetworkFee calculates network fee for the transaction.
func (s *Server) calculateNetworkFee(reqParams request.Params) (interface{}, *response.Error) {
	if len(reqParams) < 1 {
		return 0, response.ErrInvalidParams
	}
	byteTx, err := reqParams[0].GetBytesBase64()
	if err != nil {
		return 0, response.WrapErrorWithData(response.ErrInvalidParams, err)
	}
	tx, err := transaction.NewTransactionFromBytes(byteTx)
	if err != nil {
		return 0, response.WrapErrorWithData(response.ErrInvalidParams, err)
	}
	hashablePart, err := tx.EncodeHashableFields()
	if err != nil {
		return 0, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("failed to compute tx size: %w", err))
	}
	size := len(hashablePart) + io.GetVarSize(len(tx.Signers))
	var (
		ef     int64
		netFee int64
	)
	for i, signer := range tx.Signers {
		var verificationScript []byte
		for _, w := range tx.Scripts {
			if w.VerificationScript != nil && hash.Hash160(w.VerificationScript).Equals(signer.Account) {
				// then it's a standard sig/multisig witness
				verificationScript = w.VerificationScript
				break
			}
		}
		if verificationScript == nil { // then it still might be a contract-based verification
			gasConsumed, err := s.chain.VerifyWitness(signer.Account, tx, &tx.Scripts[i], int64(s.config.MaxGasInvoke))
			if err != nil {
				return 0, response.NewRPCError(fmt.Sprintf("contract verification for signer #%d failed", i), err.Error(), err)
			}
			netFee += gasConsumed
			size += io.GetVarSize([]byte{}) + // verification script is empty (contract-based witness)
				io.GetVarSize(tx.Scripts[i].InvocationScript) // invocation script might not be empty (args for `verify`)
			continue
		}

		if ef == 0 {
			ef = s.chain.GetBaseExecFee()
		}
		fee, sizeDelta := fee.Calculate(ef, verificationScript)
		netFee += fee
		size += sizeDelta
	}
	fee := s.chain.FeePerByte()
	netFee += int64(size) * fee
	return result.NetworkFee{Value: netFee}, nil
}

// getApplicationLog returns the contract log based on the specified txid or blockid.
func (s *Server) getApplicationLog(reqParams request.Params) (interface{}, *response.Error) {
	hash, err := reqParams.Value(0).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	trig := trigger.All
	if len(reqParams) > 1 {
		trigString, err := reqParams.Value(1).GetString()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		trig, err = trigger.FromString(trigString)
		if err != nil {
			return nil, response.ErrInvalidParams
		}
	}

	appExecResults, err := s.chain.GetAppExecResults(hash, trigger.All)
	if err != nil {
		return nil, response.NewRPCError("Unknown transaction or block", "", err)
	}
	return result.NewApplicationLog(hash, appExecResults, trig), nil
}

func (s *Server) getNEP11Tokens(h util.Uint160, acc util.Uint160, bw *io.BufBinWriter) ([]stackitem.Item, error) {
	item, finalize, err := s.invokeReadOnly(bw, h, "tokensOf", acc)
	if err != nil {
		return nil, err
	}
	defer finalize()
	if (item.Type() == stackitem.InteropT) && iterator.IsIterator(item) {
		vals, _ := iterator.Values(item, s.config.MaxNEP11Tokens)
		return vals, nil
	}
	return nil, fmt.Errorf("invalid `tokensOf` result type %s", item.String())
}

func (s *Server) getNEP11Balances(ps request.Params) (interface{}, *response.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	bs := &result.NEP11Balances{
		Address:  address.Uint160ToString(u),
		Balances: []result.NEP11AssetBalance{},
	}
	lastUpdated, err := s.chain.GetTokenLastUpdated(u)
	if err != nil {
		return nil, response.NewRPCError("Failed to get NEP-11 last updated block", err.Error(), err)
	}
	var count int
	stateSyncPoint := lastUpdated[math.MinInt32]
	bw := io.NewBufBinWriter()
contract_loop:
	for _, h := range s.chain.GetNEP11Contracts() {
		toks, err := s.getNEP11Tokens(h, u, bw)
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
		isDivisible := (cs.Manifest.ABI.GetMethod("balanceOf", 2) != nil)
		lub, ok := lastUpdated[cs.ID]
		if !ok {
			cfg := s.chain.GetConfig()
			if !cfg.P2PStateExchangeExtensions && cfg.RemoveUntraceableBlocks {
				return nil, response.NewInternalServerError(fmt.Sprintf("failed to get LastUpdatedBlock for balance of %s token", cs.Hash.StringLE()), nil)
			}
			lub = stateSyncPoint
		}
		bs.Balances = append(bs.Balances, result.NEP11AssetBalance{
			Asset:  h,
			Tokens: make([]result.NEP11TokenBalance, 0, len(toks)),
		})
		curAsset := &bs.Balances[len(bs.Balances)-1]
		for i := range toks {
			id, err := toks[i].TryBytes()
			if err != nil || len(id) > storage.MaxStorageKeyLen {
				continue
			}
			var amount = "1"
			if isDivisible {
				balance, err := s.getTokenBalance(h, u, id, bw)
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

func (s *Server) getNEP11Properties(ps request.Params) (interface{}, *response.Error) {
	asset, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	token, err := ps.Value(1).GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	props, err := s.invokeNEP11Properties(asset, token, nil)
	if err != nil {
		return nil, response.NewRPCError("failed to get NEP-11 properties", err.Error(), err)
	}
	res := make(map[string]interface{})
	for _, kv := range props {
		key, err := kv.Key.TryBytes()
		if err != nil {
			continue
		}
		var val interface{}
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

func (s *Server) getNEP17Balances(ps request.Params) (interface{}, *response.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	bs := &result.NEP17Balances{
		Address:  address.Uint160ToString(u),
		Balances: []result.NEP17Balance{},
	}
	lastUpdated, err := s.chain.GetTokenLastUpdated(u)
	if err != nil {
		return nil, response.NewRPCError("Failed to get NEP-17 last updated block", err.Error(), err)
	}
	stateSyncPoint := lastUpdated[math.MinInt32]
	bw := io.NewBufBinWriter()
	for _, h := range s.chain.GetNEP17Contracts() {
		balance, err := s.getTokenBalance(h, u, nil, bw)
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
			if !cfg.P2PStateExchangeExtensions && cfg.RemoveUntraceableBlocks {
				return nil, response.NewInternalServerError(fmt.Sprintf("failed to get LastUpdatedBlock for balance of %s token", cs.Hash.StringLE()), nil)
			}
			lub = stateSyncPoint
		}
		bs.Balances = append(bs.Balances, result.NEP17Balance{
			Asset:       h,
			Amount:      balance.String(),
			LastUpdated: lub,
		})
	}
	return bs, nil
}

func (s *Server) invokeReadOnly(bw *io.BufBinWriter, h util.Uint160, method string, params ...interface{}) (stackitem.Item, func(), error) {
	if bw == nil {
		bw = io.NewBufBinWriter()
	} else {
		bw.Reset()
	}
	emit.AppCall(bw.BinWriter, h, method, callflag.ReadStates|callflag.AllowCall, params...)
	if bw.Err != nil {
		return nil, nil, fmt.Errorf("failed to create `%s` invocation script: %w", method, bw.Err)
	}
	script := bw.Bytes()
	tx := &transaction.Transaction{Script: script}
	b, err := s.getFakeNextBlock()
	if err != nil {
		return nil, nil, err
	}
	ic := s.chain.GetTestVM(trigger.Application, tx, b)
	ic.VM.GasLimit = core.HeaderVerificationGasLimit
	ic.VM.LoadScriptWithFlags(script, callflag.All)
	err = ic.VM.Run()
	if err != nil {
		ic.Finalize()
		return nil, nil, fmt.Errorf("failed to run `%s` for %s: %w", method, h.StringLE(), err)
	}
	if ic.VM.Estack().Len() != 1 {
		ic.Finalize()
		return nil, nil, fmt.Errorf("invalid `%s` return values count: expected 1, got %d", method, ic.VM.Estack().Len())
	}
	return ic.VM.Estack().Pop().Item(), ic.Finalize, nil
}

func (s *Server) getTokenBalance(h util.Uint160, acc util.Uint160, id []byte, bw *io.BufBinWriter) (*big.Int, error) {
	var (
		item     stackitem.Item
		finalize func()
		err      error
	)
	if id == nil { // NEP-17 and NEP-11 generic.
		item, finalize, err = s.invokeReadOnly(bw, h, "balanceOf", acc)
	} else { // NEP-11 divisible.
		item, finalize, err = s.invokeReadOnly(bw, h, "balanceOf", acc, id)
	}
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

func getTimestampsAndLimit(ps request.Params, index int) (uint64, uint64, int, int, error) {
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

func (s *Server) getNEP11Transfers(ps request.Params) (interface{}, *response.Error) {
	return s.getTokenTransfers(ps, true)
}

func (s *Server) getNEP17Transfers(ps request.Params) (interface{}, *response.Error) {
	return s.getTokenTransfers(ps, false)
}

func (s *Server) getTokenTransfers(ps request.Params, isNEP11 bool) (interface{}, *response.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	start, end, limit, page, err := getTimestampsAndLimit(ps, 1)
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}

	bs := &tokenTransfers{
		Address:  address.Uint160ToString(u),
		Received: []interface{}{},
		Sent:     []interface{}{},
	}
	cache := make(map[int32]util.Uint160)
	var resCount, frameCount int
	// handleTransfer returns items to be added into received and sent arrays
	// along with a continue flag and error.
	var handleTransfer = func(tr *state.NEP17Transfer) (*result.NEP17Transfer, *result.NEP17Transfer, bool, error) {
		var received, sent *result.NEP17Transfer

		// Iterating from newest to oldest, not yet reached required
		// time frame, continue looping.
		if tr.Timestamp > end {
			return nil, nil, true, nil
		}
		// Iterating from newest to oldest, moved past required
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
		if tr.Amount.Sign() > 0 { // token was received
			transfer.Amount = tr.Amount.String()
			if !tr.From.Equals(util.Uint160{}) {
				transfer.Address = address.Uint160ToString(tr.From)
			}
			received = &result.NEP17Transfer{}
			*received = transfer // Make a copy, transfer is to be modified below.
		} else {
			transfer.Amount = new(big.Int).Neg(&tr.Amount).String()
			if !tr.To.Equals(util.Uint160{}) {
				transfer.Address = address.Uint160ToString(tr.To)
			}
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
		return nil, response.NewInternalServerError(fmt.Sprintf("invalid transfer log: %v", err), err)
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

func (s *Server) contractIDFromParam(param *request.Param) (int32, *response.Error) {
	var result int32
	if param == nil {
		return 0, response.ErrInvalidParams
	}
	if scriptHash, err := param.GetUint160FromHex(); err == nil {
		cs := s.chain.GetContractState(scriptHash)
		if cs == nil {
			return 0, response.ErrUnknown
		}
		result = cs.ID
	} else {
		id, err := param.GetInt()
		if err != nil {
			return 0, response.ErrInvalidParams
		}
		if err := checkInt32(id); err != nil {
			return 0, response.WrapErrorWithData(response.ErrInvalidParams, err)
		}
		result = int32(id)
	}
	return result, nil
}

// getContractScriptHashFromParam returns the contract script hash by hex contract hash, address, id or native contract name.
func (s *Server) contractScriptHashFromParam(param *request.Param) (util.Uint160, *response.Error) {
	var result util.Uint160
	if param == nil {
		return result, response.ErrInvalidParams
	}
	nameOrHashOrIndex, err := param.GetString()
	if err != nil {
		return result, response.ErrInvalidParams
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
		return result, response.NewRPCError("Unknown contract", "", err)
	}
	if err := checkInt32(id); err != nil {
		return result, response.WrapErrorWithData(response.ErrInvalidParams, err)
	}
	result, err = s.chain.GetContractScriptHash(int32(id))
	if err != nil {
		return result, response.NewRPCError("Unknown contract", "", err)
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

func (s *Server) getProof(ps request.Params) (interface{}, *response.Error) {
	if s.chain.GetConfig().KeepOnlyLatestState {
		return nil, response.NewInvalidRequestError("'getproof' is not supported", errKeepOnlyLatestState)
	}
	root, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	sc, err := ps.Value(1).GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	key, err := ps.Value(2).GetBytesBase64()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	cs, respErr := s.getHistoricalContractState(root, sc)
	if respErr != nil {
		return nil, respErr
	}
	skey := makeStorageKey(cs.ID, key)
	proof, err := s.chain.GetStateModule().GetStateProof(root, skey)
	if err != nil {
		return nil, response.NewInternalServerError("failed to get proof", err)
	}
	return &result.ProofWithKey{
		Key:   skey,
		Proof: proof,
	}, nil
}

func (s *Server) verifyProof(ps request.Params) (interface{}, *response.Error) {
	if s.chain.GetConfig().KeepOnlyLatestState {
		return nil, response.NewInvalidRequestError("'verifyproof' is not supported", errKeepOnlyLatestState)
	}
	root, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	proofStr, err := ps.Value(1).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	var p result.ProofWithKey
	if err := p.FromString(proofStr); err != nil {
		return nil, response.ErrInvalidParams
	}
	vp := new(result.VerifyProof)
	val, ok := mpt.VerifyProof(root, p.Key, p.Proof)
	if ok {
		vp.Value = val
	}
	return vp, nil
}

func (s *Server) getState(ps request.Params) (interface{}, *response.Error) {
	root, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("invalid stateroot"))
	}
	if s.chain.GetConfig().KeepOnlyLatestState {
		curr, err := s.chain.GetStateModule().GetStateRoot(s.chain.BlockHeight())
		if err != nil {
			return nil, response.NewInternalServerError("failed to get current stateroot", err)
		}
		if !curr.Root.Equals(root) {
			return nil, response.NewInvalidRequestError("'getstate' is not supported for old states", errKeepOnlyLatestState)
		}
	}
	csHash, err := ps.Value(1).GetUint160FromHex()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("invalid contract hash"))
	}
	key, err := ps.Value(2).GetBytesBase64()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("invalid key"))
	}
	cs, respErr := s.getHistoricalContractState(root, csHash)
	if respErr != nil {
		return nil, respErr
	}
	sKey := makeStorageKey(cs.ID, key)
	res, err := s.chain.GetStateModule().GetState(root, sKey)
	if err != nil {
		return nil, response.NewInternalServerError("failed to get historical item state", err)
	}
	return res, nil
}

func (s *Server) findStates(ps request.Params) (interface{}, *response.Error) {
	root, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("invalid stateroot"))
	}
	if s.chain.GetConfig().KeepOnlyLatestState {
		curr, err := s.chain.GetStateModule().GetStateRoot(s.chain.BlockHeight())
		if err != nil {
			return nil, response.NewInternalServerError("failed to get current stateroot", err)
		}
		if !curr.Root.Equals(root) {
			return nil, response.NewInvalidRequestError("'findstates' is not supported for old states", errKeepOnlyLatestState)
		}
	}
	csHash, err := ps.Value(1).GetUint160FromHex()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("invalid contract hash: %w", err))
	}
	prefix, err := ps.Value(2).GetBytesBase64()
	if err != nil {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("invalid prefix: %w", err))
	}
	var (
		key   []byte
		count = s.config.MaxFindResultItems
	)
	if len(ps) > 3 {
		key, err = ps.Value(3).GetBytesBase64()
		if err != nil {
			return nil, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("invalid key: %w", err))
		}
		if len(key) > 0 {
			if !bytes.HasPrefix(key, prefix) {
				return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("key doesn't match prefix"))
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
			return nil, response.WrapErrorWithData(response.ErrInvalidParams, fmt.Errorf("invalid count: %w", err))
		}
		if count > s.config.MaxFindResultItems {
			count = s.config.MaxFindResultItems
		}
	}
	cs, respErr := s.getHistoricalContractState(root, csHash)
	if respErr != nil {
		return nil, respErr
	}
	pKey := makeStorageKey(cs.ID, prefix)
	kvs, err := s.chain.GetStateModule().FindStates(root, pKey, key, count+1) // +1 to define result truncation
	if err != nil {
		return nil, response.NewInternalServerError("failed to find historical items", err)
	}
	res := result.FindStates{}
	if len(kvs) == count+1 {
		res.Truncated = true
		kvs = kvs[:len(kvs)-1]
	}
	if len(kvs) > 0 {
		proof, err := s.chain.GetStateModule().GetStateProof(root, kvs[0].Key)
		if err != nil {
			return nil, response.NewInternalServerError("failed to get first proof", err)
		}
		res.FirstProof = &result.ProofWithKey{
			Key:   kvs[0].Key,
			Proof: proof,
		}
	}
	if len(kvs) > 1 {
		proof, err := s.chain.GetStateModule().GetStateProof(root, kvs[len(kvs)-1].Key)
		if err != nil {
			return nil, response.NewInternalServerError("failed to get first proof", err)
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

func (s *Server) getHistoricalContractState(root util.Uint256, csHash util.Uint160) (*state.Contract, *response.Error) {
	csKey := makeStorageKey(native.ManagementContractID, native.MakeContractKey(csHash))
	csBytes, err := s.chain.GetStateModule().GetState(root, csKey)
	if err != nil {
		return nil, response.NewInternalServerError("failed to get historical contract state", err)
	}
	contract := new(state.Contract)
	err = stackitem.DeserializeConvertible(csBytes, contract)
	if err != nil {
		return nil, response.NewInternalServerError("failed to deserialize historical contract state", err)
	}
	return contract, nil
}

func (s *Server) getStateHeight(_ request.Params) (interface{}, *response.Error) {
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

func (s *Server) getStateRoot(ps request.Params) (interface{}, *response.Error) {
	p := ps.Value(0)
	if p == nil {
		return nil, response.NewRPCError("Invalid parameter.", "", nil)
	}
	var rt *state.MPTRoot
	var h util.Uint256
	height, err := p.GetIntStrict()
	if err == nil {
		if err := checkUint32(height); err != nil {
			return nil, response.WrapErrorWithData(response.ErrInvalidParams, err)
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
		return nil, response.NewRPCError("Unknown state root.", "", err)
	}
	return rt, nil
}

func (s *Server) getStorage(ps request.Params) (interface{}, *response.Error) {
	id, rErr := s.contractIDFromParam(ps.Value(0))
	if rErr == response.ErrUnknown {
		return nil, nil
	}
	if rErr != nil {
		return nil, rErr
	}

	key, err := ps.Value(1).GetBytesBase64()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	item := s.chain.GetStorageItem(id, key)
	if item == nil {
		return "", nil
	}

	return []byte(item), nil
}

func (s *Server) getrawtransaction(reqParams request.Params) (interface{}, *response.Error) {
	txHash, err := reqParams.Value(0).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	tx, height, err := s.chain.GetTransaction(txHash)
	if err != nil {
		err = fmt.Errorf("invalid transaction %s: %w", txHash, err)
		return nil, response.NewRPCError("Unknown transaction", err.Error(), err)
	}
	if v, _ := reqParams.Value(1).GetBoolean(); v {
		if height == math.MaxUint32 {
			return result.NewTransactionOutputRaw(tx, nil, nil, s.chain), nil
		}
		_header := s.chain.GetHeaderHash(int(height))
		header, err := s.chain.GetHeader(_header)
		if err != nil {
			return nil, response.NewRPCError("Failed to get header for the transaction", err.Error(), err)
		}
		aers, err := s.chain.GetAppExecResults(txHash, trigger.Application)
		if err != nil {
			return nil, response.NewRPCError("Failed to get application log for the transaction", err.Error(), err)
		}
		if len(aers) == 0 {
			return nil, response.NewRPCError("Application log for the transaction is empty", "", nil)
		}
		return result.NewTransactionOutputRaw(tx, header, &aers[0], s.chain), nil
	}
	return tx.Bytes(), nil
}

func (s *Server) getTransactionHeight(ps request.Params) (interface{}, *response.Error) {
	h, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	_, height, err := s.chain.GetTransaction(h)
	if err != nil || height == math.MaxUint32 {
		return nil, response.NewRPCError("Unknown transaction", "", nil)
	}

	return height, nil
}

// getContractState returns contract state (contract information, according to the contract script hash,
// contract id or native contract name).
func (s *Server) getContractState(reqParams request.Params) (interface{}, *response.Error) {
	scriptHash, err := s.contractScriptHashFromParam(reqParams.Value(0))
	if err != nil {
		return nil, err
	}
	cs := s.chain.GetContractState(scriptHash)
	if cs == nil {
		return nil, response.NewRPCError("Unknown contract", "", nil)
	}
	return cs, nil
}

func (s *Server) getNativeContracts(_ request.Params) (interface{}, *response.Error) {
	return s.chain.GetNatives(), nil
}

// getBlockSysFee returns the system fees of the block, based on the specified index.
func (s *Server) getBlockSysFee(reqParams request.Params) (interface{}, *response.Error) {
	num, err := s.blockHeightFromParam(reqParams.Value(0))
	if err != nil {
		return 0, response.NewRPCError("Invalid height", "", nil)
	}

	headerHash := s.chain.GetHeaderHash(num)
	block, errBlock := s.chain.GetBlock(headerHash)
	if errBlock != nil {
		return 0, response.NewRPCError(errBlock.Error(), "", nil)
	}

	var blockSysFee int64
	for _, tx := range block.Transactions {
		blockSysFee += tx.SystemFee
	}

	return blockSysFee, nil
}

// getBlockHeader returns the corresponding block header information according to the specified script hash.
func (s *Server) getBlockHeader(reqParams request.Params) (interface{}, *response.Error) {
	param := reqParams.Value(0)
	hash, respErr := s.blockHashFromParam(param)
	if respErr != nil {
		return nil, respErr
	}

	verbose, _ := reqParams.Value(1).GetBoolean()
	h, err := s.chain.GetHeader(hash)
	if err != nil {
		return nil, response.NewRPCError("unknown block", "", nil)
	}

	if verbose {
		return result.NewHeader(h, s.chain), nil
	}

	buf := io.NewBufBinWriter()
	h.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil, response.NewInternalServerError("encoding error", buf.Err)
	}
	return buf.Bytes(), nil
}

// getUnclaimedGas returns unclaimed GAS amount of the specified address.
func (s *Server) getUnclaimedGas(ps request.Params) (interface{}, *response.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	neo, _ := s.chain.GetGoverningTokenBalance(u)
	if neo.Sign() == 0 {
		return result.UnclaimedGas{
			Address: u,
		}, nil
	}
	gas, err := s.chain.CalculateClaimable(u, s.chain.BlockHeight()+1) // +1 as in C#, for the next block.
	if err != nil {
		return nil, response.NewInternalServerError("can't calculate claimable", err)
	}
	return result.UnclaimedGas{
		Address:   u,
		Unclaimed: *gas,
	}, nil
}

// getNextBlockValidators returns validators for the next block with voting status.
func (s *Server) getNextBlockValidators(_ request.Params) (interface{}, *response.Error) {
	var validators keys.PublicKeys

	validators, err := s.chain.GetNextBlockValidators()
	if err != nil {
		return nil, response.NewRPCError("can't get next block validators", "", err)
	}
	enrollments, err := s.chain.GetEnrollments()
	if err != nil {
		return nil, response.NewRPCError("can't get enrollments", "", err)
	}
	var res = make([]result.Validator, 0)
	for _, v := range enrollments {
		res = append(res, result.Validator{
			PublicKey: *v.Key,
			Votes:     v.Votes.Int64(),
			Active:    validators.Contains(v.Key),
		})
	}
	return res, nil
}

// getCommittee returns the current list of NEO committee members.
func (s *Server) getCommittee(_ request.Params) (interface{}, *response.Error) {
	keys, err := s.chain.GetCommittee()
	if err != nil {
		return nil, response.NewInternalServerError("can't get committee members", err)
	}
	return keys, nil
}

// invokeFunction implements the `invokeFunction` RPC call.
func (s *Server) invokeFunction(reqParams request.Params) (interface{}, *response.Error) {
	if len(reqParams) < 2 {
		return nil, response.ErrInvalidParams
	}
	scriptHash, responseErr := s.contractScriptHashFromParam(reqParams.Value(0))
	if responseErr != nil {
		return nil, responseErr
	}
	method, err := reqParams[1].GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	var params *request.Param
	if len(reqParams) > 2 {
		params = &reqParams[2]
	}
	tx := &transaction.Transaction{}
	if len(reqParams) > 3 {
		signers, _, err := reqParams[3].GetSignersWithWitnesses()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		tx.Signers = signers
	}
	var verbose bool
	if len(reqParams) > 4 {
		verbose, err = reqParams[4].GetBoolean()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
	}
	if len(tx.Signers) == 0 {
		tx.Signers = []transaction.Signer{{Account: util.Uint160{}, Scopes: transaction.None}}
	}
	script, err := request.CreateFunctionInvocationScript(scriptHash, method, params)
	if err != nil {
		return nil, response.NewInternalServerError("can't create invocation script", err)
	}
	tx.Script = script
	return s.runScriptInVM(trigger.Application, script, util.Uint160{}, tx, verbose)
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokescript(reqParams request.Params) (interface{}, *response.Error) {
	if len(reqParams) < 1 {
		return nil, response.ErrInvalidParams
	}

	script, err := reqParams[0].GetBytesBase64()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	tx := &transaction.Transaction{}
	if len(reqParams) > 1 {
		signers, witnesses, err := reqParams[1].GetSignersWithWitnesses()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		tx.Signers = signers
		tx.Scripts = witnesses
	}
	var verbose bool
	if len(reqParams) > 2 {
		verbose, err = reqParams[2].GetBoolean()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
	}
	if len(tx.Signers) == 0 {
		tx.Signers = []transaction.Signer{{Account: util.Uint160{}, Scopes: transaction.None}}
	}
	tx.Script = script
	return s.runScriptInVM(trigger.Application, script, util.Uint160{}, tx, verbose)
}

// invokeContractVerify implements the `invokecontractverify` RPC call.
func (s *Server) invokeContractVerify(reqParams request.Params) (interface{}, *response.Error) {
	scriptHash, responseErr := s.contractScriptHashFromParam(reqParams.Value(0))
	if responseErr != nil {
		return nil, responseErr
	}

	bw := io.NewBufBinWriter()
	if len(reqParams) > 1 {
		args, err := reqParams[1].GetArray() // second `invokecontractverify` parameter is an array of arguments for `verify` method
		if err != nil {
			return nil, response.WrapErrorWithData(response.ErrInvalidParams, err)
		}
		if len(args) > 0 {
			err := request.ExpandArrayIntoScript(bw.BinWriter, args)
			if err != nil {
				return nil, response.NewRPCError("can't create witness invocation script", err.Error(), err)
			}
		}
	}
	invocationScript := bw.Bytes()

	tx := &transaction.Transaction{Script: []byte{byte(opcode.RET)}} // need something in script
	if len(reqParams) > 2 {
		signers, witnesses, err := reqParams[2].GetSignersWithWitnesses()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		tx.Signers = signers
		tx.Scripts = witnesses
	} else { // fill the only known signer - the contract with `verify` method
		tx.Signers = []transaction.Signer{{Account: scriptHash}}
		tx.Scripts = []transaction.Witness{{InvocationScript: invocationScript, VerificationScript: []byte{}}}
	}
	return s.runScriptInVM(trigger.Verification, invocationScript, scriptHash, tx, false)
}

func (s *Server) getFakeNextBlock() (*block.Block, error) {
	// When transferring funds, script execution does no auto GAS claim,
	// because it depends on persisting tx height.
	// This is why we provide block here.
	b := block.New(s.stateRootEnabled)
	b.Index = s.chain.BlockHeight() + 1
	hdr, err := s.chain.GetHeader(s.chain.GetHeaderHash(int(s.chain.BlockHeight())))
	if err != nil {
		return nil, err
	}
	b.Timestamp = hdr.Timestamp + uint64(s.chain.GetConfig().SecondsPerBlock*int(time.Second/time.Millisecond))
	return b, nil
}

// runScriptInVM runs given script in a new test VM and returns the invocation
// result. The script is either a simple script in case of `application` trigger
// witness invocation script in case of `verification` trigger (it pushes `verify`
// arguments on stack before verification). In case of contract verification
// contractScriptHash should be specified.
func (s *Server) runScriptInVM(t trigger.Type, script []byte, contractScriptHash util.Uint160, tx *transaction.Transaction, verbose bool) (*result.Invoke, *response.Error) {
	b, err := s.getFakeNextBlock()
	if err != nil {
		return nil, response.NewInternalServerError("can't create fake block", err)
	}
	ic := s.chain.GetTestVM(t, tx, b)
	if verbose {
		ic.VM.EnableInvocationTree()
	}
	ic.VM.GasLimit = int64(s.config.MaxGasInvoke)
	if t == trigger.Verification {
		// We need this special case because witnesses verification is not the simple System.Contract.Call,
		// and we need to define exactly the amount of gas consumed for a contract witness verification.
		gasPolicy := s.chain.GetMaxVerificationGAS()
		if ic.VM.GasLimit > gasPolicy {
			ic.VM.GasLimit = gasPolicy
		}

		err := s.chain.InitVerificationContext(ic, contractScriptHash, &transaction.Witness{InvocationScript: script, VerificationScript: []byte{}})
		if err != nil {
			return nil, response.NewInternalServerError("can't prepare verification VM", err)
		}
	} else {
		ic.VM.LoadScriptWithFlags(script, callflag.All)
	}
	err = ic.VM.Run()
	var faultException string
	if err != nil {
		faultException = err.Error()
	}
	return result.NewInvoke(ic, script, faultException, s.config.MaxIteratorResultItems), nil
}

// submitBlock broadcasts a raw block over the NEO network.
func (s *Server) submitBlock(reqParams request.Params) (interface{}, *response.Error) {
	blockBytes, err := reqParams.Value(0).GetBytesBase64()
	if err != nil {
		return nil, response.NewInvalidParamsError("missing parameter or not base64", err)
	}
	b := block.New(s.stateRootEnabled)
	r := io.NewBinReaderFromBuf(blockBytes)
	b.DecodeBinary(r)
	if r.Err != nil {
		return nil, response.NewInvalidParamsError("can't decode block", r.Err)
	}
	err = s.chain.AddBlock(b)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrInvalidBlockIndex) || errors.Is(err, core.ErrAlreadyExists):
			return nil, response.WrapErrorWithData(response.ErrAlreadyExists, err)
		default:
			return nil, response.WrapErrorWithData(response.ErrValidationFailed, err)
		}
	}
	return &result.RelayResult{
		Hash: b.Hash(),
	}, nil
}

// submitNotaryRequest broadcasts P2PNotaryRequest over the NEO network.
func (s *Server) submitNotaryRequest(ps request.Params) (interface{}, *response.Error) {
	if !s.chain.P2PSigExtensionsEnabled() {
		return nil, response.NewInternalServerError("P2PNotaryRequest was received, but P2PSignatureExtensions are disabled", nil)
	}

	bytePayload, err := ps.Value(0).GetBytesBase64()
	if err != nil {
		return nil, response.NewInvalidParamsError("not base64", err)
	}
	r, err := payload.NewP2PNotaryRequestFromBytes(bytePayload)
	if err != nil {
		return nil, response.NewInvalidParamsError("can't decode notary payload", err)
	}
	return getRelayResult(s.coreServer.RelayP2PNotaryRequest(r), r.FallbackTransaction.Hash())
}

// getRelayResult returns successful relay result or an error.
func getRelayResult(err error, hash util.Uint256) (interface{}, *response.Error) {
	switch {
	case err == nil:
		return result.RelayResult{
			Hash: hash,
		}, nil
	case errors.Is(err, core.ErrAlreadyExists):
		return nil, response.WrapErrorWithData(response.ErrAlreadyExists, err)
	case errors.Is(err, core.ErrOOM):
		return nil, response.WrapErrorWithData(response.ErrOutOfMemory, err)
	case errors.Is(err, core.ErrPolicy):
		return nil, response.WrapErrorWithData(response.ErrPolicyFail, err)
	default:
		return nil, response.WrapErrorWithData(response.ErrValidationFailed, err)
	}
}

func (s *Server) submitOracleResponse(ps request.Params) (interface{}, *response.Error) {
	if s.oracle == nil {
		return nil, response.NewInternalServerError("oracle is not enabled", nil)
	}
	var pub *keys.PublicKey
	pubBytes, err := ps.Value(0).GetBytesBase64()
	if err == nil {
		pub, err = keys.NewPublicKeyFromBytes(pubBytes, elliptic.P256())
	}
	if err != nil {
		return nil, response.NewInvalidParamsError("public key is missing", err)
	}
	reqID, err := ps.Value(1).GetInt()
	if err != nil {
		return nil, response.NewInvalidParamsError("request ID is missing", err)
	}
	txSig, err := ps.Value(2).GetBytesBase64()
	if err != nil {
		return nil, response.NewInvalidParamsError("tx signature is missing", err)
	}
	msgSig, err := ps.Value(3).GetBytesBase64()
	if err != nil {
		return nil, response.NewInvalidParamsError("msg signature is missing", err)
	}
	data := broadcaster.GetMessage(pubBytes, uint64(reqID), txSig)
	if !pub.Verify(msgSig, hash.Sha256(data).BytesBE()) {
		return nil, response.NewRPCError("Invalid sign", "", nil)
	}
	s.oracle.AddResponse(pub, uint64(reqID), txSig)
	return json.RawMessage([]byte("{}")), nil
}

func (s *Server) sendrawtransaction(reqParams request.Params) (interface{}, *response.Error) {
	if len(reqParams) < 1 {
		return nil, response.NewInvalidParamsError("not enough parameters", nil)
	}
	byteTx, err := reqParams[0].GetBytesBase64()
	if err != nil {
		return nil, response.NewInvalidParamsError("not base64", err)
	}
	tx, err := transaction.NewTransactionFromBytes(byteTx)
	if err != nil {
		return nil, response.NewInvalidParamsError("can't decode transaction", err)
	}
	return getRelayResult(s.coreServer.RelayTxn(tx), tx.Hash())
}

// subscribe handles subscription requests from websocket clients.
func (s *Server) subscribe(reqParams request.Params, sub *subscriber) (interface{}, *response.Error) {
	streamName, err := reqParams.Value(0).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	event, err := response.GetEventIDFromString(streamName)
	if err != nil || event == response.MissedEventID {
		return nil, response.ErrInvalidParams
	}
	if event == response.NotaryRequestEventID && !s.chain.P2PSigExtensionsEnabled() {
		return nil, response.WrapErrorWithData(response.ErrInvalidParams, errors.New("P2PSigExtensions are disabled"))
	}
	// Optional filter.
	var filter interface{}
	if p := reqParams.Value(1); p != nil {
		param := *p
		jd := json.NewDecoder(bytes.NewReader(param.RawMessage))
		jd.DisallowUnknownFields()
		switch event {
		case response.BlockEventID:
			flt := new(request.BlockFilter)
			err = jd.Decode(flt)
			filter = *flt
		case response.TransactionEventID, response.NotaryRequestEventID:
			flt := new(request.TxFilter)
			err = jd.Decode(flt)
			filter = *flt
		case response.NotificationEventID:
			flt := new(request.NotificationFilter)
			err = jd.Decode(flt)
			filter = *flt
		case response.ExecutionEventID:
			flt := new(request.ExecutionFilter)
			err = jd.Decode(flt)
			if err == nil && (flt.State == "HALT" || flt.State == "FAULT") {
				filter = *flt
			} else if err == nil {
				err = errors.New("invalid state")
			}
		}
		if err != nil {
			return nil, response.ErrInvalidParams
		}
	}

	s.subsLock.Lock()
	defer s.subsLock.Unlock()
	select {
	case <-s.shutdown:
		return nil, response.NewInternalServerError("server is shutting down", nil)
	default:
	}
	var id int
	for ; id < len(sub.feeds); id++ {
		if sub.feeds[id].event == response.InvalidEventID {
			break
		}
	}
	if id == len(sub.feeds) {
		return nil, response.NewInternalServerError("maximum number of subscriptions is reached", nil)
	}
	sub.feeds[id].event = event
	sub.feeds[id].filter = filter
	s.subscribeToChannel(event)
	return strconv.FormatInt(int64(id), 10), nil
}

// subscribeToChannel subscribes RPC server to appropriate chain events if
// it's not yet subscribed for them. It's supposed to be called with s.subsLock
// taken by the caller.
func (s *Server) subscribeToChannel(event response.EventID) {
	switch event {
	case response.BlockEventID:
		if s.blockSubs == 0 {
			s.chain.SubscribeForBlocks(s.blockCh)
		}
		s.blockSubs++
	case response.TransactionEventID:
		if s.transactionSubs == 0 {
			s.chain.SubscribeForTransactions(s.transactionCh)
		}
		s.transactionSubs++
	case response.NotificationEventID:
		if s.notificationSubs == 0 {
			s.chain.SubscribeForNotifications(s.notificationCh)
		}
		s.notificationSubs++
	case response.ExecutionEventID:
		if s.executionSubs == 0 {
			s.chain.SubscribeForExecutions(s.executionCh)
		}
		s.executionSubs++
	case response.NotaryRequestEventID:
		if s.notaryRequestSubs == 0 {
			s.coreServer.SubscribeForNotaryRequests(s.notaryRequestCh)
		}
		s.notaryRequestSubs++
	}
}

// unsubscribe handles unsubscription requests from websocket clients.
func (s *Server) unsubscribe(reqParams request.Params, sub *subscriber) (interface{}, *response.Error) {
	id, err := reqParams.Value(0).GetInt()
	if err != nil || id < 0 {
		return nil, response.ErrInvalidParams
	}
	s.subsLock.Lock()
	defer s.subsLock.Unlock()
	if len(sub.feeds) <= id || sub.feeds[id].event == response.InvalidEventID {
		return nil, response.ErrInvalidParams
	}
	event := sub.feeds[id].event
	sub.feeds[id].event = response.InvalidEventID
	sub.feeds[id].filter = nil
	s.unsubscribeFromChannel(event)
	return true, nil
}

// unsubscribeFromChannel unsubscribes RPC server from appropriate chain events
// if there are no other subscribers for it. It's supposed to be called with
// s.subsLock taken by the caller.
func (s *Server) unsubscribeFromChannel(event response.EventID) {
	switch event {
	case response.BlockEventID:
		s.blockSubs--
		if s.blockSubs == 0 {
			s.chain.UnsubscribeFromBlocks(s.blockCh)
		}
	case response.TransactionEventID:
		s.transactionSubs--
		if s.transactionSubs == 0 {
			s.chain.UnsubscribeFromTransactions(s.transactionCh)
		}
	case response.NotificationEventID:
		s.notificationSubs--
		if s.notificationSubs == 0 {
			s.chain.UnsubscribeFromNotifications(s.notificationCh)
		}
	case response.ExecutionEventID:
		s.executionSubs--
		if s.executionSubs == 0 {
			s.chain.UnsubscribeFromExecutions(s.executionCh)
		}
	case response.NotaryRequestEventID:
		s.notaryRequestSubs--
		if s.notaryRequestSubs == 0 {
			s.coreServer.UnsubscribeFromNotaryRequests(s.notaryRequestCh)
		}
	}
}

func (s *Server) handleSubEvents() {
	b, err := json.Marshal(response.Notification{
		JSONRPC: request.JSONRPCVersion,
		Event:   response.MissedEventID,
		Payload: make([]interface{}, 0),
	})
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
		var resp = response.Notification{
			JSONRPC: request.JSONRPCVersion,
			Payload: make([]interface{}, 1),
		}
		var msg *websocket.PreparedMessage
		select {
		case <-s.shutdown:
			break chloop
		case b := <-s.blockCh:
			resp.Event = response.BlockEventID
			resp.Payload[0] = b
		case execution := <-s.executionCh:
			resp.Event = response.ExecutionEventID
			resp.Payload[0] = execution
		case notification := <-s.notificationCh:
			resp.Event = response.NotificationEventID
			resp.Payload[0] = notification
		case tx := <-s.transactionCh:
			resp.Event = response.TransactionEventID
			resp.Payload[0] = tx
		case e := <-s.notaryRequestCh:
			resp.Event = response.NotaryRequestEventID
			resp.Payload[0] = &subscriptions.NotaryRequestEvent{
				Type:          e.Type,
				NotaryRequest: e.Data.(*payload.P2PNotaryRequest),
			}
		}
		s.subsLock.RLock()
	subloop:
		for sub := range s.subscribers {
			if sub.overflown.Load() {
				continue
			}
			for i := range sub.feeds {
				if sub.feeds[i].Matches(&resp) {
					if msg == nil {
						b, err = json.Marshal(resp)
						if err != nil {
							s.log.Error("failed to marshal notification",
								zap.Error(err),
								zap.String("type", resp.Event.String()))
							break subloop
						}
						msg, err = websocket.NewPreparedMessage(websocket.TextMessage, b)
						if err != nil {
							s.log.Error("failed to prepare notification message",
								zap.Error(err),
								zap.String("type", resp.Event.String()))
							break subloop
						}
					}
					select {
					case sub.writer <- msg:
					default:
						sub.overflown.Store(true)
						// MissedEvent is to be delivered eventually.
						go func(sub *subscriber) {
							sub.writer <- overflowMsg
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
	// It's important to do it with lock held because no subscription routine
	// should be running concurrently to this one. And even if one is to run
	// after unlock, it'll see closed s.shutdown and won't subscribe.
	s.subsLock.Lock()
	// There might be no subscription in reality, but it's not a problem as
	// core.Blockchain allows unsubscribing non-subscribed channels.
	s.chain.UnsubscribeFromBlocks(s.blockCh)
	s.chain.UnsubscribeFromTransactions(s.transactionCh)
	s.chain.UnsubscribeFromNotifications(s.notificationCh)
	s.chain.UnsubscribeFromExecutions(s.executionCh)
	if s.chain.P2PSigExtensionsEnabled() {
		s.coreServer.UnsubscribeFromNotaryRequests(s.notaryRequestCh)
	}
	s.subsLock.Unlock()
drainloop:
	for {
		select {
		case <-s.blockCh:
		case <-s.executionCh:
		case <-s.notificationCh:
		case <-s.transactionCh:
		case <-s.notaryRequestCh:
		default:
			break drainloop
		}
	}
	// It's not required closing these, but since they're drained already
	// this is safe and it also allows to give a signal to Shutdown routine.
	close(s.blockCh)
	close(s.transactionCh)
	close(s.notificationCh)
	close(s.executionCh)
	close(s.notaryRequestCh)
}

func (s *Server) blockHeightFromParam(param *request.Param) (int, *response.Error) {
	num, err := param.GetInt()
	if err != nil {
		return 0, response.ErrInvalidParams
	}

	if num < 0 || num > int(s.chain.BlockHeight()) {
		return 0, invalidBlockHeightError(0, num)
	}
	return num, nil
}

func (s *Server) packResponse(r *request.In, result interface{}, respErr *response.Error) response.Abstract {
	resp := response.Abstract{
		HeaderAndError: response.HeaderAndError{
			Header: response.Header{
				JSONRPC: r.JSONRPC,
				ID:      r.RawID,
			},
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
func (s *Server) logRequestError(r *request.Request, jsonErr *response.Error) {
	logFields := []zap.Field{
		zap.Error(jsonErr.Cause),
	}

	if r.In != nil {
		logFields = append(logFields, zap.String("method", r.In.Method))
		params := request.Params(r.In.RawParams)
		logFields = append(logFields, zap.Any("params", params))
	}

	s.log.Error("Error encountered with rpc request", logFields...)
}

// writeHTTPErrorResponse writes an error response to the ResponseWriter.
func (s *Server) writeHTTPErrorResponse(r *request.In, w http.ResponseWriter, jsonErr *response.Error) {
	resp := s.packResponse(r, nil, jsonErr)
	s.writeHTTPServerResponse(&request.Request{In: r}, w, resp)
}

func (s *Server) writeHTTPServerResponse(r *request.Request, w http.ResponseWriter, resp response.AbstractResult) {
	// Errors can happen in many places and we can only catch ALL of them here.
	resp.RunForErrors(func(jsonErr *response.Error) {
		s.logRequestError(r, jsonErr)
	})
	if r.In != nil {
		resp := resp.(response.Abstract)
		if resp.Error != nil {
			w.WriteHeader(resp.Error.HTTPCode)
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if s.config.EnableCORSWorkaround {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
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

// validateAddress verifies that the address is a correct NEO address
// see https://docs.neo.org/en-us/node/cli/2.9.4/api/validateaddress.html
func validateAddress(addr interface{}) bool {
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
