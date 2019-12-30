package rpc

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	jsonRPCVersion = "2.0"
)

type (
	// Request represents a standard JSON-RPC 2.0
	// request: http://www.jsonrpc.org/specification#request_object.
	Request struct {
		JSONRPC              string          `json:"jsonrpc"`
		Method               string          `json:"method"`
		RawParams            json.RawMessage `json:"params,omitempty"`
		RawID                json.RawMessage `json:"id,omitempty"`
		enableCORSWorkaround bool
	}

	// Response represents a standard JSON-RPC 2.0
	// response: http://www.jsonrpc.org/specification#response_object.
	Response struct {
		JSONRPC string          `json:"jsonrpc"`
		Result  interface{}     `json:"result,omitempty"`
		Error   *Error          `json:"error,omitempty"`
		ID      json.RawMessage `json:"id,omitempty"`
	}
)

// NewRequest creates a new Request struct.
func NewRequest(corsWorkaround bool) *Request {
	return &Request{
		JSONRPC:              jsonRPCVersion,
		enableCORSWorkaround: corsWorkaround,
	}
}

// DecodeData decodes the given reader into the the request
// struct.
func (r *Request) DecodeData(data io.ReadCloser) error {
	defer data.Close()

	err := json.NewDecoder(data).Decode(r)
	if err != nil {
		return errors.Errorf("error parsing JSON payload: %s", err)
	}

	if r.JSONRPC != jsonRPCVersion {
		return errors.Errorf("invalid version, expected 2.0 got: '%s'", r.JSONRPC)
	}

	return nil
}

// Params takes a slice of any type and attempts to bind
// the params to it.
func (r *Request) Params() (*Params, error) {
	params := Params{}

	err := json.Unmarshal(r.RawParams, &params)
	if err != nil {
		return nil, errors.Errorf("error parsing params field in payload: %s", err)
	}

	return &params, nil
}

// WriteErrorResponse writes an error response to the ResponseWriter.
func (s *Server) WriteErrorResponse(r *Request, w http.ResponseWriter, err error) {
	jsonErr, ok := err.(*Error)
	if !ok {
		jsonErr = NewInternalServerError("Internal server error", err)
	}

	response := Response{
		JSONRPC: r.JSONRPC,
		Error:   jsonErr,
		ID:      r.RawID,
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
	s.writeServerResponse(r, w, response)
}

// WriteResponse encodes the response and writes it to the ResponseWriter.
func (s *Server) WriteResponse(r *Request, w http.ResponseWriter, result interface{}) {
	response := Response{
		JSONRPC: r.JSONRPC,
		Result:  result,
		ID:      r.RawID,
	}

	s.writeServerResponse(r, w, response)
}

func (s *Server) writeServerResponse(r *Request, w http.ResponseWriter, response Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.enableCORSWorkaround {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
	}

	encoder := json.NewEncoder(w)
	err := encoder.Encode(response)

	if err != nil {
		s.log.Error("Error encountered while encoding response",
			zap.String("err", err.Error()),
			zap.String("method", r.Method))
	}
}
