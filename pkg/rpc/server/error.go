package server

import (
	"net/http"

	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
)

// abstractResult is an interface which represents either single JSON-RPC 2.0 response
// or batch JSON-RPC 2.0 response.
type abstractResult interface {
	RunForErrors(f func(jsonErr *response.Error))
}

// abstract represents abstract JSON-RPC 2.0 response. It is used as a server-side response
// representation.
type abstract struct {
	response.Header
	Error  *response.Error `json:"error,omitempty"`
	Result interface{}     `json:"result,omitempty"`
}

// RunForErrors implements abstractResult interface.
func (a abstract) RunForErrors(f func(jsonErr *response.Error)) {
	if a.Error != nil {
		f(a.Error)
	}
}

// abstractBatch represents abstract JSON-RPC 2.0 batch-response.
type abstractBatch []abstract

// RunForErrors implements abstractResult interface.
func (ab abstractBatch) RunForErrors(f func(jsonErr *response.Error)) {
	for _, a := range ab {
		if a.Error != nil {
			f(a.Error)
		}
	}
}

func getHTTPCodeForError(respErr *response.Error) int {
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
	return httpCode
}
