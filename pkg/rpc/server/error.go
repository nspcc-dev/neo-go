package server

import (
	"net/http"

	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
)

// serverError represents RPC error response on the server side.
type serverError struct {
	*response.Error
	HTTPCode int // HTTPCode won't be marshalled because Error's marshaller is used.
}

// abstractResult is an interface which represents either single JSON-RPC 2.0 response
// or batch JSON-RPC 2.0 response.
type abstractResult interface {
	RunForErrors(f func(jsonErr *response.Error))
}

// abstract represents abstract JSON-RPC 2.0 response. It is used as a server-side response
// representation.
type abstract struct {
	response.Header
	Error  *serverError `json:"error,omitempty"`
	Result interface{}  `json:"result,omitempty"`
}

// RunForErrors implements abstractResult interface.
func (a abstract) RunForErrors(f func(jsonErr *response.Error)) {
	if a.Error != nil {
		f(a.Error.Error)
	}
}

// abstractBatch represents abstract JSON-RPC 2.0 batch-response.
type abstractBatch []abstract

// RunForErrors implements abstractResult interface.
func (ab abstractBatch) RunForErrors(f func(jsonErr *response.Error)) {
	for _, a := range ab {
		if a.Error != nil {
			f(a.Error.Error)
		}
	}
}

func packClientError(respErr *response.Error) *serverError {
	var httpCode int
	switch respErr.Code {
	case response.BadRequestCode:
		httpCode = http.StatusBadRequest
	case response.InvalidRequestCode, response.RPCErrorCode, response.InvalidParamsCode:
		httpCode = http.StatusUnprocessableEntity
	case response.MethodNotFoundCode:
		httpCode = http.StatusMethodNotAllowed
	case response.InternalServerErrorCode:
		httpCode = http.StatusInternalServerError
	default:
		httpCode = http.StatusUnprocessableEntity
	}
	return &serverError{
		Error:    respErr,
		HTTPCode: httpCode,
	}
}
