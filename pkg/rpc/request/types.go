package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	// JSONRPCVersion is the only JSON-RPC protocol version supported.
	JSONRPCVersion = "2.0"

	// maxBatchSize is the maximum number of request per batch.
	maxBatchSize = 100
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
	RawParams json.RawMessage `json:"params,omitempty"`
	RawID     json.RawMessage `json:"id,omitempty"`
}

// Batch represents a standard JSON-RPC 2.0
// batch: https://www.jsonrpc.org/specification#batch.
type Batch []In

// MarshalJSON implements json.Marshaler interface.
func (r Request) MarshalJSON() ([]byte, error) {
	if r.In != nil {
		return json.Marshal(r.In)
	}
	return json.Marshal(r.Batch)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (r *Request) UnmarshalJSON(data []byte) error {
	return r.decodeData(bytes.NewReader(data))
}

// DecodeData decodes the given reader into the the request
// struct.
func (r *Request) DecodeData(data io.ReadCloser) error {
	defer data.Close()
	return r.decodeData(data)
}

func (r *Request) decodeData(data io.Reader) error {

	var (
		in    *In
		batch Batch
	)

	isBatch := false
	count := 0
	decoder := json.NewDecoder(data)
loop:
	for {
		t, err := decoder.Token() // read `[` or `{`
		if err != nil {
			return err
		}

		switch t {
		case json.Delim('['):
			if isBatch {
				return fmt.Errorf("unexpected token: %s", t)
			}
			isBatch = true
		case json.Delim('{'):
			in = r.In
			for decoder.More() {
				t, err := decoder.Token()
				if err != nil {
					return fmt.Errorf("map key is expected: %w", err)
				}

				// Having t other than string is JSON spec violation and
				// Go catches this for us the `Token` call above.
				key := t.(string)
				switch key {
				case "jsonrpc":
					err = decoder.Decode(&in.JSONRPC)
				case "method":
					err = decoder.Decode(&in.Method)
				case "params":
					err = decoder.Decode(&in.RawParams)
				case "id":
					err = decoder.Decode(&in.RawID)
				default: // skip extra fields for compatibility
					var v interface{}
					err = decoder.Decode(&v)
					if err != nil {
						return err
					}
					continue
				}
				if err != nil {
					return fmt.Errorf("invalid value for '%s': %w", key, err)
				}
			}
			_, err := decoder.Token()
			if err != nil {
				return err
			}

			if !isBatch {
				r.In = in
				return nil
			}
			batch = append(batch, *in)
			count++
			if count > maxBatchSize {
				return fmt.Errorf("the number of requests in batch shouldn't exceed %d", maxBatchSize)
			}
		case json.Delim(']'):
			break loop
		default:
			return fmt.Errorf("`[` or `{` expected, got %s", t)
		}
	}
	if len(batch) == 0 {
		return errors.New("empty request")
	}
	r.Batch = batch
	return nil
}

// NewRequest creates a new Request struct.
func NewRequest() *Request {
	return &Request{}
}

// NewIn creates a new In struct.
func NewIn() *In {
	return &In{
		JSONRPC: JSONRPCVersion,
	}
}

// Params takes a slice of any type and attempts to bind
// the params to it.
func (r *In) Params() (*Params, error) {
	params := Params{}

	err := json.Unmarshal(r.RawParams, &params)
	if err != nil {
		return nil, fmt.Errorf("error parsing params: %w", err)
	}

	return &params, nil
}
