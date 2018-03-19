package rpc

import (
	"fmt"
)

type (
	// Error object for outputting JSON-RPC 2.0
	// errors.
	Error struct {
		Code    int64  `json:"code"`
		Cause   error  `json:"-"`
		Message string `json:"message"`
		Data    string `json:"data,omitempty"`
	}
)

func newError(code int64, message string, data string, cause error) *Error {
	return &Error{
		Code:    code,
		Cause:   cause,
		Message: message,
		Data:    data,
	}

}

// NewParseError creates a new error with code
// -32700.:%s
func NewParseError(data string, cause error) *Error {
	return newError(-32700, "Parse Error", data, cause)
}

// NewInvalidRequestError creates a new error with
// code -32600.
func NewInvalidRequestError(data string, cause error) *Error {
	return newError(-32600, "Invalid Request", data, cause)
}

// NewMethodNotFoundError creates a new error with
// code -32601.
func NewMethodNotFoundError(data string, cause error) *Error {
	return newError(-32601, "Method not found", data, cause)
}

// NewInvalidParmsError creates a new error with
// code -32602.
func NewInvalidParmsError(data string, cause error) *Error {
	return newError(-32602, "Invalid Params", data, cause)
}

// NewInternalErrorError creates a new error with
// code -32603.
func NewInternalErrorError(data string, cause error) *Error {
	return newError(-32603, "Internal error", data, cause)
}

// Error implements the error interface.
func (e Error) Error() string {
	return fmt.Sprintf("%s (%d) - %s - %s", e.Message, e.Code, e.Data, e.Cause)
}
