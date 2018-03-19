package rpc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network"
	bopResult "github.com/CityOfZion/neo-go/pkg/rpc/result"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		*http.Server
		version string
		chain   core.Blockchainer
		server  *network.Server
	}
)

// NewServer creates a new Server struct.
func NewServer(chain core.Blockchainer, port uint16, server *network.Server) Server {
	return Server{
		Server: &http.Server{
			Addr: fmt.Sprintf(":%d", port),
		},
		version: jsonRPCVersion,
		chain:   chain,
		server:  server,
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
			NewInvalidParmsError(
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

	params, err := req.Params()
	if err != nil {
		req.WriteErrorResponse(w, NewInvalidParmsError("Problem parsing request parameters", err))
		return
	}

	s.methodHandler(w, req, params)
}

func (s *Server) methodHandler(w http.ResponseWriter, req *Request, params *Params) {
	log.WithFields(log.Fields{
		"method": req.Method,
		"params": fmt.Sprintf("%+v", params),
	}).Info("processing rpc request")

	var result interface{}
	var resultErr *Error

	switch req.Method {
	case "getbestblockhash":
		result = s.chain.CurrentBlockHash().String()

	case "getblock":
		var hash util.Uint256
		var err error

		switch params.ValueAt(0) {
		case "string":
			hash, err = util.Uint256DecodeString(params.StringValueAt(0))
			if err != nil {
				resultErr = NewInvalidParmsError("Problem decoding block hash", err)
				break
			}

		case "number":
			hash = s.chain.GetHeaderHash(params.IntValueAt(0))

		default:
			err := errors.New("Unable to parse parameter in position 0, expected either a number or string")
			resultErr = NewInvalidParmsError(err.Error(), err)
			break
		}

		result, err = s.chain.GetBlock(hash)
		if err != nil {
			resultErr = NewInvalidParmsError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
			break
		}

	case "getblockcount":
		result = s.chain.BlockHeight()

	case "getblockhash":
		result = s.chain.GetHeaderHash(params.IntValueAt(0))

	case "getconnectioncount":
		result = s.server.PeerCount()

	case "submitblock":
		if !params.IsTypeOfValueAt("string", 0) {
			err := errors.New("Unable to parse parameter in position 0, expected string")
			resultErr = NewInvalidParmsError(err.Error(), err)
			break
		}

		blockHex := params.StringValueAt(0)
		block := &core.Block{}
		err := block.DecodeBinary(
			strings.NewReader(blockHex),
		)
		if err != nil {
			resultErr = NewInvalidParmsError("Problem decoding raw block data", err)
			break
		}

		err = s.chain.AddBlock(block)
		if err != nil {
			resultErr = NewInternalErrorError("Problem adding block to chain", err)
			break
		}

		result = true

	case "getversion":
		result = bopResult.Version{
			Port:      s.server.ListenTCP,
			Nonce:     s.server.ID(),
			UserAgent: s.server.UserAgent,
		}

	case "getpeers":
		peers := bopResult.NewPeers()
		for _, addr := range s.server.UnconnectedPeers() {
			peers.AddPeer("unconnected", addr)
		}

		for _, addr := range s.server.BadPeers() {
			peers.AddPeer("bad", addr)
		}

		for addr := range s.server.Peers() {
			peers.AddPeer("connected", addr.Endpoint().String())
		}

		result = peers

	case "validateaddress", "getblocksysfee", "getcontractstate", "getrawmempool", "getrawtransaction", "getstorage", "gettxout", "invoke", "invokefunction", "invokescript", "sendrawtransaction", "getaccountstate", "getassetstate":
		result = "TODO"

	default:
		resultErr = NewInternalErrorError(fmt.Sprintf("Method '%s' not supported", req.Method), nil)
	}

	if resultErr != nil {
		req.WriteErrorResponse(w, resultErr)
	}

	if result != nil {
		req.WriteResponse(w, result)
	}
}
