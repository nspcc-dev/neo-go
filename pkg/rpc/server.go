package rpc

import (
	"fmt"
	"net/http"
	"os"

	"github.com/CityOfZion/neo-go/pkg/core"
	log "github.com/go-kit/kit/log"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		port       string
		version    string
		blockchain *core.Blockchain
		logger     log.Logger
	}
)

// NewServer creates a new Server struct.
func NewServer(blockchain *core.Blockchain, port string) Server {
	logger := log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "component", "rpc-server")

	return Server{
		port:       port,
		logger:     logger,
		blockchain: blockchain,
	}
}

// Start creates a new JSON-RPC server
// listening on the configured port.
func (s Server) Start() error {
	s.logger.Log("msg", "started", "endpoint", fmt.Sprintf("0.0.0.0:%s", s.port))
	http.HandleFunc("/", s.requestHandler)
	return http.ListenAndServe(fmt.Sprintf(":%s", s.port), nil)
}

func (s Server) requestHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		//s.writeError(w, 405)
		return
	}

	request := NewRequest(req.Body)
	if request.HasError() {
		request.WriteError(w, 500, nil)
		return
	}

	s.logger.Log("msg", "processing rpc request", "method", request.Method)

	switch request.Method {
	case "getbestblockhash":
		result := s.blockchain.CurrentBlockHash().String()
		request.WriteResponse(w, result)
	}
}
