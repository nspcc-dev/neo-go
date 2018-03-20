package rpc

import (
	"fmt"
	"net/http"
)

type (
	// Error object for outputting JSON-RPC 2.0
	// errors.
	Error struct {
		Code     int64  `json:"code"`
		HTTPCode int    `json:"-"`
		Cause    error  `json:"-"`
		Message  string `json:"message"`
		Data     string `json:"data,omitempty"`
	}
)

func newError(code int64, httpCode int, message string, data string, cause error) *Error {
	return &Error{
		Code:     code,
		HTTPCode: httpCode,
		Cause:    cause,
		Message:  message,
		Data:     data,
	}

}

// NewParseError creates a new error with code
// -32700.:%s
func NewParseError(data string, cause error) *Error {
	return newError(-32700, http.StatusBadRequest, "Parse Error", data, cause)
}

// NewInvalidRequestError creates a new error with
// code -32600.
func NewInvalidRequestError(data string, cause error) *Error {
	return newError(-32600, http.StatusUnprocessableEntity, "Invalid Request", data, cause)
}

// NewMethodNotFoundError creates a new error with
// code -32601.
func NewMethodNotFoundError(data string, cause error) *Error {
	return newError(-32601, http.StatusMethodNotAllowed, "Method not found", data, cause)
}

// NewInvalidParamsError creates a new error with
// code -32602.
func NewInvalidParamsError(data string, cause error) *Error {
	return newError(-32602, http.StatusUnprocessableEntity, "Invalid Params", data, cause)
}

// NewInternalServerError creates a new error with
// code -32603.
func NewInternalServerError(data string, cause error) *Error {
	return newError(-32603, http.StatusInternalServerError, "Internal error", data, cause)
}

// Error implements the error interface.
func (e Error) Error() string {
	return fmt.Sprintf("%s (%d) - %s - %s", e.Message, e.Code, e.Data, e.Cause)
}
