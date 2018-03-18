package rpc

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/rpc/models"
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
			Addr: fmt.Sprintf("127.0.0.1:%d", port),
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
		err := fmt.Errorf("Invalid method '%s', please retry with 'POST'", httpRequest.Method)
		req.WriteErrorResponse(w, NewInvalidParmsError(err))
		return
	}

	err := req.DecodeData(httpRequest.Body)
	if err != nil {
		req.WriteErrorResponse(w, NewParseError())
		return
	}

	params, err := req.Params()
	if err != nil {
		req.WriteErrorResponse(w, NewInvalidParmsError(err))
		return
	}

	log.WithFields(log.Fields{
		"method": req.Method,
		"params": fmt.Sprintf("%+v", params),
	}).Info("processing rpc request")

	switch req.Method {
	case "getaccountstate", "getassetstate":
		req.WriteResponse(w, "TODO")

	case "getbestblockhash":
		result := s.chain.CurrentBlockHash().String()
		req.WriteResponse(w, result)

	case "getblock":
		var hash util.Uint256
		var err error

		if params.IsStringValueAt(0) {
			hash, err = util.Uint256DecodeString(params.StringValueAt(0))
			if err != nil {
				req.WriteErrorResponse(w, NewInvalidParmsError(err))
				break
			}
		} else {
			hash = s.chain.GetHeaderHash(params.IntValueAt(0))
		}

		result, err := s.chain.GetBlock(hash)
		if err != nil {
			req.WriteErrorResponse(w, NewInvalidParmsError(err))
			break
		}

		req.WriteResponse(w, result)

	case "getblockcount":
		result := s.chain.BlockHeight()
		req.WriteResponse(w, result)

	case "getblockhash":
		result := s.chain.GetHeaderHash(params.IntValueAt(0))
		req.WriteResponse(w, result)

	case "getblocksysfee", "getcontractstate", "getrawmempool", "getrawtransaction", "getstorage", "gettxout", "invoke", "invokefunction", "invokescript", "sendrawtransaction":
		req.WriteResponse(w, "TODO")

	case "getconnectioncount":
		result := s.server.PeerCount()
		req.WriteResponse(w, result)

	case "submitblock":
		blockHex := params.StringValueAt(0)
		block := &core.Block{}
		err := block.DecodeBinary(
			strings.NewReader(blockHex),
		)
		if err != nil {
			req.WriteErrorResponse(w, NewInvalidParmsError(err))
			break
		}

		err = s.chain.AddBlock(block)
		if err != nil {
			req.WriteErrorResponse(w, NewInternalErrorError(err))
			break
		}

	case "validateaddress":
		req.WriteResponse(w, "TODO")

	case "getversion":
		version := map[string]interface{}{
			"port":      s.server.ListenTCP,
			"nonce":     s.server.ID(),
			"useragent": s.server.UserAgent,
		}
		req.WriteResponse(w, version)

	case "getpeers":
		peers := models.NewPeers()
		for _, addr := range s.server.UnconnectedNodes() {
			peers.AddPeer("unconnected", addr)
		}

		for _, addr := range s.server.BadNodes() {
			peers.AddPeer("bad", addr)
		}

		for addr := range s.server.Peers() {
			peers.AddPeer("connected", addr.Endpoint().String())
		}
		req.WriteResponse(w, peers)

	default:
		req.WriteErrorResponse(w, fmt.Errorf("Method '%s' not supported", req.Method))
	}
}
