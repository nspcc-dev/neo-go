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
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/rpc"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		*http.Server
		chain      core.Blockchainer
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

	// Maximum number of elements for get*transfers requests.
	maxTransfersLimit = 1000
)

var rpcHandlers = map[string]func(*Server, request.Params) (interface{}, *response.Error){
	"getaccountstate":      (*Server).getAccountState,
	"getalltransfertx":     (*Server).getAllTransferTx,
	"getapplicationlog":    (*Server).getApplicationLog,
	"getassetstate":        (*Server).getAssetState,
	"getbestblockhash":     (*Server).getBestBlockHash,
	"getblock":             (*Server).getBlock,
	"getblockcount":        (*Server).getBlockCount,
	"getblockhash":         (*Server).getBlockHash,
	"getblockheader":       (*Server).getBlockHeader,
	"getblocksysfee":       (*Server).getBlockSysFee,
	"getblocktransfertx":   (*Server).getBlockTransferTx,
	"getclaimable":         (*Server).getClaimable,
	"getconnectioncount":   (*Server).getConnectionCount,
	"getcontractstate":     (*Server).getContractState,
	"getminimumnetworkfee": (*Server).getMinimumNetworkFee,
	"getnep5balances":      (*Server).getNEP5Balances,
	"getnep5transfers":     (*Server).getNEP5Transfers,
	"getpeers":             (*Server).getPeers,
	"getrawmempool":        (*Server).getRawMempool,
	"getrawtransaction":    (*Server).getrawtransaction,
	"getproof":             (*Server).getProof,
	"getstateheight":       (*Server).getStateHeight,
	"getstateroot":         (*Server).getStateRoot,
	"getstorage":           (*Server).getStorage,
	"gettransactionheight": (*Server).getTransactionHeight,
	"gettxout":             (*Server).getTxOut,
	"getunclaimed":         (*Server).getUnclaimed,
	"getunspents":          (*Server).getUnspents,
	"getvalidators":        (*Server).getValidators,
	"getversion":           (*Server).getVersion,
	"getutxotransfers":     (*Server).getUTXOTransfers,
	"invoke":               (*Server).invoke,
	"invokefunction":       (*Server).invokeFunction,
	"invokescript":         (*Server).invokescript,
	"sendrawtransaction":   (*Server).sendrawtransaction,
	"submitblock":          (*Server).submitBlock,
	"validateaddress":      (*Server).validateAddress,
	"verifyproof":          (*Server).verifyProof,
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
func New(chain core.Blockchainer, conf rpc.Config, coreServer *network.Server, log *zap.Logger) Server {
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
			if err != http.ErrServerClosed {
				s.log.Error("failed to start TLS RPC server", zap.Error(err))
				errChan <- err
			}
		}()
	}
	err := s.ListenAndServe()
	if err != http.ErrServerClosed {
		s.log.Error("failed to start RPC server", zap.Error(err))
		errChan <- err
	}
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
		resChan := make(chan response.AbstractResult) // response.Raw or response.RawBatch
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
	resp := make(response.RawBatch, len(req.Batch))
	for i, in := range req.Batch {
		resp[i] = s.handleIn(&in, sub)
	}
	return resp
}

func (s *Server) handleIn(req *request.In, sub *subscriber) response.Raw {
	var res interface{}
	var resErr *response.Error
	if req.JSONRPC != request.JSONRPCVersion {
		return s.packResponseToRaw(req, nil, response.NewInvalidParamsError("Problem parsing JSON", fmt.Errorf("invalid version, expected 2.0 got: '%s'", req.JSONRPC)))
	}

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

func (s *Server) handleWsReads(ws *websocket.Conn, resChan chan<- response.AbstractResult, subscr *subscriber) {
	ws.SetReadLimit(wsReadLimit)
	ws.SetReadDeadline(time.Now().Add(wsPongLimit))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(wsPongLimit)); return nil })
requestloop:
	for {
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

func (s *Server) getConnectionCount(_ request.Params) (interface{}, *response.Error) {
	return s.coreServer.PeerCount(), nil
}

func (s *Server) getBlockHashFromParam(param *request.Param) (util.Uint256, *response.Error) {
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
	hash, respErr := s.getBlockHashFromParam(reqParams.Value(0))
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
	return hex.EncodeToString(writer.Bytes()), nil
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
	return result.Version{
		Port:      s.coreServer.Port,
		Nonce:     s.coreServer.ID(),
		UserAgent: s.coreServer.UserAgent,
	}, nil
}

func getTimestampsAndLimit(ps request.Params, index int) (uint32, uint32, int, int, error) {
	var start, end uint32
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
		end = uint32(val)
	} else {
		end = uint32(time.Now().Unix())
	}
	if pStart != nil {
		val, err := pStart.GetInt()
		if err != nil {
			return 0, 0, 0, 0, err
		}
		start = uint32(val)
	} else {
		start = uint32(time.Now().Add(-time.Hour * 24 * 7).Unix())
	}
	return start, end, limit, page, nil
}

func getAssetMaps(name string) (map[util.Uint256]*result.AssetUTXO, map[util.Uint256]*result.AssetUTXO, error) {
	sent := make(map[util.Uint256]*result.AssetUTXO)
	recv := make(map[util.Uint256]*result.AssetUTXO)
	name = strings.ToLower(name)
	switch name {
	case "neo", "gas", "":
	default:
		return nil, nil, errors.New("invalid asset")
	}
	if name == "neo" || name == "" {
		sent[core.GoverningTokenID()] = &result.AssetUTXO{
			AssetHash:    core.GoverningTokenID(),
			AssetName:    "NEO",
			Transactions: []result.UTXO{},
		}
		recv[core.GoverningTokenID()] = &result.AssetUTXO{
			AssetHash:    core.GoverningTokenID(),
			AssetName:    "NEO",
			Transactions: []result.UTXO{},
		}
	}
	if name == "gas" || name == "" {
		sent[core.UtilityTokenID()] = &result.AssetUTXO{
			AssetHash:    core.UtilityTokenID(),
			AssetName:    "GAS",
			Transactions: []result.UTXO{},
		}
		recv[core.UtilityTokenID()] = &result.AssetUTXO{
			AssetHash:    core.UtilityTokenID(),
			AssetName:    "GAS",
			Transactions: []result.UTXO{},
		}
	}
	return sent, recv, nil
}

func (s *Server) getUTXOTransfers(ps request.Params) (interface{}, *response.Error) {
	addr, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.NewInvalidParamsError("", err)
	}

	index := 1
	assetName, err := ps.Value(index).GetString()
	if err == nil {
		index++
	}

	start, end, limit, page, err := getTimestampsAndLimit(ps, index)
	if err != nil {
		return nil, response.NewInvalidParamsError("", err)
	}

	sent, recv, err := getAssetMaps(assetName)
	if err != nil {
		return nil, response.NewInvalidParamsError("", err)
	}
	tr := new(state.Transfer)
	var resCount, frameCount int
	err = s.chain.ForEachTransfer(addr, tr, func() (bool, error) {
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
		assetID := core.GoverningTokenID()
		if !tr.IsGoverning {
			assetID = core.UtilityTokenID()
		}
		m := recv
		if tr.IsSent {
			m = sent
		}
		a, ok := m[assetID]
		if ok {
			a.Transactions = append(a.Transactions, result.UTXO{
				Index:     tr.Block,
				Timestamp: tr.Timestamp,
				TxHash:    tr.Tx,
				Amount:    tr.Amount,
			})
			a.TotalAmount += tr.Amount
		}
		resCount++
		// Using limits, reached limit.
		if limit != 0 && resCount >= limit {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, response.NewInternalServerError("", err)
	}

	res := &result.GetUTXO{
		Address:  address.Uint160ToString(addr),
		Sent:     []result.AssetUTXO{},
		Received: []result.AssetUTXO{},
	}
	for _, a := range sent {
		res.Sent = append(res.Sent, *a)
	}
	for _, a := range recv {
		res.Received = append(res.Received, *a)
	}
	return res, nil
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
		hashList = append(hashList, item.Tx.Hash())
	}
	return hashList, nil
}

func (s *Server) validateAddress(reqParams request.Params) (interface{}, *response.Error) {
	param := reqParams.Value(0)
	if param == nil {
		return nil, response.ErrInvalidParams
	}
	return validateAddress(param.Value), nil
}

func (s *Server) getAssetState(reqParams request.Params) (interface{}, *response.Error) {
	paramAssetID, err := reqParams.ValueWithType(0, request.StringT).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	as := s.chain.GetAssetState(paramAssetID)
	if as != nil {
		return result.NewAssetState(as), nil
	}
	return nil, response.NewRPCError("Unknown asset", "", nil)
}

// getApplicationLog returns the contract log based on the specified txid.
func (s *Server) getApplicationLog(reqParams request.Params) (interface{}, *response.Error) {
	txHash, err := reqParams.Value(0).GetUint256()
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

	var scriptHash util.Uint160
	switch t := tx.Data.(type) {
	case *transaction.InvocationTX:
		scriptHash = hash.Hash160(t.Script)
	default:
		return nil, response.NewRPCError("Invalid transaction type", "", nil)
	}

	return result.NewApplicationLog(appExecResult, scriptHash), nil
}

func (s *Server) getClaimable(ps request.Params) (interface{}, *response.Error) {
	p := ps.ValueWithType(0, request.StringT)
	u, err := p.GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	var unclaimed []state.UnclaimedBalance
	if acc := s.chain.GetAccountState(u); acc != nil {
		err := acc.Unclaimed.ForEach(func(b *state.UnclaimedBalance) error {
			unclaimed = append(unclaimed, *b)
			return nil
		})
		if err != nil {
			return nil, response.NewInternalServerError("Unclaimed processing failure", err)
		}
	}

	var sum util.Fixed8
	claimable := make([]result.Claimable, 0, len(unclaimed))
	for _, ub := range unclaimed {
		gen, sys, err := s.chain.CalculateClaimable(ub.Value, ub.Start, ub.End)
		if err != nil {
			s.log.Info("error while calculating claim bonus", zap.Error(err))
			continue
		}

		uc := gen.Add(sys)
		sum += uc

		claimable = append(claimable, result.Claimable{
			Tx:          ub.Tx,
			N:           int(ub.Index),
			Value:       ub.Value,
			StartHeight: ub.Start,
			EndHeight:   ub.End,
			Generated:   gen,
			SysFee:      sys,
			Unclaimed:   uc,
		})
	}

	return result.ClaimableInfo{
		Spents:    claimable,
		Address:   p.String(),
		Unclaimed: sum,
	}, nil
}

func (s *Server) getNEP5Balances(ps request.Params) (interface{}, *response.Error) {
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	as := s.chain.GetNEP5Balances(u)
	bs := &result.NEP5Balances{
		Address:  address.Uint160ToString(u),
		Balances: []result.NEP5Balance{},
	}
	if as != nil {
		for h, bal := range as.Trackers {
			amount := strconv.FormatInt(bal.Balance, 10)
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
	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	start, end, limit, page, err := getTimestampsAndLimit(ps, 1)
	if err != nil {
		return nil, response.NewInvalidParamsError("", err)
	}

	bs := &result.NEP5Transfers{
		Address:  address.Uint160ToString(u),
		Received: []result.NEP5Transfer{},
		Sent:     []result.NEP5Transfer{},
	}
	tr := new(state.NEP5Transfer)
	var resCount, frameCount int
	err = s.chain.ForEachNEP5Transfer(u, tr, func() (bool, error) {
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
		transfer := result.NEP5Transfer{
			Timestamp: tr.Timestamp,
			Asset:     tr.Asset,
			Index:     tr.Block,
			TxHash:    tr.Tx,

			NotifyIndex: tr.Index,
		}
		if tr.Amount > 0 { // token was received
			transfer.Amount = strconv.FormatInt(tr.Amount, 10)
			if !tr.From.Equals(util.Uint160{}) {
				transfer.Address = address.Uint160ToString(tr.From)
			}
			bs.Received = append(bs.Received, transfer)
		} else {
			transfer.Amount = strconv.FormatInt(-tr.Amount, 10)
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
		return nil, response.NewInternalServerError("invalid NEP5 transfer log", err)
	}
	return bs, nil
}

func appendUTXOToTransferTx(transfer *result.TransferTx, tx *transaction.Transaction, chain core.Blockchainer) *response.Error {
	inouts, err := chain.References(tx)
	if err != nil {
		return response.NewInternalServerError("invalid tx", err)
	}
	for _, inout := range inouts {
		var event result.TransferTxEvent

		event.Address = address.Uint160ToString(inout.Out.ScriptHash)
		event.Type = "input"
		event.Value = inout.Out.Amount.String()
		event.Asset = inout.Out.AssetID.StringLE()
		transfer.Elements = append(transfer.Elements, event)
	}
	for _, out := range tx.Outputs {
		var event result.TransferTxEvent

		event.Address = address.Uint160ToString(out.ScriptHash)
		event.Type = "output"
		event.Value = out.Amount.String()
		event.Asset = out.AssetID.StringLE()
		transfer.Elements = append(transfer.Elements, event)
	}
	return nil
}

// uint160ToString converts given hash to address, unless it's zero and an empty
// string is returned then.
func uint160ToString(u util.Uint160) string {
	if u.Equals(util.Uint160{}) {
		return ""
	}
	return address.Uint160ToString(u)
}

func appendNEP5ToTransferTx(transfer *result.TransferTx, nepTr *state.NEP5Transfer) {
	var event result.TransferTxEvent
	event.Asset = nepTr.Asset.StringLE()
	if nepTr.Amount > 0 { // token was received
		event.Value = strconv.FormatInt(nepTr.Amount, 10)
		event.Type = "receive"
		event.Address = uint160ToString(nepTr.From)
	} else {
		event.Value = strconv.FormatInt(-nepTr.Amount, 10)
		event.Type = "send"
		event.Address = uint160ToString(nepTr.To)
	}
	transfer.Events = append(transfer.Events, event)
}

func (s *Server) getAllTransferTx(ps request.Params) (interface{}, *response.Error) {
	var respErr *response.Error

	u, err := ps.Value(0).GetUint160FromAddressOrHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	start, end, limit, page, err := getTimestampsAndLimit(ps, 1)
	if err != nil {
		return nil, response.NewInvalidParamsError("", err)
	}

	var (
		utxoCont = make(chan bool)
		nep5Cont = make(chan bool)
		utxoTrs  = make(chan state.Transfer)
		nep5Trs  = make(chan state.NEP5Transfer)
	)

	go func() {
		tr := new(state.Transfer)
		_ = s.chain.ForEachTransfer(u, tr, func() (bool, error) {
			var cont bool

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
			utxoTrs <- *tr
			cont = <-utxoCont
			return cont, nil
		})
		close(utxoTrs)
	}()

	go func() {
		tr := new(state.NEP5Transfer)
		_ = s.chain.ForEachNEP5Transfer(u, tr, func() (bool, error) {
			var cont bool

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
			nep5Trs <- *tr
			cont = <-nep5Cont
			return cont, nil
		})
		close(nep5Trs)
	}()

	var (
		res                = make([]result.TransferTx, 0, limit)
		frameCount         int
		utxoLast           state.Transfer
		nep5Last           state.NEP5Transfer
		haveUtxo, haveNep5 bool
	)

	utxoLast, haveUtxo = <-utxoTrs
	if haveUtxo {
		utxoCont <- true
	}
	nep5Last, haveNep5 = <-nep5Trs
	if haveNep5 {
		nep5Cont <- true
	}
	for len(res) < limit {
		if !haveUtxo && !haveNep5 {
			break
		}
		var isNep5 = haveNep5 && (!haveUtxo || (nep5Last.Timestamp > utxoLast.Timestamp))
		var transfer result.TransferTx
		if isNep5 {
			transfer.TxID = nep5Last.Tx
			transfer.Timestamp = nep5Last.Timestamp
			transfer.Index = nep5Last.Block
		} else {
			transfer.TxID = utxoLast.Tx
			transfer.Timestamp = utxoLast.Timestamp
			transfer.Index = utxoLast.Block
		}
		frameCount++
		// Using limits, not yet reached required page. But still need
		// to drain inputs for this tx.
		skipTx := page*limit >= frameCount

		if !skipTx {
			tx, _, err := s.chain.GetTransaction(transfer.TxID)
			if err != nil {
				respErr = response.NewInternalServerError("invalid NEP5 transfer log", err)
				break
			}
			transfer.NetworkFee = s.chain.NetworkFee(tx).String()
			transfer.SystemFee = s.chain.SystemFee(tx).String()
			respErr = appendUTXOToTransferTx(&transfer, tx, s.chain)
			if respErr != nil {
				break
			}
		}
		// Pick all NEP5 events for this transaction, if there are any.
		for haveNep5 && nep5Last.Tx.Equals(transfer.TxID) {
			if !skipTx {
				appendNEP5ToTransferTx(&transfer, &nep5Last)
			}
			nep5Last, haveNep5 = <-nep5Trs
			if haveNep5 {
				nep5Cont <- true
			}
		}

		// Skip UTXO events, we've already got them from inputs and outputs.
		for haveUtxo && utxoLast.Tx.Equals(transfer.TxID) {
			utxoLast, haveUtxo = <-utxoTrs
			if haveUtxo {
				utxoCont <- true
			}
		}
		if !skipTx {
			res = append(res, transfer)
		}
	}
	if haveUtxo {
		_, ok := <-utxoTrs
		if ok {
			utxoCont <- false
		}
	}
	if haveNep5 {
		_, ok := <-nep5Trs
		if ok {
			nep5Cont <- false
		}
	}
	if respErr != nil {
		return nil, respErr
	}
	return res, nil
}

func (s *Server) getMinimumNetworkFee(ps request.Params) (interface{}, *response.Error) {
	return s.chain.GetConfig().MinimumNetworkFee, nil
}

func (s *Server) getProof(ps request.Params) (interface{}, *response.Error) {
	root, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	sc, err := ps.Value(1).GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	sc = sc.Reverse()
	key, err := ps.Value(2).GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	skey := mpt.ToNeoStorageKey(append(sc.BytesBE(), key...))
	proof, err := s.chain.GetStateProof(root, skey)
	return &result.GetProof{
		Result: result.ProofWithKey{
			Key:   skey,
			Proof: proof,
		},
		Success: err == nil,
	}, nil
}

func (s *Server) verifyProof(ps request.Params) (interface{}, *response.Error) {
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
		var si state.StorageItem
		r := io.NewBinReaderFromBuf(val[1:])
		si.DecodeBinary(r)
		if r.Err != nil {
			return nil, response.NewInternalServerError("invalid item in trie", r.Err)
		}
		vp.Value = si.Value
	}
	return vp, nil
}

func (s *Server) getStateHeight(_ request.Params) (interface{}, *response.Error) {
	return &result.StateHeight{
		BlockHeight: s.chain.BlockHeight(),
		StateHeight: s.chain.StateHeight(),
	}, nil
}

func (s *Server) getStateRoot(ps request.Params) (interface{}, *response.Error) {
	p := ps.Value(0)
	if p == nil {
		return nil, response.NewRPCError("Invalid parameter.", "", nil)
	}
	var rt *state.MPTRootState
	var h util.Uint256
	height, err := p.GetInt()
	if err == nil {
		rt, err = s.chain.GetStateRoot(uint32(height))
	} else if h, err = p.GetUint256(); err == nil {
		hdr, err := s.chain.GetHeader(h)
		if err == nil {
			rt, err = s.chain.GetStateRoot(hdr.Index)
		}
	}
	if err != nil {
		return nil, response.NewRPCError("Unknown state root.", "", err)
	}
	return rt, nil
}

func (s *Server) getStorage(ps request.Params) (interface{}, *response.Error) {
	scriptHash, err := ps.Value(0).GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	scriptHash = scriptHash.Reverse()

	key, err := ps.Value(1).GetBytesHex()
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
	txHash, err := reqParams.Value(0).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	tx, height, err := s.chain.GetTransaction(txHash)
	if err != nil {
		err = errors.Wrapf(err, "Invalid transaction hash: %s", txHash)
		return nil, response.NewRPCError("Unknown transaction", err.Error(), err)
	}
	if reqParams.Value(1).GetBoolean() {
		if height == math.MaxUint32 {
			return result.NewTransactionOutputRaw(tx, nil, s.chain), nil
		}
		_header := s.chain.GetHeaderHash(int(height))
		header, err := s.chain.GetHeader(_header)
		if err != nil {
			return nil, response.NewInvalidParamsError(err.Error(), err)
		}
		return result.NewTransactionOutputRaw(tx, header, s.chain), nil

	}
	return hex.EncodeToString(tx.Bytes()), nil

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

func (s *Server) getTxOut(ps request.Params) (interface{}, *response.Error) {
	h, err := ps.Value(0).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	num, err := ps.ValueWithType(1, request.NumberT).GetInt()
	if err != nil || num < 0 {
		return nil, response.ErrInvalidParams
	}

	tx, _, err := s.chain.GetTransaction(h)
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}

	if num >= len(tx.Outputs) {
		return nil, response.NewInvalidParamsError("invalid index", errors.New("too big index"))
	}

	out := tx.Outputs[num]
	return result.NewTxOutput(&out), nil
}

// getContractState returns contract state (contract information, according to the contract script hash).
func (s *Server) getContractState(reqParams request.Params) (interface{}, *response.Error) {
	var results interface{}

	scriptHash, err := reqParams.ValueWithType(0, request.StringT).GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	cs := s.chain.GetContractState(scriptHash)
	if cs != nil {
		results = result.NewContractState(cs)
	} else {
		return nil, response.NewRPCError("Unknown contract", "", nil)
	}
	return results, nil
}

func (s *Server) getAccountState(ps request.Params) (interface{}, *response.Error) {
	return s.getAccountStateAux(ps, false)
}

func (s *Server) getUnspents(ps request.Params) (interface{}, *response.Error) {
	return s.getAccountStateAux(ps, true)
}

// getAccountState returns account state either in short or full (unspents included) form.
func (s *Server) getAccountStateAux(reqParams request.Params, unspents bool) (interface{}, *response.Error) {
	var resultsErr *response.Error
	var results interface{}

	param := reqParams.ValueWithType(0, request.StringT)
	scriptHash, err := param.GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	as := s.chain.GetAccountState(scriptHash)
	if as == nil {
		as = state.NewAccount(scriptHash)
	}
	if unspents {
		str, err := param.GetString()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		results = result.NewUnspents(as, s.chain, str)
	} else {
		results = result.NewAccountState(as)
	}
	return results, resultsErr
}

func (s *Server) getBlockTransferTx(ps request.Params) (interface{}, *response.Error) {
	var (
		res     = make([]result.TransferTx, 0)
		respErr *response.Error
	)

	hash, respErr := s.getBlockHashFromParam(ps.Value(0))
	if respErr != nil {
		return nil, respErr
	}

	block, err := s.chain.GetBlock(hash)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
	}

	for _, tx := range block.Transactions {
		var transfer = result.TransferTx{
			TxID:       tx.Hash(),
			Timestamp:  block.Timestamp,
			Index:      block.Index,
			NetworkFee: s.chain.NetworkFee(tx).String(),
			SystemFee:  s.chain.SystemFee(tx).String(),
		}

		respErr = appendUTXOToTransferTx(&transfer, tx, s.chain)
		if respErr != nil {
			break
		}
		if tx.Type == transaction.InvocationType {
			execRes, err := s.chain.GetAppExecResult(tx.Hash())
			if err != nil {
				respErr = response.NewInternalServerError(fmt.Sprintf("no application log for invocation tx %s", tx.Hash()), err)
				break
			}

			if execRes.VMState != "HALT" {
				continue
			}

			var index uint32
			for _, note := range execRes.Events {
				nepTr, err := state.NEP5TransferFromNotification(note, tx.Hash(), block.Index, block.Timestamp, index)
				// It's OK for event to be something different from NEP5 transfer.
				if err != nil {
					continue
				}
				transfer.Events = append(transfer.Events, result.TransferTxEvent{
					Asset: nepTr.Asset.StringLE(),
					From:  uint160ToString(nepTr.From),
					To:    uint160ToString(nepTr.To),
					Value: strconv.FormatInt(nepTr.Amount, 10),
				})
				index++
			}
		}

		if len(transfer.Elements) != 0 || len(transfer.Events) != 0 {
			res = append(res, transfer)
		}
	}
	if respErr != nil {
		return nil, respErr
	}
	return res, nil
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

	var blockSysFee util.Fixed8
	for _, tx := range block.Transactions {
		blockSysFee += s.chain.SystemFee(tx)
	}

	return blockSysFee, nil
}

// getBlockHeader returns the corresponding block header information according to the specified script hash.
func (s *Server) getBlockHeader(reqParams request.Params) (interface{}, *response.Error) {
	hash, err := reqParams.ValueWithType(0, request.StringT).GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
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
	return hex.EncodeToString(buf.Bytes()), nil
}

// getUnclaimed returns unclaimed GAS amount of the specified address.
func (s *Server) getUnclaimed(ps request.Params) (interface{}, *response.Error) {
	u, err := ps.ValueWithType(0, request.StringT).GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	acc := s.chain.GetAccountState(u)
	if acc == nil {
		return nil, response.NewInternalServerError("unknown account", nil)
	}
	res, errRes := result.NewUnclaimed(acc, s.chain)
	if errRes != nil {
		return nil, response.NewInternalServerError("can't create unclaimed response", errRes)
	}
	return res, nil
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
			PublicKey: *v.PublicKey,
			Votes:     v.Votes,
			Active:    validators.Contains(v.PublicKey),
		})
	}
	return res, nil
}

// invoke implements the `invoke` RPC call.
func (s *Server) invoke(reqParams request.Params) (interface{}, *response.Error) {
	scriptHash, err := reqParams.ValueWithType(0, request.StringT).GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	slice, err := reqParams.ValueWithType(1, request.ArrayT).GetArray()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	hashesForVerifying, err := reqParams.ValueWithType(2, request.ArrayT).GetArrayUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	script, err := request.CreateInvocationScript(scriptHash, slice)
	if err != nil {
		return nil, response.NewInternalServerError("can't create invocation script", err)
	}
	return s.runScriptInVM(script, hashesForVerifying), nil
}

// invokeFunction implements the `invokefunction` RPC call.
func (s *Server) invokeFunction(reqParams request.Params) (interface{}, *response.Error) {
	scriptHash, err := reqParams.ValueWithType(0, request.StringT).GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	var hashesForVerifying []util.Uint160
	hashesForVerifyingIndex := len(reqParams)
	if hashesForVerifyingIndex > 3 {
		hashesForVerifying, err = reqParams.ValueWithType(3, request.ArrayT).GetArrayUint160FromHex()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		hashesForVerifyingIndex--
	}
	script, err := request.CreateFunctionInvocationScript(scriptHash, reqParams[1:hashesForVerifyingIndex])
	if err != nil {
		return nil, response.NewInternalServerError("can't create invocation script", err)
	}
	return s.runScriptInVM(script, hashesForVerifying), nil
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

	hashesForVerifying, err := reqParams.ValueWithType(1, request.ArrayT).GetArrayUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	return s.runScriptInVM(script, hashesForVerifying), nil
}

// runScriptInVM runs given script in a new test VM and returns the invocation
// result.
func (s *Server) runScriptInVM(script []byte, scriptHashesForVerifying []util.Uint160) *result.Invoke {
	var tx *transaction.Transaction
	if count := len(scriptHashesForVerifying); count != 0 {
		tx := new(transaction.Transaction)
		tx.Attributes = make([]transaction.Attribute, count)
		for i, a := range tx.Attributes {
			a.Data = scriptHashesForVerifying[i].BytesBE()
			a.Usage = transaction.Script
		}
	}
	vm := s.chain.GetTestVM(tx)
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
	blockBytes, err := reqParams.ValueWithType(0, request.StringT).GetBytesHex()
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
		r := io.NewBinReaderFromBuf(byteTx)
		tx := &transaction.Transaction{}
		tx.DecodeBinary(r)
		if r.Err != nil {
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
	streamName, err := reqParams.Value(0).GetString()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	event, err := response.GetEventIDFromString(streamName)
	if err != nil || event == response.MissedEventID {
		return nil, response.ErrInvalidParams
	}
	// Optional filter.
	var filter interface{}
	if p := reqParams.Value(1); p != nil {
		// It doesn't accept filters.
		if event == response.BlockEventID {
			return nil, response.ErrInvalidParams
		}

		switch event {
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
	resp := s.packResponseToRaw(r, nil, jsonErr)
	s.writeHTTPServerResponse(&request.Request{In: r}, w, resp)
}

func (s *Server) writeHTTPServerResponse(r *request.Request, w http.ResponseWriter, resp response.AbstractResult) {
	// Errors can happen in many places and we can only catch ALL of them here.
	resp.RunForErrors(func(jsonErr *response.Error) {
		s.logRequestError(r, jsonErr)
	})
	if r.In != nil {
		resp := resp.(response.Raw)
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
