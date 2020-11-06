package response

import (
	"encoding/json"
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

// AbstractResult is an interface which represents either single JSON-RPC 2.0 response
// or batch JSON-RPC 2.0 response.
type AbstractResult interface {
	RunForErrors(f func(jsonErr *Error))
}

// Abstract represents abstract JSON-RPC 2.0 response, it differs from Raw in
// that Result field is an interface here.
type Abstract struct {
	HeaderAndError
	Result interface{} `json:"result,omitempty"`
}

// RunForErrors implements AbstractResult interface.
func (a Abstract) RunForErrors(f func(jsonErr *Error)) {
	if a.Error != nil {
		f(a.Error)
	}
}

// AbstractBatch represents abstract JSON-RPC 2.0 batch-response.
type AbstractBatch []Abstract

// RunForErrors implements AbstractResult interface.
func (ab AbstractBatch) RunForErrors(f func(jsonErr *Error)) {
	for _, a := range ab {
		if a.Error != nil {
			f(a.Error)
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
