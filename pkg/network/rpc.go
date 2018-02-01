package network

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	rpcPortMainNet = 20332
	rpcPortTestNet = 10332
	rpcVersion     = "2.0"

	// error response messages
	methodNotFound = "Method not found"
	parseError     = "Parse error"
)

// Each NEO node has a set of optional APIs for accessing blockchain
// data and making things easier for development of blockchain apps.
// APIs are provided via JSON-RPC , comm at bottom layer is with http/https protocol.

// listenHTTP creates an ingress bridge from the outside world to the passed
// server, by installing handlers for all the necessary RPCs to the passed mux.
func listenHTTP(s *Server, port int) {
	api := &API{s}
	p := fmt.Sprintf(":%d", port)
	s.logger.Printf("serving RPC on %d", port)
	s.logger.Printf("%s", http.ListenAndServe(p, api))
}

// API serves JSON-RPC.
type API struct {
	s *Server
}

func (s *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Official nodes respond a parse error if the method is not POST.
	// Instead of returning a decent response for this, let's do the same.
	if r.Method != "POST" {
		writeError(w, 0, 0, parseError)
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 0, 0, parseError)
		return
	}
	defer r.Body.Close()

	if req.Version != rpcVersion {
		writeJSON(w, http.StatusBadRequest, nil)
		return
	}

	switch req.Method {
	case "getconnectioncount":
		if err := s.getConnectionCount(w, &req); err != nil {
			writeError(w, 0, 0, parseError)
			return
		}
	case "getblockcount":
	case "getbestblockhash":
	default:
		writeError(w, 0, 0, methodNotFound)
	}
}

// This is an Example on how we could handle incomming RPC requests.
func (s *API) getConnectionCount(w http.ResponseWriter, req *Request) error {
	count := s.s.peerCount()

	resp := ConnectionCountResponse{
		Version: rpcVersion,
		Result:  count,
		ID:      1,
	}

	return writeJSON(w, http.StatusOK, resp)
}

// writeError returns a JSON error with given parameters. All error HTTP
// status codes are 200. According to the official API.
func writeError(w http.ResponseWriter, id, code int, msg string) error {
	resp := RequestError{
		Version: rpcVersion,
		ID:      id,
		Error: Error{
			Code:    code,
			Message: msg,
		},
	}

	return writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// Request is an object received through JSON-RPC from the client.
type Request struct {
	Version string   `json:"jsonrpc"`
	Method  string   `json:"method"`
	Params  []string `json:"params"`
	ID      int      `json:"id"`
}

// ConnectionCountResponse ..
type ConnectionCountResponse struct {
	Version string `json:"jsonrpc"`
	Result  int    `json:"result"`
	ID      int    `json:"id"`
}

// RequestError ..
type RequestError struct {
	Version string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Error   Error  `json:"error"`
}

// Error holds information about an RCP error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
