package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type (
	// Request represents a standard JSON-RPC 2.0
	// request: http://www.jsonrpc.org/specification#request_object.
	Request struct {
		JSONRPC   string          `json:"jsonrpc"`
		Method    string          `json:"method"`
		RawParams json.RawMessage `json:"params,omitempty"`
		RawID     json.RawMessage `json:"id,omitempty"`
		err       *Error
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

// NewRequest creates a new request from the given io.ReadCloser.
func NewRequest(data io.ReadCloser) *Request {
	defer data.Close()

	request := &Request{}

	err := json.NewDecoder(data).Decode(request)
	if err != nil {
		request.err = NewInvalidRequestError(
			fmt.Sprintf("Error parsing JSON payload: %s", err),
		)
	}

	if request.JSONRPC != "2.0" {
		request.err = NewInvalidParmsError(
			fmt.Sprintf("Invalid version, expected 2.0 got: '%s'", request.JSONRPC),
		)
	}

	return request
}

// HasError indicates if the request has an erorr or not.
func (r Request) HasError() bool {
	return r.err != nil
}

// ID returns the parsed ID if we have one.
func (r *Request) ID() (int, error) {
	var id *int
	err := json.Unmarshal(r.RawID, &id)
	if err != nil {
		r.err = NewInvalidRequestError(
			fmt.Sprintf("Error parsing JSON payload: %s", err),
		)
		return 0, err
	}

	return *id, nil
}

// Params takes a slice of any type and attempts to bind
// the params to it.
func (r *Request) Params() Params {
	params := Params{}
	err := json.Unmarshal(r.RawParams, &params)
	if err != nil {
		r.err = NewInvalidRequestError(
			fmt.Sprintf("Error parsing params field in payload: %s", err),
		)
	}

	return params
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

// WriteError writes an error to the given response writer.
func (r Request) WriteError(w http.ResponseWriter, status int, err error) {
	jsonErr, ok := err.(*Error)
	if !ok {
		jsonErr = NewInternalErrorError(err.Error())
	}

	res := Response{
		JSONRPC: r.JSONRPC,
		Error:   jsonErr,
		ID:      r.RawID,
	}

	r.writeServerResponse(w, res)
}

func (r Request) writeServerResponse(w http.ResponseWriter, res Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	err := encoder.Encode(res)

	if err != nil {
		fmt.Println(err)
	}
}
