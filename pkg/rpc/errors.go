package rpc

import (
	"fmt"
)

type (
	// Error object for outputting JSON-RPC 2.0
	// errors.
	Error struct {
		Code    int64  `json:"code"`
		Message string `json:"message"`
		Data    string `json:"data,omitempty"`
	}
)

// NewParseError creates a new error with code
// -32700.
func NewParseError() *Error {
	return &Error{
		Code:    -32700,
		Message: "Parse Error",
	}
}

// NewInvalidRequestError creates a new error with
// code -32600.
func NewInvalidRequestError(data string) *Error {
	return &Error{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    data,
	}
}

// NewMethodNotFoundError creates a new error with
// code -32601.
func NewMethodNotFoundError(data string) *Error {
	return &Error{
		Code:    -32601,
		Message: "Method not found",
		Data:    data,
	}
}

// NewInvalidParmsError creates a new error with
// code -32602.
func NewInvalidParmsError(data string) *Error {
	return &Error{
		Code:    -32602,
		Message: "Invalid Params",
		Data:    data,
	}
}

// NewInternalErrorError creates a new error with
// code -32603.
func NewInternalErrorError(data string) *Error {
	return &Error{
		Code:    -32603,
		Message: "Internal error",
		Data:    data,
	}
}

// Error implements the error interface.
func (e Error) Error() string {
	return fmt.Sprintf("%s (%d) - %s", e.Message, e.Code, e.Data)
}
