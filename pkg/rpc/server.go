package rpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/rpc/result"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		*http.Server
		version    string
		chain      core.Blockchainer
		coreServer *network.Server
	}
)

// NewServer creates a new Server struct.
func NewServer(chain core.Blockchainer, port uint16, coreServer *network.Server) Server {
	return Server{
		Server: &http.Server{
			Addr: fmt.Sprintf(":%d", port),
		},
		version:    jsonRPCVersion,
		chain:      chain,
		coreServer: coreServer,
	}
}

// Start creates a new JSON-RPC server
// listening on the configured port.
func (s *Server) Start(errChan chan error) {
	s.Handler = http.HandlerFunc(s.requestHandler)
	log.WithFields(log.Fields{
		"endpoint": s.Addr,
	}).Info("starting rpc-server")

	errChan <- s.ListenAndServe()
}

// Shutdown overrride the http.Server Shutdown
// method.
func (s *Server) Shutdown() error {
	log.WithFields(log.Fields{
		"endpoint": s.Addr,
	}).Info("shutting down rpc-server")
	return s.Server.Shutdown(context.Background())
}

func (s *Server) requestHandler(w http.ResponseWriter, httpRequest *http.Request) {
	req := NewRequest()

	if httpRequest.Method != "POST" {
		req.WriteErrorResponse(
			w,
			NewInvalidParamsError(
				fmt.Sprintf("Invalid method '%s', please retry with 'POST'", httpRequest.Method), nil,
			),
		)
		return
	}

	err := req.DecodeData(httpRequest.Body)
	if err != nil {
		req.WriteErrorResponse(w, NewParseError("Problem parsing JSON-RPC request body", err))
		return
	}

	reqParams, err := req.Params()
	if err != nil {
		req.WriteErrorResponse(w, NewInvalidParamsError("Problem parsing request parameters", err))
		return
	}

	s.methodHandler(w, req, *reqParams)
}

func (s *Server) methodHandler(w http.ResponseWriter, req *Request, reqParams Params) {
	log.WithFields(log.Fields{
		"method": req.Method,
		"params": fmt.Sprintf("%v", reqParams),
	}).Info("processing rpc request")

	var results interface{}
	var resultsErr *Error

	switch req.Method {
	case "getbestblockhash":
		results = s.chain.CurrentBlockHash().String()

	case "getblock":
		var hash util.Uint256
		var err error

		param, exists := reqParams.ValueAt(0)
		if !exists {
			err = errors.New("Param at index at 0 doesn't exist")
			resultsErr = NewInvalidParamsError(err.Error(), err)
			break
		}

		switch param.Type {
		case "string":
			hash, err = util.Uint256DecodeString(param.StringVal)
			if err != nil {
				resultsErr = NewInvalidParamsError("Problem decoding block hash", err)
				break
			}
		case "number":
			hash = s.chain.GetHeaderHash(param.IntVal)
		case "default":
			err = errors.New("Expected param at index 0 to be either string or number")
			resultsErr = NewInvalidParamsError(err.Error(), err)
			break
		}

		results, err = s.chain.GetBlock(hash)
		if err != nil {
			resultsErr = NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
			break
		}

	case "getblockcount":
		results = s.chain.BlockHeight()

	case "getblockhash":
		if param, exists := reqParams.GetValueAt(0, "number"); exists {
			results = s.chain.GetHeaderHash(param.IntVal)
		} else {
			err := errors.New("Unable to parse parameter in position 0, expected a number")
			resultsErr = NewInvalidParamsError(err.Error(), err)
			break
		}

	case "getconnectioncount":
		results = s.coreServer.PeerCount()

	case "submitblock":
		if param, exists := reqParams.GetValueAt(0, "string"); exists {
			block := &core.Block{}
			err := block.DecodeBinary(
				strings.NewReader(param.StringVal),
			)
			if err != nil {
				resultsErr = NewInvalidParamsError("Problem decoding raw block data", err)
				break
			}

			err = s.chain.AddBlock(block)
			if err != nil {
				resultsErr = NewInternalServerError("Problem adding block to chain", err)
				break
			}

			results = true

		} else {
			err := errors.New("Unable to parse parameter in position 0, expected a string")
			resultsErr = NewInvalidParamsError(err.Error(), err)
			break
		}

	case "getversion":
		results = result.Version{
			Port:      s.coreServer.ListenTCP,
			Nonce:     s.coreServer.ID(),
			UserAgent: s.coreServer.UserAgent,
		}

	case "getpeers":
		peers := result.NewPeers()
		for _, addr := range s.coreServer.UnconnectedPeers() {
			peers.AddPeer("unconnected", addr)
		}

		for _, addr := range s.coreServer.BadPeers() {
			peers.AddPeer("bad", addr)
		}

		for addr := range s.coreServer.Peers() {
			peers.AddPeer("connected", addr.Endpoint().String())
		}

		results = peers

	case "validateaddress", "getblocksysfee", "getcontractstate", "getrawmempool", "getrawtransaction", "getstorage", "gettxout", "invoke", "invokefunction", "invokescript", "sendrawtransaction", "getaccountstate", "getassetstate":
		results = "TODO"

	default:
		resultsErr = NewMethodNotFoundError(fmt.Sprintf("Method '%s' not supported", req.Method), nil)
	}

	if resultsErr != nil {
		req.WriteErrorResponse(w, resultsErr)
	}

	if results != nil {
		req.WriteResponse(w, results)
	}
}
