package response

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
)

// Header is a generic JSON-RPC 2.0 response header (ID and JSON-RPC version).
type Header struct {
	ID      json.RawMessage `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
}

// HeaderAndError adds an Error (that can be empty) to the Header, it's used
// to construct type-specific responses.
type HeaderAndError struct {
	Header
	Error *Error `json:"error,omitempty"`
}

// Raw represents a standard raw JSON-RPC 2.0
// response: http://www.jsonrpc.org/specification#response_object.
type Raw struct {
	HeaderAndError
	Result json.RawMessage `json:"result,omitempty"`
}

// GetTxOut represents result of `gettxout` RPC call.
type GetTxOut struct {
	HeaderAndError
	Result *result.TransactionOutput
}

// GetRawTx represents verbose output of `getrawtransaction` RPC call.
type GetRawTx struct {
	HeaderAndError
	Result *result.TransactionOutputRaw `json:"result"`
}

// AbstractResult is an interface which represents either single JSON-RPC 2.0 response
// or batch JSON-RPC 2.0 response.
type AbstractResult interface {
	RunForErrors(f func(jsonErr *Error))
}

// RunForErrors implements AbstractResult interface.
func (r Raw) RunForErrors(f func(jsonErr *Error)) {
	if r.Error != nil {
		f(r.Error)
	}
}

// RawBatch represents abstract JSON-RPC 2.0 batch-response.
type RawBatch []Raw

// RunForErrors implements AbstractResult interface.
func (rb RawBatch) RunForErrors(f func(jsonErr *Error)) {
	for _, r := range rb {
		if r.Error != nil {
			f(r.Error)
		}
	}
}

// Notification is a type used to represent wire format of events, they're
// special in that they look like requests but they don't have IDs and their
// "method" is actually an event name.
type Notification struct {
	JSONRPC string        `json:"jsonrpc"`
	Event   EventID       `json:"method"`
	Payload []interface{} `json:"params"`
}
