package response

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

var (
	// ErrInvalidParams represents a generic 'invalid parameters' error.
	ErrInvalidParams = NewInvalidParamsError("", nil)
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

// NewError is an Error constructor that takes Error contents from its
// parameters.
func NewError(code int64, httpCode int, message string, data string, cause error) *Error {
	return &Error{
		Code:     code,
		HTTPCode: httpCode,
		Cause:    cause,
		Message:  message,
		Data:     data,
	}
}

// NewParseError creates a new error with code
// -32700.
func NewParseError(data string, cause error) *Error {
	return NewError(-32700, http.StatusBadRequest, "Parse Error", data, cause)
}

// NewInvalidRequestError creates a new error with
// code -32600.
func NewInvalidRequestError(data string, cause error) *Error {
	return NewError(-32600, http.StatusUnprocessableEntity, "Invalid Request", data, cause)
}

// NewMethodNotFoundError creates a new error with
// code -32601.
func NewMethodNotFoundError(data string, cause error) *Error {
	return NewError(-32601, http.StatusMethodNotAllowed, "Method not found", data, cause)
}

// NewInvalidParamsError creates a new error with
// code -32602.
func NewInvalidParamsError(data string, cause error) *Error {
	return NewError(-32602, http.StatusUnprocessableEntity, "Invalid Params", data, cause)
}

// NewInternalServerError creates a new error with
// code -32603.
func NewInternalServerError(data string, cause error) *Error {
	return NewError(-32603, http.StatusInternalServerError, "Internal error", data, cause)
}

// NewRPCError creates a new error with
// code -100.
func NewRPCError(message string, data string, cause error) *Error {
	return NewError(-100, http.StatusUnprocessableEntity, message, data, cause)
}

// NewSubmitError creates a new error with
// specified error code and error message.
func NewSubmitError(code int64, message string) *Error {
	return NewError(code, http.StatusUnprocessableEntity, message, "", nil)
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s (%d) - %s", e.Message, e.Code, e.Data)
	}
	return fmt.Sprintf("%s (%d) - %s - %s", e.Message, e.Code, e.Data, e.Cause)
}

// WrapErrorWithData returns copy of the given error with specified data and cause.
// It does not modify the source error.
func WrapErrorWithData(e *Error, data error) *Error {
	return NewError(e.Code, e.HTTPCode, e.Message, data.Error(), data)
}
