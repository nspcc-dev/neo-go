package rpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/rpc/result"
	"github.com/CityOfZion/neo-go/pkg/rpc/wrappers"
	"github.com/CityOfZion/neo-go/pkg/util"
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

// NewServer creates a new Server struct.
func NewServer(chain core.Blockchainer, conf config.RPCConfig, coreServer *network.Server, log *zap.Logger) Server {
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
	req := NewRequest(s.config.EnableCORSWorkaround)

	if httpRequest.Method != "POST" {
		s.WriteErrorResponse(
			req,
			w,
			NewInvalidParamsError(
				fmt.Sprintf("Invalid method '%s', please retry with 'POST'", httpRequest.Method), nil,
			),
		)
		return
	}

	err := req.DecodeData(httpRequest.Body)
	if err != nil {
		s.WriteErrorResponse(req, w, NewParseError("Problem parsing JSON-RPC request body", err))
		return
	}

	reqParams, err := req.Params()
	if err != nil {
		s.WriteErrorResponse(req, w, NewInvalidParamsError("Problem parsing request parameters", err))
		return
	}

	s.methodHandler(w, req, *reqParams)
}

func (s *Server) methodHandler(w http.ResponseWriter, req *Request, reqParams Params) {
	s.log.Info("processing rpc request",
		zap.String("method", req.Method),
		zap.String("params", fmt.Sprintf("%v", reqParams)))

	var (
		results    interface{}
		resultsErr error
	)

Methods:
	switch req.Method {
	case "getbestblockhash":
		getbestblockhashCalled.Inc()
		results = "0x" + s.chain.CurrentBlockHash().StringLE()

	case "getblock":
		getbestblockCalled.Inc()
		var hash util.Uint256

		param, ok := reqParams.Value(0)
		if !ok {
			resultsErr = errInvalidParams
			break Methods
		}

		switch param.Type {
		case stringT:
			var err error
			hash, err = param.GetUint256()
			if err != nil {
				resultsErr = errInvalidParams
				break Methods
			}
		case numberT:
			num, err := s.blockHeightFromParam(param)
			if err != nil {
				resultsErr = errInvalidParams
				break Methods
			}
			hash = s.chain.GetHeaderHash(num)
		default:
			resultsErr = errInvalidParams
			break Methods
		}

		block, err := s.chain.GetBlock(hash)
		if err != nil {
			resultsErr = NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
			break
		}

		if len(reqParams) == 2 && reqParams[1].Value == 1 {
			results = wrappers.NewBlock(block, s.chain)
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
		param, ok := reqParams.ValueWithType(0, numberT)
		if !ok {
			resultsErr = errInvalidParams
			break Methods
		}
		num, err := s.blockHeightFromParam(param)
		if err != nil {
			resultsErr = errInvalidParams
			break Methods
		}

		results = s.chain.GetHeaderHash(num)

	case "getconnectioncount":
		getconnectioncountCalled.Inc()
		results = s.coreServer.PeerCount()

	case "getversion":
		getversionCalled.Inc()
		results = result.Version{
			Port:      s.coreServer.Port,
			Nonce:     s.coreServer.ID(),
			UserAgent: s.coreServer.UserAgent,
		}

	case "getpeers":
		getpeersCalled.Inc()
		peers := result.NewPeers()
		for _, addr := range s.coreServer.UnconnectedPeers() {
			peers.AddPeer("unconnected", addr)
		}

		for _, addr := range s.coreServer.BadPeers() {
			peers.AddPeer("bad", addr)
		}

		for addr := range s.coreServer.Peers() {
			peers.AddPeer("connected", addr.PeerAddr().String())
		}

		results = peers

	case "validateaddress":
		validateaddressCalled.Inc()
		param, ok := reqParams.Value(0)
		if !ok {
			resultsErr = errInvalidParams
			break Methods
		}
		results = wrappers.ValidateAddress(param.Value)

	case "getassetstate":
		getassetstateCalled.Inc()
		param, ok := reqParams.ValueWithType(0, stringT)
		if !ok {
			resultsErr = errInvalidParams
			break Methods
		}

		paramAssetID, err := param.GetUint256()
		if err != nil {
			resultsErr = errInvalidParams
			break
		}

		as := s.chain.GetAssetState(paramAssetID)
		if as != nil {
			results = wrappers.NewAssetState(as)
		} else {
			results = "Invalid assetid"
		}

	case "getaccountstate":
		getaccountstateCalled.Inc()
		results, resultsErr = s.getAccountState(reqParams, false)

	case "getrawtransaction":
		getrawtransactionCalled.Inc()
		results, resultsErr = s.getrawtransaction(reqParams)

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
		resultsErr = NewMethodNotFoundError(fmt.Sprintf("Method '%s' not supported", req.Method), nil)
	}

	if resultsErr != nil {
		s.WriteErrorResponse(req, w, resultsErr)
		return
	}

	s.WriteResponse(req, w, results)
}

func (s *Server) getrawtransaction(reqParams Params) (interface{}, error) {
	var resultsErr error
	var results interface{}

	if param0, ok := reqParams.Value(0); !ok {
		return nil, errInvalidParams
	} else if txHash, err := param0.GetUint256(); err != nil {
		resultsErr = errInvalidParams
	} else if tx, height, err := s.chain.GetTransaction(txHash); err != nil {
		err = errors.Wrapf(err, "Invalid transaction hash: %s", txHash)
		return nil, NewInvalidParamsError(err.Error(), err)
	} else if len(reqParams) >= 2 {
		_header := s.chain.GetHeaderHash(int(height))
		header, err := s.chain.GetHeader(_header)
		if err != nil {
			resultsErr = NewInvalidParamsError(err.Error(), err)
		}

		param1, _ := reqParams.Value(1)
		switch v := param1.Value.(type) {

		case int, float64, bool, string:
			if v == 0 || v == "0" || v == 0.0 || v == false || v == "false" {
				results = hex.EncodeToString(tx.Bytes())
			} else {
				results = wrappers.NewTransactionOutputRaw(tx, header, s.chain)
			}
		default:
			results = wrappers.NewTransactionOutputRaw(tx, header, s.chain)
		}
	} else {
		results = hex.EncodeToString(tx.Bytes())
	}

	return results, resultsErr
}

// getAccountState returns account state either in short or full (unspents included) form.
func (s *Server) getAccountState(reqParams Params, unspents bool) (interface{}, error) {
	var resultsErr error
	var results interface{}

	param, ok := reqParams.ValueWithType(0, stringT)
	if !ok {
		return nil, errInvalidParams
	} else if scriptHash, err := param.GetUint160FromAddress(); err != nil {
		return nil, errInvalidParams
	} else if as := s.chain.GetAccountState(scriptHash); as != nil {
		if unspents {
			str, err := param.GetString()
			if err != nil {
				return nil, errInvalidParams
			}
			results = wrappers.NewUnspents(as, s.chain, str)
		} else {
			results = wrappers.NewAccountState(as)
		}
	} else {
		results = "Invalid public account address"
	}
	return results, resultsErr
}

// invoke implements the `invoke` RPC call.
func (s *Server) invoke(reqParams Params) (interface{}, error) {
	scriptHashHex, ok := reqParams.ValueWithType(0, stringT)
	if !ok {
		return nil, errInvalidParams
	}
	scriptHash, err := scriptHashHex.GetUint160FromHex()
	if err != nil {
		return nil, err
	}
	sliceP, ok := reqParams.ValueWithType(1, arrayT)
	if !ok {
		return nil, errInvalidParams
	}
	slice, err := sliceP.GetArray()
	if err != nil {
		return nil, err
	}
	script, err := CreateInvocationScript(scriptHash, slice)
	if err != nil {
		return nil, err
	}
	return s.runScriptInVM(script), nil
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokeFunction(reqParams Params) (interface{}, error) {
	scriptHashHex, ok := reqParams.ValueWithType(0, stringT)
	if !ok {
		return nil, errInvalidParams
	}
	scriptHash, err := scriptHashHex.GetUint160FromHex()
	if err != nil {
		return nil, err
	}
	script, err := CreateFunctionInvocationScript(scriptHash, reqParams[1:])
	if err != nil {
		return nil, err
	}
	return s.runScriptInVM(script), nil
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokescript(reqParams Params) (interface{}, error) {
	if len(reqParams) < 1 {
		return nil, errInvalidParams
	}

	script, err := reqParams[0].GetBytesHex()
	if err != nil {
		return nil, errInvalidParams
	}

	return s.runScriptInVM(script), nil
}

// runScriptInVM runs given script in a new test VM and returns the invocation
// result.
func (s *Server) runScriptInVM(script []byte) *wrappers.InvokeResult {
	vm, _ := s.chain.GetTestVM()
	vm.LoadScript(script)
	_ = vm.Run()
	result := &wrappers.InvokeResult{
		State:       vm.State(),
		GasConsumed: "0.1",
		Script:      hex.EncodeToString(script),
		Stack:       vm.Estack(),
	}
	return result
}

func (s *Server) sendrawtransaction(reqParams Params) (interface{}, error) {
	var resultsErr error
	var results interface{}

	if len(reqParams) < 1 {
		return nil, errInvalidParams
	} else if byteTx, err := reqParams[0].GetBytesHex(); err != nil {
		return nil, errInvalidParams
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
			resultsErr = NewInternalServerError(err.Error(), err)
		}
	}

	return results, resultsErr
}

func (s Server) blockHeightFromParam(param *Param) (int, error) {
	num, err := param.GetInt()
	if err != nil {
		return 0, nil
	}

	if num < 0 || num > int(s.chain.BlockHeight()) {
		return 0, invalidBlockHeightError(0, num)
	}
	return num, nil
}
