package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
)

const (
	jsonRPCVersion = "2.0"
)

type (
	// Request represents a standard JSON-RPC 2.0
	// request: http://www.jsonrpc.org/specification#request_object.
	Request struct {
		JSONRPC   string          `json:"jsonrpc"`
		Method    string          `json:"method"`
		RawParams json.RawMessage `json:"params,omitempty"`
		RawID     json.RawMessage `json:"id,omitempty"`
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
func NewRequest() *Request {
	return &Request{
		JSONRPC: jsonRPCVersion,
	}
}

// DecodeData decodes the given reader into the the request
// struct.
func (r *Request) DecodeData(data io.ReadCloser) error {
	defer data.Close()

	err := json.NewDecoder(data).Decode(r)
	if err != nil {
		return fmt.Errorf("Error parsing JSON payload: %s", err)
	}

	if r.JSONRPC != jsonRPCVersion {
		return fmt.Errorf("Invalid version, expected 2.0 got: '%s'", r.JSONRPC)
	}

	return nil
}

// Params takes a slice of any type and attempts to bind
// the params to it.
func (r *Request) Params() (*Params, error) {
	params := Params{}

	err := json.Unmarshal(r.RawParams, &params)
	if err != nil {
		return nil, fmt.Errorf("Error parsing params field in payload: %s", err)
	}

	return &params, nil
}

// WriteErrorResponse writes an error response to the ResponseWriter.
func (r Request) WriteErrorResponse(w http.ResponseWriter, err error) {
	jsonErr, ok := err.(*Error)
	if !ok {
		jsonErr = NewInternalServerError("Internal server error", err)
	}

	response := Response{
		JSONRPC: r.JSONRPC,
		Error:   jsonErr,
		ID:      r.RawID,
	}

	logFields := log.Fields{
		"err":    jsonErr.Cause,
		"method": r.Method,
	}
	params, err := r.Params()
	if err == nil {
		logFields["params"] = *params
	}

	log.WithFields(logFields).Error("Error encountered with rpc request")
	w.WriteHeader(jsonErr.HTTPCode)
	r.writeServerResponse(w, response)
}

// WriteResponse encodes the response and writes it to the ResponseWriter.
func (r Request) WriteResponse(w http.ResponseWriter, result interface{}) {
	response := Response{
		JSONRPC: r.JSONRPC,
		Result:  result,
		ID:      r.RawID,
	}

	r.writeServerResponse(w, response)
}

func (r Request) writeServerResponse(w http.ResponseWriter, response Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	err := encoder.Encode(response)

	if err != nil {
		fmt.Println(err)
	}
}
