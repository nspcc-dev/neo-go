package server

import (
	"bytes"
	"context"
	"crypto/elliptic"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
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
	"github.com/nspcc-dev/neo-go/pkg/services/oracle"
	"github.com/nspcc-dev/neo-go/pkg/services/oracle/broadcaster"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
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

		subsLock          sync.RWMutex
		subscribers       map[*subscriber]bool
		blockSubs         int
		executionSubs     int
		notificationSubs  int
		transactionSubs   int
		notaryRequestSubs int
		blockCh           chan *block.Block
		executionCh       chan *state.AppExecResult
		notificationCh    chan *state.NotificationEvent
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
	"getnep17balances":       (*Server).getNEP17Balances,
	"getnep17transfers":      (*Server).getNEP17Transfers,
	"getpeers":               (*Server).getPeers,
	"getproof":               (*Server).getProof,
	"getrawmempool":          (*Server).getRawMempool,
	"getrawtransaction":      (*Server).getrawtransaction,
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
	orc *oracle.Oracle, log *zap.Logger) Server {
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

		subscribers: make(map[*subscriber]bool),
		// These are NOT buffered to preserve original order of events.
		blockCh:         make(chan *block.Block),
		executionCh:     make(chan *state.AppExecResult),
		notificationCh:  make(chan *state.NotificationEvent),
		transactionCh:   make(chan *transaction.Transaction),
		notaryRequestCh: make(chan mempoolevent.Event),
	}
}

// Start creates a new JSON-RPC server listening on the configured port. It's
// supposed to be run as a separate goroutine (like http.Server's Serve) and it
// returns its errors via given errChan.
func (s *Server) Start(errChan chan error) {
	if !s.config.Enabled {
		s.log.Info("RPC server is not enabled")
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
				errChan <- err
				return
			}
			s.https.Addr = ln.Addr().String()
			err = s.https.ServeTLS(ln, cfg.CertFile, cfg.KeyFile)
			if err != http.ErrServerClosed {
				s.log.Error("failed to start TLS RPC server", zap.Error(err))
				errChan <- err
			}
		}()
	}
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		errChan <- err
		return
	}
	s.Addr = ln.Addr().String() // set Addr to the actual address
	go func() {
		err = s.Serve(ln)
		if err != http.ErrServerClosed {
			s.log.Error("failed to start RPC server", zap.Error(err))
			errChan <- err
		}
	}()
}

// Shutdown overrides the http.Server Shutdown
// method.
func (s *Server) Shutdown() error {
	var httpsErr error

	// Signal to websocket writer routines and handleSubEvents.
	close(s.shutdown)

	if s.config.TLSConfig.Enabled {
		s.log.Info("shutting down rpc-server (https)", zap.String("endpoint", s.https.Addr))
		httpsErr = s.https.Shutdown(context.Background())
	}

	s.log.Info("shutting down rpc-server", zap.String("endpoint", s.Addr))
	err := s.Server.Shutdown(context.Background())

	// Wait for handleSubEvents to finish.
	<-s.executionCh

	if err == nil {
		return httpsErr
	}
	return err
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
		return s.handleIn(req.In, sub)
	}
	resp := make(response.AbstractBatch, len(req.Batch))
	for i, in := range req.Batch {
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

	reqParams, err := req.Params()
	if err != nil {
		return s.packResponse(req, nil, response.NewInvalidParamsError("Problem parsing request parameters", err))
	}

	s.log.Debug("processing rpc request",
		zap.String("method", req.Method),
		zap.Stringer("params", reqParams))

	incCounter(req.Method)

	resErr = response.NewMethodNotFoundError(fmt.Sprintf("Method '%s' not supported", req.Method), nil)
	handler, ok := rpcHandlers[req.Method]
	if ok {
		res, resErr = handler(s, *reqParams)
	} else if sub != nil {
		handler, ok := rpcWsHandlers[req.Method]
		if ok {
			res, resErr = handler(s, *reqParams, sub)
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
	var hash util.Uint256

	if param == nil {
		return hash, response.ErrInvalidParams
	}

	switch param.Type {
	case request.StringT:
		var err error
		hash, err = param.GetUint256()
		if err != nil {
			return hash, response.ErrInvalidParams
		}
	case request.NumberT:
		num, err := s.blockHeightFromParam(param)
		if err != nil {
			return hash, response.ErrInvalidParams
		}
		hash = s.chain.GetHeaderHash(num)
	default:
		return hash, response.ErrInvalidParams
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

	if reqParams.Value(1).GetBoolean() {
		return result.NewBlock(block, s.chain), nil
	}
	writer := io.NewBufBinWriter()
	block.EncodeBinary(writer.BinWriter)
	return writer.Bytes(), nil
}

func (s *Server) getBlockHash(reqParams request.Params) (interface{}, *response.Error) {
	param := reqParams.ValueWithType(0, request.NumberT)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	num, err := s.blockHeightFromParam(param)
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
	return result.Version{
		Magic:             s.network,
		TCPPort:           port,
		Nonce:             s.coreServer.ID(),
		UserAgent:         s.coreServer.UserAgent,
		StateRootInHeader: s.chain.GetConfig().StateRootInHeader,
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
	verbose := reqParams.Value(0).GetBoolean()
	mp := s.chain.GetMemPool()
	hashList := make([]util.Uint256, 0)
	for _, item := range mp.GetVerifiedTransactions() {
		hashList = append(hashList, item.Hash())
	}
	if !verbose {
		return hashList, nil
	}
	return result.RawMempool{
		Height:   s.chain.BlockHeight(),
		Verified: hashList,
	}, nil
}

func (s *Server) validateAddress(reqParams request.Params) (interface{}, *response.Error) {
	param := reqParams.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	return validateAddress(param.Value), nil
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
			verificationErr := fmt.Sprintf("contract verification for signer #%d failed", i)
			res, respErr := s.runScriptInVM(trigger.Verification, tx.Scripts[i].InvocationScript, signer.Account, tx)
			if respErr != nil && errors.Is(respErr.Cause, core.ErrUnknownVerificationContract) {
				// it's neither a contract-based verification script nor a standard witness attached to
				// the tx, so the user did not provide enough data to calculate fee for that witness =>
				// it's a user error
				return 0, response.NewRPCError(verificationErr, respErr.Cause.Error(), respErr.Cause)
			}
			if respErr != nil {
				return 0, respErr
			}
			if res.State != "HALT" {
				cause := fmt.Errorf("invalid VM state %s due to an error: %s", res.State, res.FaultException)
				return 0, response.NewRPCError(verificationErr, cause.Error(), cause)
			}
			if l := len(res.Stack); l != 1 {
				cause := fmt.Errorf("result stack length should be equal to 1, got %d", l)
				return 0, response.NewRPCError(verificationErr, cause.Error(), cause)
			}
			isOK, err := res.Stack[0].TryBool()
			if err != nil {
				cause := fmt.Errorf("resulting stackitem cannot be converted to Boolean: %w", err)
				return 0, response.NewRPCError(verificationErr, cause.Error(), cause)
			}
			if !isOK {
				cause := errors.New("`verify` method returned `false` on stack")
				return 0, response.NewRPCError(verificationErr, cause.Error(), cause)
			}
			netFee += res.GasConsumed
			size += io.GetVarSize([]byte{}) + // verification script is empty (contract-based witness)
				io.GetVarSize(tx.Scripts[i].InvocationScript) // invocation script might not be empty (args for `verify`)
			continue
		}

		if ef == 0 {
			ef = s.chain.GetPolicer().GetBaseExecFee()
		}
		fee, sizeDelta := fee.Calculate(ef, verificationScript)
		netFee += fee
		size += sizeDelta
	}
	fee := s.chain.GetPolicer().FeePerByte()
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
		trigString := reqParams.ValueWithType(1, request.StringT)
		if trigString == nil {
			return nil, response.ErrInvalidParams
		}
		trig, err = trigger.FromString(trigString.String())
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

func (s *Server) getNEP17Balances(ps request.Params) (interface{}, *response.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	bs := &result.NEP17Balances{
		Address:  address.Uint160ToString(u),
		Balances: []result.NEP17Balance{},
	}
	lastUpdated, err := s.chain.GetNEP17LastUpdated(u)
	if err != nil {
		return nil, response.NewRPCError("Failed to get NEP17 last updated block", err.Error(), err)
	}
	bw := io.NewBufBinWriter()
	for _, h := range s.chain.GetNEP17Contracts() {
		balance, err := s.getNEP17Balance(h, u, bw)
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
		bs.Balances = append(bs.Balances, result.NEP17Balance{
			Asset:       h,
			Amount:      balance.String(),
			LastUpdated: lastUpdated[cs.ID],
		})
	}
	return bs, nil
}

func (s *Server) getNEP17Balance(h util.Uint160, acc util.Uint160, bw *io.BufBinWriter) (*big.Int, error) {
	if bw == nil {
		bw = io.NewBufBinWriter()
	} else {
		bw.Reset()
	}
	emit.AppCall(bw.BinWriter, h, "balanceOf", callflag.ReadStates, acc)
	if bw.Err != nil {
		return nil, fmt.Errorf("failed to create `balanceOf` invocation script: %w", bw.Err)
	}
	script := bw.Bytes()
	tx := &transaction.Transaction{Script: script}
	v := s.chain.GetTestVM(trigger.Application, tx, nil)
	v.GasLimit = core.HeaderVerificationGasLimit
	v.LoadScriptWithFlags(script, callflag.All)
	err := v.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run `balanceOf` for %s: %w", h.StringLE(), err)
	}
	if v.Estack().Len() != 1 {
		return nil, fmt.Errorf("invalid `balanceOf` return values count: expected 1, got %d", v.Estack().Len())
	}
	res, err := v.Estack().Pop().Item().TryInteger()
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

func (s *Server) getNEP17Transfers(ps request.Params) (interface{}, *response.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	start, end, limit, page, err := getTimestampsAndLimit(ps, 1)
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}

	bs := &result.NEP17Transfers{
		Address:  address.Uint160ToString(u),
		Received: []result.NEP17Transfer{},
		Sent:     []result.NEP17Transfer{},
	}
	cache := make(map[int32]util.Uint160)
	var resCount, frameCount int
	err = s.chain.ForEachNEP17Transfer(u, func(tr *state.NEP17Transfer) (bool, error) {
		// Iterating from newest to oldest, not yet reached required
		// time frame, continue looping.
		if tr.Timestamp > end {
			return true, nil
		}
		// Iterating from newest to oldest, moved past required
		// time frame, stop looping.
		if tr.Timestamp < start {
			return false, nil
		}
		frameCount++
		// Using limits, not yet reached required page.
		if limit != 0 && page*limit >= frameCount {
			return true, nil
		}

		h, err := s.getHash(tr.Asset, cache)
		if err != nil {
			return false, err
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
			bs.Received = append(bs.Received, transfer)
		} else {
			transfer.Amount = new(big.Int).Neg(&tr.Amount).String()
			if !tr.To.Equals(util.Uint160{}) {
				transfer.Address = address.Uint160ToString(tr.To)
			}
			bs.Sent = append(bs.Sent, transfer)
		}

		resCount++
		// Using limits, reached limit.
		if limit != 0 && resCount >= limit {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, response.NewInternalServerError("invalid NEP17 transfer log", err)
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
	switch param.Type {
	case request.StringT:
		var err error
		scriptHash, err := param.GetUint160FromHex()
		if err != nil {
			return 0, response.ErrInvalidParams
		}
		cs := s.chain.GetContractState(scriptHash)
		if cs == nil {
			return 0, response.ErrUnknown
		}
		result = cs.ID
	case request.NumberT:
		id, err := param.GetInt()
		if err != nil {
			return 0, response.ErrInvalidParams
		}
		if err := checkInt32(id); err != nil {
			return 0, response.WrapErrorWithData(response.ErrInvalidParams, err)
		}
		result = int32(id)
	default:
		return 0, response.ErrInvalidParams
	}
	return result, nil
}

// getContractScriptHashFromParam returns the contract script hash by hex contract hash, address, id or native contract name.
func (s *Server) contractScriptHashFromParam(param *request.Param) (util.Uint160, *response.Error) {
	var result util.Uint160
	if param == nil {
		return result, response.ErrInvalidParams
	}
	switch param.Type {
	case request.StringT:
		var err error
		result, err = param.GetUint160FromAddressOrHex()
		if err == nil {
			return result, nil
		}
		name, err := param.GetString()
		if err != nil {
			return result, response.ErrInvalidParams
		}
		result, err = s.chain.GetNativeContractScriptHash(name)
		if err != nil {
			return result, response.NewRPCError("Unknown contract: querying by name is supported for native contracts only", "", nil)
		}
	case request.NumberT:
		id, err := param.GetInt()
		if err != nil {
			return result, response.ErrInvalidParams
		}
		if err := checkInt32(id); err != nil {
			return result, response.WrapErrorWithData(response.ErrInvalidParams, err)
		}
		result, err = s.chain.GetContractScriptHash(int32(id))
		if err != nil {
			return result, response.NewRPCError("Unknown contract", "", err)
		}
	default:
		return result, response.ErrInvalidParams
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
	cs := s.chain.GetContractState(sc)
	if cs == nil {
		return nil, response.ErrInvalidParams
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
	height, err := p.GetInt()
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
	if reqParams.Value(1).GetBoolean() {
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
		return nil, response.NewRPCError("unknown transaction", "", nil)
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
	param := reqParams.ValueWithType(0, request.NumberT)
	if param == nil {
		return 0, response.ErrInvalidParams
	}

	num, err := s.blockHeightFromParam(param)
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

	verbose := reqParams.Value(1).GetBoolean()
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
	u, err := ps.ValueWithType(0, request.StringT).GetUint160FromAddressOrHex()
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
	scriptHash, responseErr := s.contractScriptHashFromParam(reqParams.Value(0))
	if responseErr != nil {
		return nil, responseErr
	}
	tx := &transaction.Transaction{}
	checkWitnessHashesIndex := len(reqParams)
	if checkWitnessHashesIndex > 3 {
		signers, _, err := reqParams[3].GetSignersWithWitnesses()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		tx.Signers = signers
		checkWitnessHashesIndex--
	}
	if len(tx.Signers) == 0 {
		tx.Signers = []transaction.Signer{{Account: util.Uint160{}, Scopes: transaction.None}}
	}
	script, err := request.CreateFunctionInvocationScript(scriptHash, reqParams[1].String(), reqParams[2:checkWitnessHashesIndex])
	if err != nil {
		return nil, response.NewInternalServerError("can't create invocation script", err)
	}
	tx.Script = script
	return s.runScriptInVM(trigger.Application, script, util.Uint160{}, tx)
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
		signers, _, err := reqParams[1].GetSignersWithWitnesses()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		tx.Signers = signers
	}
	if len(tx.Signers) == 0 {
		tx.Signers = []transaction.Signer{{Account: util.Uint160{}, Scopes: transaction.None}}
	}
	tx.Script = script
	return s.runScriptInVM(trigger.Application, script, util.Uint160{}, tx)
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

	return s.runScriptInVM(trigger.Verification, invocationScript, scriptHash, tx)
}

// runScriptInVM runs given script in a new test VM and returns the invocation
// result. The script is either a simple script in case of `application` trigger
// witness invocation script in case of `verification` trigger (it pushes `verify`
// arguments on stack before verification). In case of contract verification
// contractScriptHash should be specified.
func (s *Server) runScriptInVM(t trigger.Type, script []byte, contractScriptHash util.Uint160, tx *transaction.Transaction) (*result.Invoke, *response.Error) {
	// When transferring funds, script execution does no auto GAS claim,
	// because it depends on persisting tx height.
	// This is why we provide block here.
	b := block.New(s.stateRootEnabled)
	b.Index = s.chain.BlockHeight() + 1
	hdr, err := s.chain.GetHeader(s.chain.GetHeaderHash(int(s.chain.BlockHeight())))
	if err != nil {
		return nil, response.NewInternalServerError("can't get last block", err)
	}
	b.Timestamp = hdr.Timestamp + uint64(s.chain.GetConfig().SecondsPerBlock*int(time.Second/time.Millisecond))

	vm := s.chain.GetTestVM(t, tx, b)
	vm.GasLimit = int64(s.config.MaxGasInvoke)
	if t == trigger.Verification {
		// We need this special case because witnesses verification is not the simple System.Contract.Call,
		// and we need to define exactly the amount of gas consumed for a contract witness verification.
		gasPolicy := s.chain.GetPolicer().GetMaxVerificationGAS()
		if vm.GasLimit > gasPolicy {
			vm.GasLimit = gasPolicy
		}

		err := s.chain.InitVerificationVM(vm, func(h util.Uint160) (*state.Contract, error) {
			res := s.chain.GetContractState(h)
			if res == nil {
				return nil, fmt.Errorf("unknown contract: %s", h.StringBE())
			}
			return res, nil
		}, contractScriptHash, &transaction.Witness{InvocationScript: script, VerificationScript: []byte{}})
		if err != nil {
			return nil, response.NewInternalServerError("can't prepare verification VM", err)
		}
	} else {
		vm.LoadScriptWithFlags(script, callflag.All)
	}
	err = vm.Run()
	var faultException string
	if err != nil {
		faultException = err.Error()
	}
	return result.NewInvoke(vm, script, faultException, s.config.MaxIteratorResultItems), nil
}

// submitBlock broadcasts a raw block over the NEO network.
func (s *Server) submitBlock(reqParams request.Params) (interface{}, *response.Error) {
	blockBytes, err := reqParams.ValueWithType(0, request.StringT).GetBytesBase64()
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

	if len(ps) < 1 {
		return nil, response.NewInvalidParamsError("not enough parameters", nil)
	}
	bytePayload, err := ps[0].GetBytesBase64()
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
		param, ok := p.Value.(json.RawMessage)
		if !ok {
			return nil, response.ErrInvalidParams
		}
		jd := json.NewDecoder(bytes.NewReader(param))
		jd.DisallowUnknownFields()
		switch event {
		case response.BlockEventID:
			flt := new(request.BlockFilter)
			err = jd.Decode(flt)
			p.Type = request.BlockFilterT
			p.Value = *flt
		case response.TransactionEventID, response.NotaryRequestEventID:
			flt := new(request.TxFilter)
			err = jd.Decode(flt)
			p.Type = request.TxFilterT
			p.Value = *flt
		case response.NotificationEventID:
			flt := new(request.NotificationFilter)
			err = jd.Decode(flt)
			p.Type = request.NotificationFilterT
			p.Value = *flt
		case response.ExecutionEventID:
			flt := new(request.ExecutionFilter)
			err = jd.Decode(flt)
			if err == nil && (flt.State == "HALT" || flt.State == "FAULT") {
				p.Type = request.ExecutionFilterT
				p.Value = *flt
			} else if err == nil {
				err = errors.New("invalid state")
			}
		}
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		filter = p.Value
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
			resp.Payload[0] = &response.NotaryRequestEvent{
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
		return 0, nil
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
		params, err := r.In.Params()
		if err == nil {
			logFields = append(logFields, zap.Any("params", params))
		}
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
func validateAddress(addr interface{}) result.ValidateAddress {
	resp := result.ValidateAddress{Address: addr}
	if addr, ok := addr.(string); ok {
		_, err := address.StringToUint160(addr)
		resp.IsValid = (err == nil)
	}
	return resp
}
