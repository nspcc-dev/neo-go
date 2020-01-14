package request

import (
	"encoding/json"
	"io"

	"github.com/pkg/errors"
)

const (
	// JSONRPCVersion is the only JSON-RPC protocol version supported.
	JSONRPCVersion = "2.0"
)

// RawParams is just a slice of abstract values, used to represent parameters
// passed from client to server.
type RawParams struct {
	Values []interface{}
}

// NewRawParams creates RawParams from its parameters.
func NewRawParams(vals ...interface{}) RawParams {
	p := RawParams{}
	p.Values = make([]interface{}, len(vals))
	for i := 0; i < len(p.Values); i++ {
		p.Values[i] = vals[i]
	}
	return p
}

// Raw represents JSON-RPC request.
type Raw struct {
	JSONRPC   string        `json:"jsonrpc"`
	Method    string        `json:"method"`
	RawParams []interface{} `json:"params"`
	ID        int           `json:"id"`
}

// In represents a standard JSON-RPC 2.0
// request: http://www.jsonrpc.org/specification#request_object. It's used in
// server to represent incoming queries.
type In struct {
	JSONRPC   string          `json:"jsonrpc"`
	Method    string          `json:"method"`
	RawParams json.RawMessage `json:"params,omitempty"`
	RawID     json.RawMessage `json:"id,omitempty"`
}

// NewIn creates a new Request struct.
func NewIn() *In {
	return &In{
		JSONRPC: JSONRPCVersion,
	}
}

// DecodeData decodes the given reader into the the request
// struct.
func (r *In) DecodeData(data io.ReadCloser) error {
	defer data.Close()

	err := json.NewDecoder(data).Decode(r)
	if err != nil {
		return errors.Errorf("error parsing JSON payload: %s", err)
	}

	if r.JSONRPC != JSONRPCVersion {
		return errors.Errorf("invalid version, expected 2.0 got: '%s'", r.JSONRPC)
	}

	return nil
}

// Params takes a slice of any type and attempts to bind
// the params to it.
func (r *In) Params() (*Params, error) {
	params := Params{}

	err := json.Unmarshal(r.RawParams, &params)
	if err != nil {
		return nil, errors.Errorf("error parsing params field in payload: %s", err)
	}

	return &params, nil
}
