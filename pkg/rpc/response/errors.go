package response

import (
	"fmt"
)

// Error represents JSON-RPC 2.0 error type.
type Error struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// Standard RPC error codes defined by the JSON-RPC 2.0 specification.
const (
	// InternalServerErrorCode is returned for internal RPC server error.
	InternalServerErrorCode = -32603
	// BadRequestCode is returned on parse error.
	BadRequestCode = -32700
	// InvalidRequestCode is returned on invalid request.
	InvalidRequestCode = -32600
	// MethodNotFoundCode is returned on unknown method calling.
	MethodNotFoundCode = -32601
	// InvalidParamsCode is returned on request with invalid params.
	InvalidParamsCode = -32602
)

// RPC error codes defined by the Neo JSON-RPC specification extension.
const (
	// RPCErrorCode is returned on RPC request processing error.
	RPCErrorCode = -100
)

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

// NewError is an Error constructor that takes Error contents from its parameters.
func NewError(code int64, message string, data string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// NewParseError creates a new error with code
// -32700.
func NewParseError(data string) *Error {
	return NewError(BadRequestCode, "Parse Error", data)
}

// NewInvalidRequestError creates a new error with
// code -32600.
func NewInvalidRequestError(data string) *Error {
	return NewError(InvalidRequestCode, "Invalid Request", data)
}

// NewMethodNotFoundError creates a new error with
// code -32601.
func NewMethodNotFoundError(data string) *Error {
	return NewError(MethodNotFoundCode, "Method not found", data)
}

// NewInvalidParamsError creates a new error with
// code -32602.
func NewInvalidParamsError(data string) *Error {
	return NewError(InvalidParamsCode, "Invalid Params", data)
}

// NewInternalServerError creates a new error with
// code -32603.
func NewInternalServerError(data string) *Error {
	return NewError(InternalServerErrorCode, "Internal error", data)
}

// NewRPCError creates a new error with
// code -100.
func NewRPCError(message string, data string) *Error {
	return NewError(RPCErrorCode, message, data)
}

// NewSubmitError creates a new error with
// specified error code and error message.
func NewSubmitError(code int64, message string) *Error {
	return NewError(code, message, "")
}

// WrapErrorWithData returns copy of the given error with the specified data and cause.
// It does not modify the source error.
func WrapErrorWithData(e *Error, data string) *Error {
	return NewError(e.Code, e.Message, data)
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
