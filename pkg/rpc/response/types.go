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

// Notification is a type used to represent wire format of events, they're
// special in that they look like requests but they don't have IDs and their
// "method" is actually an event name.
type Notification struct {
	JSONRPC string        `json:"jsonrpc"`
	Event   EventID       `json:"method"`
	Payload []interface{} `json:"params"`
}
