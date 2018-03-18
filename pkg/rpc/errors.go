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
func NewInvalidRequestError(err error) *Error {
	return &Error{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    err.Error(),
	}
}

// NewMethodNotFoundError creates a new error with
// code -32601.
func NewMethodNotFoundError(err error) *Error {
	return &Error{
		Code:    -32601,
		Message: "Method not found",
		Data:    err.Error(),
	}
}

// NewInvalidParmsError creates a new error with
// code -32602.
func NewInvalidParmsError(err error) *Error {
	return &Error{
		Code:    -32602,
		Message: "Invalid Params",
		Data:    err.Error(),
	}
}

// NewInternalErrorError creates a new error with
// code -32603.
func NewInternalErrorError(err error) *Error {
	return &Error{
		Code:    -32603,
		Message: "Internal error",
		Data:    err.Error(),
	}
}

// Error implements the error interface.
func (e Error) Error() string {
	return fmt.Sprintf("%s (%d) - %s", e.Message, e.Code, e.Data)
}
