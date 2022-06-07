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
		Message  string `json:"message"`
		Data     string `json:"data,omitempty"`
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

// NewError is an Error constructor that takes Error contents from its
// parameters.
func NewError(code int64, httpCode int, message string, data string) *Error {
	return &Error{
		Code:     code,
		HTTPCode: httpCode,
		Message:  message,
		Data:     data,
	}
}

// NewParseError creates a new error with code
// -32700.
func NewParseError(data string) *Error {
	return NewError(-32700, http.StatusBadRequest, "Parse Error", data)
}

// NewInvalidRequestError creates a new error with
// code -32600.
func NewInvalidRequestError(data string) *Error {
	return NewError(-32600, http.StatusUnprocessableEntity, "Invalid Request", data)
}

// NewMethodNotFoundError creates a new error with
// code -32601.
func NewMethodNotFoundError(data string) *Error {
	return NewError(-32601, http.StatusMethodNotAllowed, "Method not found", data)
}

// NewInvalidParamsError creates a new error with
// code -32602.
func NewInvalidParamsError(data string) *Error {
	return NewError(-32602, http.StatusUnprocessableEntity, "Invalid Params", data)
}

// NewInternalServerError creates a new error with
// code -32603.
func NewInternalServerError(data string) *Error {
	return NewError(InternalServerErrorCode, http.StatusInternalServerError, "Internal error", data)
}

// NewRPCError creates a new error with
// code -100.
func NewRPCError(message string, data string) *Error {
	return NewError(-100, http.StatusUnprocessableEntity, message, data)
}

// NewSubmitError creates a new error with
// specified error code and error message.
func NewSubmitError(code int64, message string) *Error {
	return NewError(code, http.StatusUnprocessableEntity, message, "")
}

// Error implements the error interface.
func (e *Error) Error() string {
	if len(e.Data) == 0 {
		return fmt.Sprintf("%s (%d)", e.Message, e.Code)
	}
	return fmt.Sprintf("%s (%d) - %s", e.Message, e.Code, e.Data)
}

// WrapErrorWithData returns copy of the given error with the specified data and cause.
// It does not modify the source error.
func WrapErrorWithData(e *Error, data string) *Error {
	return NewError(e.Code, e.HTTPCode, e.Message, data)
}
