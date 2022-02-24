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

// Raw represents JSON-RPC request on the Client side.
type Raw struct {
	JSONRPC   string        `json:"jsonrpc"`
	Method    string        `json:"method"`
	RawParams []interface{} `json:"params"`
	ID        uint64        `json:"id"`
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
	RawParams []Param         `json:"params,omitempty"`
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
	var (
		in    *In
		batch Batch
	)
	in = &In{}
	err := json.Unmarshal(data, in)
	if err == nil {
		r.In = in
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	t, err := decoder.Token() // read `[`
	if err != nil {
		return err
	}
	if t != json.Delim('[') {
		return fmt.Errorf("`[` expected, got %s", t)
	}
	count := 0
	for decoder.More() {
		if count > maxBatchSize {
			return fmt.Errorf("the number of requests in batch shouldn't exceed %d", maxBatchSize)
		}
		in = &In{}
		decodeErr := decoder.Decode(in)
		if decodeErr != nil {
			return decodeErr
		}
		batch = append(batch, *in)
		count++
	}
	if len(batch) == 0 {
		return errors.New("empty request")
	}
	r.Batch = batch
	return nil
}

// DecodeData decodes the given reader into the the request
// struct.
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
		JSONRPC: JSONRPCVersion,
	}
}
