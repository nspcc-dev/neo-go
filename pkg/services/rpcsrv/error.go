package rpcsrv

import (
	"net/http"

	"github.com/nspcc-dev/neo-go/pkg/neorpc"
)

// abstractResult is an interface which represents either single JSON-RPC 2.0 response
// or batch JSON-RPC 2.0 response.
type abstractResult interface {
	RunForErrors(f func(jsonErr *neorpc.Error))
}

// abstract represents abstract JSON-RPC 2.0 response. It is used as a server-side response
// representation.
type abstract struct {
	neorpc.Header
	Error  *neorpc.Error `json:"error,omitempty"`
	Result any           `json:"result,omitempty"`
}

// RunForErrors implements abstractResult interface.
func (a abstract) RunForErrors(f func(jsonErr *neorpc.Error)) {
	if a.Error != nil {
		f(a.Error)
	}
}

// abstractBatch represents abstract JSON-RPC 2.0 batch-response.
type abstractBatch []abstract

// RunForErrors implements abstractResult interface.
func (ab abstractBatch) RunForErrors(f func(jsonErr *neorpc.Error)) {
	for _, a := range ab {
		if a.Error != nil {
			f(a.Error)
		}
	}
}

func getHTTPCodeForError(respErr *neorpc.Error) int {
	var httpCode int
	switch respErr.Code {
	case neorpc.BadRequestCode:
		httpCode = http.StatusBadRequest
	case neorpc.MethodNotFoundCode:
		httpCode = http.StatusMethodNotAllowed
	case neorpc.InternalServerErrorCode:
		httpCode = http.StatusInternalServerError
	default:
		httpCode = http.StatusUnprocessableEntity
	}
	return httpCode
}
