package params

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/nspcc-dev/neo-go/pkg/neorpc"
)

const (
	// maxBatchSize is the maximum number of requests per batch.
	maxBatchSize = 100

	errNotJSONObject = "request must be a JSON object"
)

// Request contains standard JSON-RPC 2.0 request and batch of
// requests: http://www.jsonrpc.org/specification.
// It's used in server to represent incoming queries.
type Request struct {
	In    *In
	Batch Batch
}

// In represents a standard JSON-RPC 2.0
// request: http://www.jsonrpc.org/specification#request_object.
type In struct {
	JSONRPC   string          `json:"jsonrpc"`
	Method    string          `json:"method"`
	RawParams []Param         `json:"params,omitzero"`
	RawID     json.RawMessage `json:"id,omitzero"`
	// Err holds a JSON-RPC error produced while decoding a batch element.
	Err *neorpc.Error `json:"-"`
}

// Batch represents a standard JSON-RPC 2.0
// batch: https://www.jsonrpc.org/specification#batch.
type Batch []In

// MarshalJSON implements the json.Marshaler interface.
func (r Request) MarshalJSON() ([]byte, error) {
	if r.In != nil {
		return json.Marshal(r.In)
	}
	return json.Marshal(r.Batch)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (r *Request) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return io.ErrUnexpectedEOF
	}
	switch trimmed[0] {
	case '{':
		in := new(In)
		if err := json.Unmarshal(trimmed, in); err != nil {
			if _, ok := errors.AsType[*json.SyntaxError](err); ok || errors.Is(err, io.ErrUnexpectedEOF) {
				return err
			}
			return neorpc.NewInvalidRequestError(err.Error())
		}
		r.In = in
		r.Batch = nil
		return nil
	case '[':
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		if _, err := decoder.Token(); err != nil {
			return err
		}
		var batch Batch
		for decoder.More() {
			if len(batch) >= maxBatchSize {
				return neorpc.NewInvalidRequestError(fmt.Sprintf("the number of requests in batch shouldn't exceed %d", maxBatchSize))
			}
			var raw json.RawMessage
			if err := decoder.Decode(&raw); err != nil {
				return err
			}
			elem := bytes.TrimSpace(raw)
			var in *In
			if elem[0] != '{' {
				in = NewIn()
				in.Err = neorpc.NewInvalidRequestError(errNotJSONObject)
			} else {
				in = new(In)
				if err := json.Unmarshal(elem, in); err != nil {
					in = NewIn()
					in.Err = neorpc.NewInvalidRequestError(err.Error())
				}
			}
			batch = append(batch, *in)
		}
		t, err := decoder.Token()
		if err != nil {
			return err
		}
		if t != json.Delim(']') {
			return fmt.Errorf("`]` expected, got %v", t)
		}
		if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
			if err != nil {
				return err
			}
			return errors.New("unexpected trailing data after JSON array")
		}
		if len(batch) == 0 {
			return neorpc.NewInvalidRequestError("empty request")
		}
		r.In = nil
		r.Batch = batch
		return nil
	default:
		var v any
		if err := json.Unmarshal(trimmed, &v); err != nil {
			return err
		}
		return neorpc.NewInvalidRequestError(errNotJSONObject)
	}
}

// DecodeData decodes the given reader into the request struct.
func (r *Request) DecodeData(data io.ReadCloser) error {
	defer data.Close()

	rawData := json.RawMessage{}
	err := json.NewDecoder(data).Decode(&rawData)
	if err != nil {
		return fmt.Errorf("error parsing JSON payload: %w", err)
	}

	return r.UnmarshalJSON(rawData)
}

// NewRequest creates a new Request struct.
func NewRequest() *Request {
	return &Request{}
}

// NewIn creates a new In struct.
func NewIn() *In {
	return &In{
		JSONRPC: neorpc.JSONRPCVersion,
	}
}
