package response

import (
	"fmt"
	"net/http"
)

type (
	// ServerError object for outputting JSON-RPC 2.0 errors on the server side.
	ServerError struct {
		*Error
		HTTPCode int // HTTPCode won't be marshalled because Error's marshaller is used.
	}

	// Error represents JSON-RPC 2.0 error type.
	Error struct {
		Code    int64  `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data,omitempty"`
	}
)

// InternalServerErrorCode is returned for internal RPC server error.
const InternalServerErrorCode = -32603

var (
	// ErrInvalidParams represents a generic 'invalid parameters' error.
	ErrInvalidParams = NewInvalidParamsError("invalid params")
	// ErrAlreadyExists represents SubmitError with code -501.
	ErrAlreadyExists = NewSubmitError(-501, "Block or transaction already exists and cannot be sent repeatedly.")
	// ErrOutOfMemory represents SubmitError with code -502.
	ErrOutOfMemory = NewSubmitError(-502, "The memory pool is full and no more transactions can be sent.")
	// ErrUnableToVerify represents SubmitError with code -503.
	ErrUnableToVerify = NewSubmitError(-503, "The block cannot be validated.")
	// ErrValidationFailed represents SubmitError with code -504.
	ErrValidationFailed = NewSubmitError(-504, "Block or transaction validation failed.")
	// ErrPolicyFail represents SubmitError with code -505.
	ErrPolicyFail = NewSubmitError(-505, "One of the Policy filters failed.")
	// ErrUnknown represents SubmitError with code -500.
	ErrUnknown = NewSubmitError(-500, "Unknown error.")
)

// NewServerError is an ServerError constructor that takes ServerError contents from its
// parameters.
func NewServerError(code int64, httpCode int, message string, data string) *ServerError {
	return &ServerError{
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
		HTTPCode: httpCode,
	}
}

// NewParseError creates a new error with code
// -32700.
func NewParseError(data string) *ServerError {
	return NewServerError(-32700, http.StatusBadRequest, "Parse Error", data)
}

// NewInvalidRequestError creates a new error with
// code -32600.
func NewInvalidRequestError(data string) *ServerError {
	return NewServerError(-32600, http.StatusUnprocessableEntity, "Invalid Request", data)
}

// NewMethodNotFoundError creates a new error with
// code -32601.
func NewMethodNotFoundError(data string) *ServerError {
	return NewServerError(-32601, http.StatusMethodNotAllowed, "Method not found", data)
}

// NewInvalidParamsError creates a new error with
// code -32602.
func NewInvalidParamsError(data string) *ServerError {
	return NewServerError(-32602, http.StatusUnprocessableEntity, "Invalid Params", data)
}

// NewInternalServerError creates a new error with
// code -32603.
func NewInternalServerError(data string) *ServerError {
	return NewServerError(InternalServerErrorCode, http.StatusInternalServerError, "Internal error", data)
}

// NewRPCError creates a new error with
// code -100.
func NewRPCError(message string, data string) *ServerError {
	return NewServerError(-100, http.StatusUnprocessableEntity, message, data)
}

// NewSubmitError creates a new error with
// specified error code and error message.
func NewSubmitError(code int64, message string) *ServerError {
	return NewServerError(code, http.StatusUnprocessableEntity, message, "")
}

// WrapErrorWithData returns copy of the given error with the specified data and cause.
// It does not modify the source error.
func WrapErrorWithData(e *ServerError, data string) *ServerError {
	return NewServerError(e.Code, e.HTTPCode, e.Message, data)
}

// Error implements the error interface.
func (e *Error) Error() string {
	if len(e.Data) == 0 {
		return fmt.Sprintf("%s (%d)", e.Message, e.Code)
	}
	return fmt.Sprintf("%s (%d) - %s", e.Message, e.Code, e.Data)
}

// Is denotes whether the error matches the target one.
func (e *Error) Is(target error) bool {
	clTarget, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == clTarget.Code
}
