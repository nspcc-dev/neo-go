package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"

	"github.com/nspcc-dev/neo-go/config"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		*http.Server
		chain      core.Blockchainer
		config     config.RPCConfig
		coreServer *network.Server
		log        *zap.Logger
	}
)

var invalidBlockHeightError = func(index int, height int) error {
	return errors.Errorf("Param at index %d should be greater than or equal to 0 and less then or equal to current block height, got: %d", index, height)
}

// New creates a new Server struct.
func New(chain core.Blockchainer, conf config.RPCConfig, coreServer *network.Server, log *zap.Logger) Server {
	httpServer := &http.Server{
		Addr: conf.Address + ":" + strconv.FormatUint(uint64(conf.Port), 10),
	}

	return Server{
		Server:     httpServer,
		chain:      chain,
		config:     conf,
		coreServer: coreServer,
		log:        log,
	}
}

// Start creates a new JSON-RPC server
// listening on the configured port.
func (s *Server) Start(errChan chan error) {
	if !s.config.Enabled {
		s.log.Info("RPC server is not enabled")
		return
	}
	s.Handler = http.HandlerFunc(s.requestHandler)
	s.log.Info("starting rpc-server", zap.String("endpoint", s.Addr))

	errChan <- s.ListenAndServe()
}

// Shutdown overrides the http.Server Shutdown
// method.
func (s *Server) Shutdown() error {
	s.log.Info("shutting down rpc-server", zap.String("endpoint", s.Addr))
	return s.Server.Shutdown(context.Background())
}

func (s *Server) requestHandler(w http.ResponseWriter, httpRequest *http.Request) {
	req := request.NewIn()

	if httpRequest.Method != "POST" {
		s.WriteErrorResponse(
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
		s.WriteErrorResponse(req, w, response.NewParseError("Problem parsing JSON-RPC request body", err))
		return
	}

	reqParams, err := req.Params()
	if err != nil {
		s.WriteErrorResponse(req, w, response.NewInvalidParamsError("Problem parsing request parameters", err))
		return
	}

	s.methodHandler(w, req, *reqParams)
}

func (s *Server) methodHandler(w http.ResponseWriter, req *request.In, reqParams request.Params) {
	s.log.Debug("processing rpc request",
		zap.String("method", req.Method),
		zap.String("params", fmt.Sprintf("%v", reqParams)))

	var (
		results    interface{}
		resultsErr error
	)

Methods:
	switch req.Method {
	case "getapplicationlog":
		getapplicationlogCalled.Inc()
		results, resultsErr = s.getApplicationLog(reqParams)

	case "getbestblockhash":
		getbestblockhashCalled.Inc()
		results = "0x" + s.chain.CurrentBlockHash().StringLE()

	case "getblock":
		getbestblockCalled.Inc()
		var hash util.Uint256

		param, ok := reqParams.Value(0)
		if !ok {
			resultsErr = response.ErrInvalidParams
			break Methods
		}

		switch param.Type {
		case request.StringT:
			var err error
			hash, err = param.GetUint256()
			if err != nil {
				resultsErr = response.ErrInvalidParams
				break Methods
			}
		case request.NumberT:
			num, err := s.blockHeightFromParam(param)
			if err != nil {
				resultsErr = response.ErrInvalidParams
				break Methods
			}
			hash = s.chain.GetHeaderHash(num)
		default:
			resultsErr = response.ErrInvalidParams
			break Methods
		}

		block, err := s.chain.GetBlock(hash)
		if err != nil {
			resultsErr = response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
			break
		}

		if len(reqParams) == 2 && reqParams[1].Value == 1 {
			results = result.NewBlock(block, s.chain)
		} else {
			writer := io.NewBufBinWriter()
			block.EncodeBinary(writer.BinWriter)
			results = hex.EncodeToString(writer.Bytes())
		}

	case "getblockcount":
		getblockcountCalled.Inc()
		results = s.chain.BlockHeight() + 1

	case "getblockhash":
		getblockHashCalled.Inc()
		param, ok := reqParams.ValueWithType(0, request.NumberT)
		if !ok {
			resultsErr = response.ErrInvalidParams
			break Methods
		}
		num, err := s.blockHeightFromParam(param)
		if err != nil {
			resultsErr = response.ErrInvalidParams
			break Methods
		}

		results = s.chain.GetHeaderHash(num)

	case "getblockheader":
		getblockheaderCalled.Inc()
		results, resultsErr = s.getBlockHeader(reqParams)

	case "getblocksysfee":
		getblocksysfeeCalled.Inc()
		results, resultsErr = s.getBlockSysFee(reqParams)

	case "getclaimable":
		getclaimableCalled.Inc()
		results, resultsErr = s.getClaimable(reqParams)

	case "getconnectioncount":
		getconnectioncountCalled.Inc()
		results = s.coreServer.PeerCount()

	case "getnep5balances":
		getnep5balancesCalled.Inc()
		results, resultsErr = s.getNEP5Balances(reqParams)

	case "getnep5transfers":
		getnep5transfersCalled.Inc()
		results, resultsErr = s.getNEP5Transfers(reqParams)

	case "getversion":
		getversionCalled.Inc()
		results = result.Version{
			Port:      s.coreServer.Port,
			Nonce:     s.coreServer.ID(),
			UserAgent: s.coreServer.UserAgent,
		}

	case "getpeers":
		getpeersCalled.Inc()
		peers := result.NewGetPeers()
		peers.AddUnconnected(s.coreServer.UnconnectedPeers())
		peers.AddConnected(s.coreServer.ConnectedPeers())
		peers.AddBad(s.coreServer.BadPeers())
		results = peers

	case "getrawmempool":
		getrawmempoolCalled.Inc()
		mp := s.chain.GetMemPool()
		hashList := make([]util.Uint256, 0)
		for _, item := range mp.GetVerifiedTransactions() {
			hashList = append(hashList, item.Tx.Hash())
		}
		results = hashList

	case "getstorage":
		getstorageCalled.Inc()
		results, resultsErr = s.getStorage(reqParams)

	case "validateaddress":
		validateaddressCalled.Inc()
		param, ok := reqParams.Value(0)
		if !ok {
			resultsErr = response.ErrInvalidParams
			break Methods
		}
		results = validateAddress(param.Value)

	case "getassetstate":
		getassetstateCalled.Inc()
		param, ok := reqParams.ValueWithType(0, request.StringT)
		if !ok {
			resultsErr = response.ErrInvalidParams
			break Methods
		}

		paramAssetID, err := param.GetUint256()
		if err != nil {
			resultsErr = response.ErrInvalidParams
			break
		}

		as := s.chain.GetAssetState(paramAssetID)
		if as != nil {
			results = result.NewAssetState(as)
		} else {
			resultsErr = response.NewRPCError("Unknown asset", "", nil)
		}

	case "getaccountstate":
		getaccountstateCalled.Inc()
		results, resultsErr = s.getAccountState(reqParams, false)

	case "getcontractstate":
		getcontractstateCalled.Inc()
		results, resultsErr = s.getContractState(reqParams)

	case "getrawtransaction":
		getrawtransactionCalled.Inc()
		results, resultsErr = s.getrawtransaction(reqParams)

	case "gettxout":
		gettxoutCalled.Inc()
		results, resultsErr = s.getTxOut(reqParams)

	case "getunspents":
		getunspentsCalled.Inc()
		results, resultsErr = s.getAccountState(reqParams, true)

	case "invoke":
		results, resultsErr = s.invoke(reqParams)

	case "invokefunction":
		results, resultsErr = s.invokeFunction(reqParams)

	case "invokescript":
		results, resultsErr = s.invokescript(reqParams)

	case "sendrawtransaction":
		sendrawtransactionCalled.Inc()
		results, resultsErr = s.sendrawtransaction(reqParams)

	default:
		resultsErr = response.NewMethodNotFoundError(fmt.Sprintf("Method '%s' not supported", req.Method), nil)
	}

	if resultsErr != nil {
		s.WriteErrorResponse(req, w, resultsErr)
		return
	}

	s.WriteResponse(req, w, results)
}

// getApplicationLog returns the contract log based on the specified txid.
func (s *Server) getApplicationLog(reqParams request.Params) (interface{}, error) {
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

	var scriptHash util.Uint160
	switch t := tx.Data.(type) {
	case *transaction.InvocationTX:
		scriptHash = hash.Hash160(t.Script)
	default:
		return nil, response.NewRPCError("Invalid transaction type", "", nil)
	}

	return result.NewApplicationLog(appExecResult, scriptHash), nil
}

func (s *Server) getClaimable(ps request.Params) (interface{}, error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	var unclaimed []state.UnclaimedBalance
	if acc := s.chain.GetAccountState(u); acc != nil {
		unclaimed = acc.Unclaimed
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

func (s *Server) getNEP5Balances(ps request.Params) (interface{}, error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	as := s.chain.GetAccountState(u)
	bs := &result.NEP5Balances{Address: address.Uint160ToString(u)}
	if as != nil {
		cache := make(map[util.Uint160]int64)
		for h, bal := range as.NEP5Balances {
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

func (s *Server) getNEP5Transfers(ps request.Params) (interface{}, error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	bs := &result.NEP5Transfers{Address: address.Uint160ToString(u)}
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

func (s *Server) getDecimals(h util.Uint160, cache map[util.Uint160]int64) (int64, error) {
	if d, ok := cache[h]; ok {
		return d, nil
	}
	w := io.NewBufBinWriter()
	emit.Int(w.BinWriter, 0)
	emit.Opcode(w.BinWriter, opcode.NEWARRAY)
	emit.String(w.BinWriter, "decimals")
	emit.AppCall(w.BinWriter, h, true)
	v, _ := s.chain.GetTestVM()
	v.LoadScript(w.Bytes())
	if err := v.Run(); err != nil {
		return 0, err
	}
	res := v.PopResult()
	if res == nil {
		return 0, errors.New("invalid result")
	}
	bi, ok := res.(*big.Int)
	if !ok {
		bs, ok := res.([]byte)
		if !ok {
			return 0, errors.New("invalid result")
		}
		bi = emit.BytesToInt(bs)
	}
	d := bi.Int64()
	if d < 0 {
		return 0, errors.New("negative decimals")
	}
	cache[h] = d
	return d, nil
}

func (s *Server) getStorage(ps request.Params) (interface{}, error) {
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

func (s *Server) getrawtransaction(reqParams request.Params) (interface{}, error) {
	var resultsErr error
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

func (s *Server) getTxOut(ps request.Params) (interface{}, error) {
	p, ok := ps.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	h, err := p.GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	p, ok = ps.ValueWithType(1, request.NumberT)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	num, err := p.GetInt()
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
func (s *Server) getContractState(reqParams request.Params) (interface{}, error) {
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

// getAccountState returns account state either in short or full (unspents included) form.
func (s *Server) getAccountState(reqParams request.Params, unspents bool) (interface{}, error) {
	var resultsErr error
	var results interface{}

	param, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	} else if scriptHash, err := param.GetUint160FromAddress(); err != nil {
		return nil, response.ErrInvalidParams
	} else {
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
	}
	return results, resultsErr
}

// getBlockSysFee returns the system fees of the block, based on the specified index.
func (s *Server) getBlockSysFee(reqParams request.Params) (util.Fixed8, error) {
	param, ok := reqParams.ValueWithType(0, request.NumberT)
	if !ok {
		return 0, response.ErrInvalidParams
	}

	num, err := s.blockHeightFromParam(param)
	if err != nil {
		return 0, response.NewRPCError("Invalid height", "", nil)
	}

	headerHash := s.chain.GetHeaderHash(num)
	block, err := s.chain.GetBlock(headerHash)
	if err != nil {
		return 0, response.NewRPCError(err.Error(), "", nil)
	}

	var blockSysFee util.Fixed8
	for _, tx := range block.Transactions {
		blockSysFee += s.chain.SystemFee(tx)
	}

	return blockSysFee, nil
}

// getBlockHeader returns the corresponding block header information according to the specified script hash.
func (s *Server) getBlockHeader(reqParams request.Params) (interface{}, error) {
	var verbose bool

	param, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	hash, err := param.GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
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
		return nil, err
	}
	return hex.EncodeToString(buf.Bytes()), nil
}

// invoke implements the `invoke` RPC call.
func (s *Server) invoke(reqParams request.Params) (interface{}, error) {
	scriptHashHex, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	scriptHash, err := scriptHashHex.GetUint160FromHex()
	if err != nil {
		return nil, err
	}
	sliceP, ok := reqParams.ValueWithType(1, request.ArrayT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	slice, err := sliceP.GetArray()
	if err != nil {
		return nil, err
	}
	script, err := request.CreateInvocationScript(scriptHash, slice)
	if err != nil {
		return nil, err
	}
	return s.runScriptInVM(script), nil
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokeFunction(reqParams request.Params) (interface{}, error) {
	scriptHashHex, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	scriptHash, err := scriptHashHex.GetUint160FromHex()
	if err != nil {
		return nil, err
	}
	script, err := request.CreateFunctionInvocationScript(scriptHash, reqParams[1:])
	if err != nil {
		return nil, err
	}
	return s.runScriptInVM(script), nil
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokescript(reqParams request.Params) (interface{}, error) {
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
	vm, _ := s.chain.GetTestVM()
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

func (s *Server) sendrawtransaction(reqParams request.Params) (interface{}, error) {
	var resultsErr error
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
			err = errors.Wrap(r.Err, "transaction DecodeBinary failed")
		} else {
			relayReason := s.coreServer.RelayTxn(tx)
			switch relayReason {
			case network.RelaySucceed:
				results = true
			case network.RelayAlreadyExists:
				err = errors.New("block or transaction already exists and cannot be sent repeatedly")
			case network.RelayOutOfMemory:
				err = errors.New("the memory pool is full and no more transactions can be sent")
			case network.RelayUnableToVerify:
				err = errors.New("the block cannot be validated")
			case network.RelayInvalid:
				err = errors.New("block or transaction validation failed")
			case network.RelayPolicyFail:
				err = errors.New("one of the Policy filters failed")
			default:
				err = errors.New("unknown error")
			}
		}
		if err != nil {
			resultsErr = response.NewInternalServerError(err.Error(), err)
		}
	}

	return results, resultsErr
}

func (s *Server) blockHeightFromParam(param *request.Param) (int, error) {
	num, err := param.GetInt()
	if err != nil {
		return 0, nil
	}

	if num < 0 || num > int(s.chain.BlockHeight()) {
		return 0, invalidBlockHeightError(0, num)
	}
	return num, nil
}

// WriteErrorResponse writes an error response to the ResponseWriter.
func (s *Server) WriteErrorResponse(r *request.In, w http.ResponseWriter, err error) {
	jsonErr, ok := err.(*response.Error)
	if !ok {
		jsonErr = response.NewInternalServerError("Internal server error", err)
	}

	resp := response.Raw{
		HeaderAndError: response.HeaderAndError{
			Header: response.Header{
				JSONRPC: r.JSONRPC,
				ID:      r.RawID,
			},
			Error: jsonErr,
		},
	}

	logFields := []zap.Field{
		zap.Error(jsonErr.Cause),
		zap.String("method", r.Method),
	}

	params, err := r.Params()
	if err == nil {
		logFields = append(logFields, zap.Any("params", params))
	}

	s.log.Error("Error encountered with rpc request", logFields...)

	w.WriteHeader(jsonErr.HTTPCode)
	s.writeServerResponse(r, w, resp)
}

// WriteResponse encodes the response and writes it to the ResponseWriter.
func (s *Server) WriteResponse(r *request.In, w http.ResponseWriter, result interface{}) {
	resJSON, err := json.Marshal(result)
	if err != nil {
		s.log.Error("Error encountered while encoding response",
			zap.String("err", err.Error()),
			zap.String("method", r.Method))
		return
	}

	resp := response.Raw{
		HeaderAndError: response.HeaderAndError{
			Header: response.Header{
				JSONRPC: r.JSONRPC,
				ID:      r.RawID,
			},
		},
		Result: resJSON,
	}

	s.writeServerResponse(r, w, resp)
}

func (s *Server) writeServerResponse(r *request.In, w http.ResponseWriter, resp response.Raw) {
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
