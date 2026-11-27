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
)

// ErrInvalidRequest is returned by (*Request).UnmarshalJSON when a syntactically
// valid JSON payload is not a valid JSON-RPC 2.0 request or batch.
var ErrInvalidRequest = errors.New("invalid request")

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
	var (
		in    *In
		batch Batch
	)
	if bytes.Equal(data, []byte("null")) {
		return fmt.Errorf("%w: null request", ErrInvalidRequest)
	}
	in = &In{}
	inErr := json.Unmarshal(data, in)
	if inErr == nil {
		r.In = in
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	t, err := decoder.Token() // read `[`
	if err != nil {
		return err
	}
	if t == json.Delim('{') {
		return fmt.Errorf("%w: %w", ErrInvalidRequest, inErr)
	}
	if t != json.Delim('[') {
		return fmt.Errorf("%w: `[` expected, got %v", ErrInvalidRequest, t)
	}
	for decoder.More() {
		if len(batch) >= maxBatchSize {
			return fmt.Errorf("%w: the number of requests in batch shouldn't exceed %d", ErrInvalidRequest, maxBatchSize)
		}
		var raw json.RawMessage
		decodeErr := decoder.Decode(&raw)
		if decodeErr != nil {
			return decodeErr
		}
		in = &In{}
		if bytes.Equal(raw, []byte("null")) {
			decodeErr = errors.New("null request")
		} else {
			decodeErr = json.Unmarshal(raw, in)
		}
		if decodeErr != nil {
			in = NewIn()
			in.Err = neorpc.NewInvalidRequestError(decodeErr.Error())
		}
		batch = append(batch, *in)
	}
	if len(batch) == 0 {
		return fmt.Errorf("%w: empty request", ErrInvalidRequest)
	}
	r.Batch = batch
	return nil
}

// DecodeData decodes the given reader into the request struct.
func (r *Request) DecodeData(data io.ReadCloser) *neorpc.Error {
	defer data.Close()

	rawData := json.RawMessage{}
	err := json.NewDecoder(data).Decode(&rawData)
	if err != nil {
		return neorpc.NewParseError(fmt.Sprintf("error parsing JSON payload: %s", err))
	}

	if err = r.UnmarshalJSON(rawData); err != nil {
		return neorpc.NewInvalidRequestError(err.Error())
	}
	return nil
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
