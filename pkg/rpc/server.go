package rpc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		*http.Server
		version string
		chain   core.Blockchainer
	}
)

// NewServer creates a new Server struct.
func NewServer(chain core.Blockchainer, port uint16) Server {
	return Server{
		Server: &http.Server{
			Addr: fmt.Sprintf("127.0.0.1:%d", port),
		},
		version: "2.0",
		chain:   chain,
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

func (s *Server) requestHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		// TODO return error.
		return
	}

	request := NewRequest(req.Body)
	if request.HasError() {
		request.WriteError(w, 500, nil)
		return
	}

	log.WithFields(log.Fields{
		"method": request.Method,
	}).Info("processing rpc request")

	switch request.Method {
	case "getbestblockhash":
		result := s.chain.CurrentBlockHash().String()
		request.WriteResponse(w, result)

	case "getblock":
		blockHash := request.Params()
		if request.HasError() {
			request.WriteError(w, 500, nil)
			break
		}

		hash, err := util.Uint256DecodeString(blockHash.StringValueAt(0))
		if err != nil {
			wrappedError := NewInvalidParmsError(err.Error())
			request.WriteError(w, 500, wrappedError)
			break
		}

		result, err := s.chain.GetBlock(hash)
		if err != nil {
			wrappedError := NewInvalidParmsError(err.Error())
			request.WriteError(w, 500, wrappedError)
			break
		}

		request.WriteResponse(w, result)
	}
}
