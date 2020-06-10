package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/bigint"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/rpc"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		*http.Server
		chain      blockchainer.Blockchainer
		config     rpc.Config
		coreServer *network.Server
		log        *zap.Logger
		https      *http.Server
		shutdown   chan struct{}

		subsLock         sync.RWMutex
		subscribers      map[*subscriber]bool
		subsGroup        sync.WaitGroup
		blockSubs        int
		executionSubs    int
		notificationSubs int
		transactionSubs  int
		blockCh          chan *block.Block
		executionCh      chan *state.AppExecResult
		notificationCh   chan *state.NotificationEvent
		transactionCh    chan *transaction.Transaction
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
)

var rpcHandlers = map[string]func(*Server, request.Params) (interface{}, *response.Error){
	"getapplicationlog":    (*Server).getApplicationLog,
	"getbestblockhash":     (*Server).getBestBlockHash,
	"getblock":             (*Server).getBlock,
	"getblockcount":        (*Server).getBlockCount,
	"getblockhash":         (*Server).getBlockHash,
	"getblockheader":       (*Server).getBlockHeader,
	"getblocksysfee":       (*Server).getBlockSysFee,
	"getconnectioncount":   (*Server).getConnectionCount,
	"getcontractstate":     (*Server).getContractState,
	"getnep5balances":      (*Server).getNEP5Balances,
	"getnep5transfers":     (*Server).getNEP5Transfers,
	"getpeers":             (*Server).getPeers,
	"getrawmempool":        (*Server).getRawMempool,
	"getrawtransaction":    (*Server).getrawtransaction,
	"getstorage":           (*Server).getStorage,
	"gettransactionheight": (*Server).getTransactionHeight,
	"getunclaimedgas":      (*Server).getUnclaimedGas,
	"getvalidators":        (*Server).getValidators,
	"getversion":           (*Server).getVersion,
	"invokefunction":       (*Server).invokeFunction,
	"invokescript":         (*Server).invokescript,
	"sendrawtransaction":   (*Server).sendrawtransaction,
	"submitblock":          (*Server).submitBlock,
	"validateaddress":      (*Server).validateAddress,
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
func New(chain blockchainer.Blockchainer, conf rpc.Config, coreServer *network.Server, log *zap.Logger) Server {
	httpServer := &http.Server{
		Addr: conf.Address + ":" + strconv.FormatUint(uint64(conf.Port), 10),
	}

	var tlsServer *http.Server
	if cfg := conf.TLSConfig; cfg.Enabled {
		tlsServer = &http.Server{
			Addr: net.JoinHostPort(cfg.Address, strconv.FormatUint(uint64(cfg.Port), 10)),
		}
	}

	return Server{
		Server:     httpServer,
		chain:      chain,
		config:     conf,
		coreServer: coreServer,
		log:        log,
		https:      tlsServer,
		shutdown:   make(chan struct{}),

		subscribers: make(map[*subscriber]bool),
		// These are NOT buffered to preserve original order of events.
		blockCh:        make(chan *block.Block),
		executionCh:    make(chan *state.AppExecResult),
		notificationCh: make(chan *state.NotificationEvent),
		transactionCh:  make(chan *transaction.Transaction),
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
			err := s.https.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
			if err != nil {
				s.log.Error("failed to start TLS RPC server", zap.Error(err))
			}
			errChan <- err
		}()
	}
	err := s.ListenAndServe()
	if err != nil {
		s.log.Error("failed to start RPC server", zap.Error(err))
	}
	errChan <- err
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
	req := request.NewIn()

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
				req,
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
		resChan := make(chan response.Raw)
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
			req,
			w,
			response.NewInvalidParamsError(
				fmt.Sprintf("Invalid method '%s', please retry with 'POST'", httpRequest.Method), nil,
			),
		)
		return
	}

	err := req.DecodeData(httpRequest.Body)
	if err != nil {
		s.writeHTTPErrorResponse(req, w, response.NewParseError("Problem parsing JSON-RPC request body", err))
		return
	}

	resp := s.handleRequest(req, nil)
	s.writeHTTPServerResponse(req, w, resp)
}

func (s *Server) handleRequest(req *request.In, sub *subscriber) response.Raw {
	var res interface{}
	var resErr *response.Error

	reqParams, err := req.Params()
	if err != nil {
		return s.packResponseToRaw(req, nil, response.NewInvalidParamsError("Problem parsing request parameters", err))
	}

	s.log.Debug("processing rpc request",
		zap.String("method", req.Method),
		zap.String("params", fmt.Sprintf("%v", reqParams)))

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
	return s.packResponseToRaw(req, res, resErr)
}

func (s *Server) handleWsWrites(ws *websocket.Conn, resChan <-chan response.Raw, subChan <-chan *websocket.PreparedMessage) {
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
			ws.SetWriteDeadline(time.Now().Add(wsWriteLimit))
			if err := ws.WritePreparedMessage(event); err != nil {
				break eventloop
			}
		case res, ok := <-resChan:
			if !ok {
				break eventloop
			}
			ws.SetWriteDeadline(time.Now().Add(wsWriteLimit))
			if err := ws.WriteJSON(res); err != nil {
				break eventloop
			}
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(wsWriteLimit))
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

func (s *Server) handleWsReads(ws *websocket.Conn, resChan chan<- response.Raw, subscr *subscriber) {
	ws.SetReadLimit(wsReadLimit)
	ws.SetReadDeadline(time.Now().Add(wsPongLimit))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(wsPongLimit)); return nil })
requestloop:
	for {
		req := new(request.In)
		err := ws.ReadJSON(req)
		if err != nil {
			break
		}
		res := s.handleRequest(req, subscr)
		if res.Error != nil {
			s.logRequestError(req, res.Error)
		}
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

func (s *Server) getConnectionCount(_ request.Params) (interface{}, *response.Error) {
	return s.coreServer.PeerCount(), nil
}

func (s *Server) blockHashFromParam(param *request.Param) (util.Uint256, *response.Error) {
	var hash util.Uint256

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
	param, ok := reqParams.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	hash, respErr := s.blockHashFromParam(param)
	if respErr != nil {
		return nil, respErr
	}

	block, err := s.chain.GetBlock(hash)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
	}

	if len(reqParams) == 2 && reqParams[1].Value == 1 {
		return result.NewBlock(block, s.chain), nil
	}
	writer := io.NewBufBinWriter()
	block.EncodeBinary(writer.BinWriter)
	return hex.EncodeToString(writer.Bytes()), nil
}

func (s *Server) getBlockHash(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.ValueWithType(0, request.NumberT)
	if !ok {
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
		TCPPort:   port,
		Nonce:     s.coreServer.ID(),
		UserAgent: s.coreServer.UserAgent,
	}, nil
}

func (s *Server) getPeers(_ request.Params) (interface{}, *response.Error) {
	peers := result.NewGetPeers()
	peers.AddUnconnected(s.coreServer.UnconnectedPeers())
	peers.AddConnected(s.coreServer.ConnectedPeers())
	peers.AddBad(s.coreServer.BadPeers())
	return peers, nil
}

func (s *Server) getRawMempool(_ request.Params) (interface{}, *response.Error) {
	mp := s.chain.GetMemPool()
	hashList := make([]util.Uint256, 0)
	for _, item := range mp.GetVerifiedTransactions() {
		hashList = append(hashList, item.Hash())
	}
	return hashList, nil
}

func (s *Server) validateAddress(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	return validateAddress(param.Value), nil
}

// getApplicationLog returns the contract log based on the specified txid.
func (s *Server) getApplicationLog(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	txHash, err := param.GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	appExecResult, err := s.chain.GetAppExecResult(txHash)
	if err != nil {
		return nil, response.NewRPCError("Unknown transaction", "", nil)
	}

	tx, _, err := s.chain.GetTransaction(txHash)
	if err != nil {
		return nil, response.NewRPCError("Error while getting transaction", "", nil)
	}

	scriptHash := hash.Hash160(tx.Script)

	return result.NewApplicationLog(appExecResult, scriptHash), nil
}

func (s *Server) getNEP5Balances(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	as := s.chain.GetNEP5Balances(u)
	bs := &result.NEP5Balances{
		Address:  address.Uint160ToString(u),
		Balances: []result.NEP5Balance{},
	}
	if as != nil {
		cache := make(map[util.Uint160]int64)
		for h, bal := range as.Trackers {
			dec, err := s.getDecimals(h, cache)
			if err != nil {
				continue
			}
			amount := amountToString(bal.Balance, dec)
			bs.Balances = append(bs.Balances, result.NEP5Balance{
				Asset:       h,
				Amount:      amount,
				LastUpdated: bal.LastUpdatedBlock,
			})
		}
	}
	return bs, nil
}

func (s *Server) getNEP5Transfers(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	bs := &result.NEP5Transfers{
		Address:  address.Uint160ToString(u),
		Received: []result.NEP5Transfer{},
		Sent:     []result.NEP5Transfer{},
	}
	lg := s.chain.GetNEP5TransferLog(u)
	cache := make(map[util.Uint160]int64)
	err = lg.ForEach(func(tr *state.NEP5Transfer) error {
		transfer := result.NEP5Transfer{
			Timestamp: tr.Timestamp,
			Asset:     tr.Asset,
			Index:     tr.Block,
			TxHash:    tr.Tx,
		}
		d, err := s.getDecimals(tr.Asset, cache)
		if err != nil {
			return nil
		}
		if tr.Amount > 0 { // token was received
			transfer.Amount = amountToString(tr.Amount, d)
			if !tr.From.Equals(util.Uint160{}) {
				transfer.Address = address.Uint160ToString(tr.From)
			}
			bs.Received = append(bs.Received, transfer)
			return nil
		}

		transfer.Amount = amountToString(-tr.Amount, d)
		if !tr.From.Equals(util.Uint160{}) {
			transfer.Address = address.Uint160ToString(tr.To)
		}
		bs.Sent = append(bs.Sent, transfer)
		return nil
	})
	if err != nil {
		return nil, response.NewInternalServerError("invalid NEP5 transfer log", err)
	}
	return bs, nil
}

func amountToString(amount int64, decimals int64) string {
	if decimals == 0 {
		return strconv.FormatInt(amount, 10)
	}
	pow := int64(math.Pow10(int(decimals)))
	q := amount / pow
	r := amount % pow
	if r == 0 {
		return strconv.FormatInt(q, 10)
	}
	fs := fmt.Sprintf("%%d.%%0%dd", decimals)
	return fmt.Sprintf(fs, q, r)
}

func (s *Server) getDecimals(h util.Uint160, cache map[util.Uint160]int64) (int64, *response.Error) {
	if d, ok := cache[h]; ok {
		return d, nil
	}
	script, err := request.CreateFunctionInvocationScript(h, request.Params{
		{
			Type:  request.StringT,
			Value: "decimals",
		},
		{
			Type:  request.ArrayT,
			Value: []request.Param{},
		},
	})
	if err != nil {
		return 0, response.NewInternalServerError("Can't create script", err)
	}
	res := s.runScriptInVM(script)
	if res == nil || res.State != "HALT" || len(res.Stack) == 0 {
		return 0, response.NewInternalServerError("execution error", errors.New("no result"))
	}

	var d int64
	switch item := res.Stack[len(res.Stack)-1]; item.Type {
	case smartcontract.IntegerType:
		d = item.Value.(int64)
	case smartcontract.ByteArrayType:
		d = bigint.FromBytes(item.Value.([]byte)).Int64()
	default:
		return 0, response.NewInternalServerError("invalid result", errors.New("not an integer"))
	}
	if d < 0 {
		return 0, response.NewInternalServerError("incorrect result", errors.New("negative result"))
	}
	cache[h] = d
	return d, nil
}

func (s *Server) getStorage(ps request.Params) (interface{}, *response.Error) {
	param, ok := ps.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	scriptHash, err := param.GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	scriptHash = scriptHash.Reverse()

	param, ok = ps.Value(1)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	key, err := param.GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	item := s.chain.GetStorageItem(scriptHash.Reverse(), key)
	if item == nil {
		return nil, nil
	}

	return hex.EncodeToString(item.Value), nil
}

func (s *Server) getrawtransaction(reqParams request.Params) (interface{}, *response.Error) {
	var resultsErr *response.Error
	var results interface{}

	if param0, ok := reqParams.Value(0); !ok {
		return nil, response.ErrInvalidParams
	} else if txHash, err := param0.GetUint256(); err != nil {
		resultsErr = response.ErrInvalidParams
	} else if tx, height, err := s.chain.GetTransaction(txHash); err != nil {
		err = errors.Wrapf(err, "Invalid transaction hash: %s", txHash)
		return nil, response.NewRPCError("Unknown transaction", err.Error(), err)
	} else if len(reqParams) >= 2 {
		_header := s.chain.GetHeaderHash(int(height))
		header, err := s.chain.GetHeader(_header)
		if err != nil {
			resultsErr = response.NewInvalidParamsError(err.Error(), err)
		}

		param1, _ := reqParams.Value(1)
		switch v := param1.Value.(type) {

		case int, float64, bool, string:
			if v == 0 || v == "0" || v == 0.0 || v == false || v == "false" {
				results = hex.EncodeToString(tx.Bytes())
			} else {
				results = result.NewTransactionOutputRaw(tx, header, s.chain)
			}
		default:
			results = result.NewTransactionOutputRaw(tx, header, s.chain)
		}
	} else {
		results = hex.EncodeToString(tx.Bytes())
	}

	return results, resultsErr
}

func (s *Server) getTransactionHeight(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	h, err := p.GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	_, height, err := s.chain.GetTransaction(h)
	if err != nil {
		return nil, response.NewRPCError("unknown transaction", "", nil)
	}

	return height, nil
}

// getContractState returns contract state (contract information, according to the contract script hash).
func (s *Server) getContractState(reqParams request.Params) (interface{}, *response.Error) {
	var results interface{}

	param, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	} else if scriptHash, err := param.GetUint160FromHex(); err != nil {
		return nil, response.ErrInvalidParams
	} else {
		cs := s.chain.GetContractState(scriptHash)
		if cs != nil {
			results = result.NewContractState(cs)
		} else {
			return nil, response.NewRPCError("Unknown contract", "", nil)
		}
	}
	return results, nil
}

// getBlockSysFee returns the system fees of the block, based on the specified index.
func (s *Server) getBlockSysFee(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.ValueWithType(0, request.NumberT)
	if !ok {
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

	var blockSysFee util.Fixed8
	for _, tx := range block.Transactions {
		blockSysFee += tx.SystemFee
	}

	return blockSysFee, nil
}

// getBlockHeader returns the corresponding block header information according to the specified script hash.
func (s *Server) getBlockHeader(reqParams request.Params) (interface{}, *response.Error) {
	var verbose bool

	param, ok := reqParams.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	hash, respErr := s.blockHashFromParam(param)
	if respErr != nil {
		return nil, respErr
	}

	param, ok = reqParams.ValueWithType(1, request.NumberT)
	if ok {
		v, err := param.GetInt()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		verbose = v != 0
	}

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
	return hex.EncodeToString(buf.Bytes()), nil
}

// getUnclaimedGas returns unclaimed GAS amount of the specified address.
func (s *Server) getUnclaimedGas(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	neo, neoHeight := s.chain.GetGoverningTokenBalance(u)
	if neo == 0 {
		return "0", nil
	}
	gas := s.chain.CalculateClaimable(int64(neo), neoHeight, s.chain.BlockHeight()+1) // +1 as in C#, for the next block.
	return strconv.FormatInt(int64(gas), 10), nil                                     // It's not represented as Fixed8 in C#.
}

// getValidators returns the current NEO consensus nodes information and voting status.
func (s *Server) getValidators(_ request.Params) (interface{}, *response.Error) {
	var validators keys.PublicKeys

	validators, err := s.chain.GetValidators()
	if err != nil {
		return nil, response.NewRPCError("can't get validators", "", err)
	}
	enrollments, err := s.chain.GetEnrollments()
	if err != nil {
		return nil, response.NewRPCError("can't get enrollments", "", err)
	}
	var res []result.Validator
	for _, v := range enrollments {
		res = append(res, result.Validator{
			PublicKey: *v.Key,
			Votes:     v.Votes.Int64(),
			Active:    validators.Contains(v.Key),
		})
	}
	return res, nil
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokeFunction(reqParams request.Params) (interface{}, *response.Error) {
	scriptHashHex, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	scriptHash, err := scriptHashHex.GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	script, err := request.CreateFunctionInvocationScript(scriptHash, reqParams[1:])
	if err != nil {
		return nil, response.NewInternalServerError("can't create invocation script", err)
	}
	return s.runScriptInVM(script), nil
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokescript(reqParams request.Params) (interface{}, *response.Error) {
	if len(reqParams) < 1 {
		return nil, response.ErrInvalidParams
	}

	script, err := reqParams[0].GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	return s.runScriptInVM(script), nil
}

// runScriptInVM runs given script in a new test VM and returns the invocation
// result.
func (s *Server) runScriptInVM(script []byte) *result.Invoke {
	vm := s.chain.GetTestVM()
	vm.SetGasLimit(s.config.MaxGasInvoke)
	vm.LoadScript(script)
	_ = vm.Run()
	result := &result.Invoke{
		State:       vm.State(),
		GasConsumed: vm.GasConsumed().String(),
		Script:      hex.EncodeToString(script),
		Stack:       vm.Estack().ToContractParameters(),
	}
	return result
}

// submitBlock broadcasts a raw block over the NEO network.
func (s *Server) submitBlock(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	blockBytes, err := param.GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	b := block.Block{}
	r := io.NewBinReaderFromBuf(blockBytes)
	b.DecodeBinary(r)
	if r.Err != nil {
		return nil, response.ErrInvalidParams
	}
	err = s.chain.AddBlock(&b)
	if err != nil {
		switch err {
		case core.ErrInvalidBlockIndex, core.ErrAlreadyExists:
			return nil, response.ErrAlreadyExists
		default:
			return nil, response.ErrValidationFailed
		}
	}
	return true, nil
}

func (s *Server) sendrawtransaction(reqParams request.Params) (interface{}, *response.Error) {
	var resultsErr *response.Error
	var results interface{}

	if len(reqParams) < 1 {
		return nil, response.ErrInvalidParams
	} else if byteTx, err := reqParams[0].GetBytesHex(); err != nil {
		return nil, response.ErrInvalidParams
	} else {
		tx, err := transaction.NewTransactionFromBytes(byteTx)
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		relayReason := s.coreServer.RelayTxn(tx)
		switch relayReason {
		case network.RelaySucceed:
			results = true
		case network.RelayAlreadyExists:
			resultsErr = response.ErrAlreadyExists
		case network.RelayOutOfMemory:
			resultsErr = response.ErrOutOfMemory
		case network.RelayUnableToVerify:
			resultsErr = response.ErrUnableToVerify
		case network.RelayInvalid:
			resultsErr = response.ErrValidationFailed
		case network.RelayPolicyFail:
			resultsErr = response.ErrPolicyFail
		default:
			resultsErr = response.ErrUnknown
		}
	}

	return results, resultsErr
}

// subscribe handles subscription requests from websocket clients.
func (s *Server) subscribe(reqParams request.Params, sub *subscriber) (interface{}, *response.Error) {
	p, ok := reqParams.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	streamName, err := p.GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	event, err := response.GetEventIDFromString(streamName)
	if err != nil || event == response.MissedEventID {
		return nil, response.ErrInvalidParams
	}
	// Optional filter.
	var filter interface{}
	p, ok = reqParams.Value(1)
	if ok {
		switch event {
		case response.BlockEventID:
			if p.Type != request.BlockFilterT {
				return nil, response.ErrInvalidParams
			}
		case response.TransactionEventID:
			if p.Type != request.TxFilterT {
				return nil, response.ErrInvalidParams
			}
		case response.NotificationEventID:
			if p.Type != request.NotificationFilterT {
				return nil, response.ErrInvalidParams
			}
		case response.ExecutionEventID:
			if p.Type != request.ExecutionFilterT {
				return nil, response.ErrInvalidParams
			}
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
	}
}

// unsubscribe handles unsubscription requests from websocket clients.
func (s *Server) unsubscribe(reqParams request.Params, sub *subscriber) (interface{}, *response.Error) {
	p, ok := reqParams.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	id, err := p.GetInt()
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
			resp.Payload[0] = result.NewApplicationLog(execution, util.Uint160{})
		case notification := <-s.notificationCh:
			resp.Event = response.NotificationEventID
			resp.Payload[0] = result.StateEventToResultNotification(*notification)
		case tx := <-s.transactionCh:
			resp.Event = response.TransactionEventID
			resp.Payload[0] = tx
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
	s.subsLock.Unlock()
drainloop:
	for {
		select {
		case <-s.blockCh:
		case <-s.executionCh:
		case <-s.notificationCh:
		case <-s.transactionCh:
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

func (s *Server) packResponseToRaw(r *request.In, result interface{}, respErr *response.Error) response.Raw {
	resp := response.Raw{
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
		resJSON, err := json.Marshal(result)
		if err != nil {
			s.log.Error("failed to marshal result",
				zap.Error(err),
				zap.String("method", r.Method))
			resp.Error = response.NewInternalServerError("failed to encode result", err)
		} else {
			resp.Result = resJSON
		}
	}
	return resp
}

// logRequestError is a request error logger.
func (s *Server) logRequestError(r *request.In, jsonErr *response.Error) {
	logFields := []zap.Field{
		zap.Error(jsonErr.Cause),
		zap.String("method", r.Method),
	}

	params, err := r.Params()
	if err == nil {
		logFields = append(logFields, zap.Any("params", params))
	}

	s.log.Error("Error encountered with rpc request", logFields...)
}

// writeHTTPErrorResponse writes an error response to the ResponseWriter.
func (s *Server) writeHTTPErrorResponse(r *request.In, w http.ResponseWriter, jsonErr *response.Error) {
	resp := s.packResponseToRaw(r, nil, jsonErr)
	s.writeHTTPServerResponse(r, w, resp)
}

func (s *Server) writeHTTPServerResponse(r *request.In, w http.ResponseWriter, resp response.Raw) {
	// Errors can happen in many places and we can only catch ALL of them here.
	if resp.Error != nil {
		s.logRequestError(r, resp.Error)
		w.WriteHeader(resp.Error.HTTPCode)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if s.config.EnableCORSWorkaround {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
	}

	encoder := json.NewEncoder(w)
	err := encoder.Encode(resp)

	if err != nil {
		s.log.Error("Error encountered while encoding response",
			zap.String("err", err.Error()),
			zap.String("method", r.Method))
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
