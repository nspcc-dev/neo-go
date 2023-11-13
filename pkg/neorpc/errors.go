package neorpc

import (
	"errors"
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
// Codes for missing items.
const (
	// ErrUnknownBlockCode is returned from a call that accepts as a parameter or searches for a header or a block
	// as a part of its job can't find it.
	ErrUnknownBlockCode = -101
	// ErrUnknownContractCode is returned from a call that accepts as a parameter or searches for a contract
	// as a part of its job can't find it.
	ErrUnknownContractCode = -102
	// ErrUnknownTransactionCode is returned from a call that accepts as a parameter or searches for a transaction
	// as a part of its job can't find it.
	ErrUnknownTransactionCode = -103
	// ErrUnknownStorageItemCode is returned from a call that looks for an item in the contract storage
	// as part of its job can't find it.
	ErrUnknownStorageItemCode = -104
	// ErrUnknownScriptContainerCode is returned from a call that accepts as a parameter or searches for a script
	// container (a block or transaction) as a part of its job can't find it
	// (this error generalizes -101 and -103 in cases where it's needed).
	ErrUnknownScriptContainerCode = -105
	// ErrUnknownStateRootCode is returned from a call that accepts as a parameter or searches for a state root
	// as a part of its job can't find it.
	ErrUnknownStateRootCode = -106
	// ErrUnknownSessionCode is returned from a call that accepts as a parameter or searches for an iterator session
	// as a part of its job can't find it.
	ErrUnknownSessionCode = -107
	// ErrUnknownIteratorCode is returned from a call that accepts as a parameter or searches for a session iterator
	// as a part of its job can't find it.
	ErrUnknownIteratorCode = -108
	// ErrUnknownHeightCode is returned if block or header height passed as parameter or calculated during call
	// execution is not correct (out of the range known to the node).
	ErrUnknownHeightCode = -109
)

// Codes for calls that use a wallet (-300...-304) can be returned by the C# RPC server only,
// see the https://github.com/nspcc-dev/neo-go/blob/master/docs/rpc.md#unsupported-methods.
const (
	// ErrInsufficientFundsWalletCode is returned if transaction that sends some assets can't be created
	// because it fails. Can be returned only by the C# RPC server.
	ErrInsufficientFundsWalletCode = -300
	// ErrWalletFeeLimitCode is returned if transaction requires more network fee to be paid
	// than is allowed by settings. Can be returned only by the C# RPC server.
	ErrWalletFeeLimitCode = -301
	// ErrNoOpenedWalletCode is returned if server doesn't have any opened wallet to operate with.
	// Can be returned only by the C# RPC server.
	ErrNoOpenedWalletCode = -302
	// ErrWalletNotFoundCode is returned if specified (or configured) wallet file path is invalid.
	// Can be returned only by the C# RPC server.
	ErrWalletNotFoundCode = -303
	// ErrWalletNotSupportedCode is returned if specified (or configured) file can't be opened as a wallet.
	// Can be returned only by the C# RPC server.
	ErrWalletNotSupportedCode = -304
)

// Inventory verification or verification script errors.
const (
	// ErrVerificationFailedCode is returned on anything that can't be expressed by other codes.
	// It is an unclassified inventory verification error.
	ErrVerificationFailedCode = -500
	// ErrAlreadyExistsCode is returned if block or transaction is already accepted and processed on chain.
	ErrAlreadyExistsCode = -501
	// ErrMempoolCapReachedCode is returned if no more transactions can be accepted into the memory pool
	// (unless they have a priority) as its full capacity is reached.
	ErrMempoolCapReachedCode = -502
	// ErrAlreadyInPoolCode is returned if transaction is already pooled, but not yet accepted into a block.
	ErrAlreadyInPoolCode = -503
	// ErrInsufficientNetworkFeeCode is returned if transaction has incorrect (too small per Policy setting)
	// network fee value.
	ErrInsufficientNetworkFeeCode = -504
	// ErrPolicyFailedCode is returned from a call denied by the Policy contract (one of signers is blocked) or
	// if one of the Policy filters failed.
	ErrPolicyFailedCode = -505
	// ErrInvalidScriptCode is returned if transaction contains incorrect executable script.
	ErrInvalidScriptCode = -506
	// ErrInvalidAttributeCode is returned if transaction contains an invalid attribute.
	ErrInvalidAttributeCode = -507
	// ErrInvalidSignatureCode is returned if one of the verification scripts failed.
	ErrInvalidSignatureCode = -508
	// ErrInvalidSizeCode is returned if transaction or its script is too big.
	ErrInvalidSizeCode = -509
	// ErrExpiredTransactionCode is returned if transaction's ValidUntilBlock value is already in the past.
	ErrExpiredTransactionCode = -510
	// ErrInsufficientFundsCode is returned if sender doesn't have enough GAS to pay for all currently pooled transactions.
	ErrInsufficientFundsCode = -511
	// ErrInvalidVerificationFunctionCode is returned if contract doesn't have a verify method or
	// this method doesn't return proper value.
	ErrInvalidVerificationFunctionCode = -512
)

// Errors related to node configuration and various services.
const (
	// ErrSessionsDisabledCode is returned if iterator session support is not enabled on the server.
	ErrSessionsDisabledCode = -601
	// ErrOracleDisabledCode is returned if Oracle service is not enabled in the configuration (service is not running).
	ErrOracleDisabledCode = -602
	// ErrOracleRequestFinishedCode is returned if Oracle request submitted is already completely processed.
	// Can be returned only by the C# RPC server.
	ErrOracleRequestFinishedCode = -603
	// ErrOracleRequestNotFoundCode is returned if Oracle request submitted is not known to this node.
	// Can be returned only by the C# RPC server.
	ErrOracleRequestNotFoundCode = -604
	// ErrOracleNotDesignatedNodeCode is returned if Oracle service is enabled, but this node is not designated
	// to provide this functionality. Can be returned only by the C# RPC server.
	ErrOracleNotDesignatedNodeCode = -605
	// ErrUnsupportedStateCode is returned if this node can't answer requests for old state because it's configured
	// to keep only the latest one.
	ErrUnsupportedStateCode = -606
	// ErrInvalidProofCode is returned if state proof verification failed.
	ErrInvalidProofCode = -607
	// ErrExecutionFailedCode is returned from a call made a VM execution, but it has failed.
	ErrExecutionFailedCode = -608
)

var (
	// ErrCompatGeneric is an error returned by nodes not compliant with the neo-project/proposals#156
	// (NeoGo pre-0.102.0 and all known C# versions).
	// It can be returned for any call and doesn't have any specific meaning.
	//
	// Deprecated: to be removed after all nodes adopt new error standard.
	ErrCompatGeneric = NewErrorWithCode(-100, "RPC error")

	// ErrCompatNoOpenedWallet is an error code returned by nodes not compliant with the neo-project/proposals#156
	// (all known C# versions, NeoGo never used this code). It can be returned for wallet-related operations.
	//
	// Deprecated: to be removed after all nodes adopt new error standard.
	ErrCompatNoOpenedWallet = NewErrorWithCode(-400, "No opened wallet")
)

var (
	// ErrInvalidParams represents a generic "Invalid params" error.
	ErrInvalidParams = NewInvalidParamsError("Invalid params")

	// ErrUnknownBlock represents an error with code [ErrUnknownBlockCode].
	// Call that accepts as a parameter or searches for a header or a block as a part of its job can't find it.
	ErrUnknownBlock = NewErrorWithCode(ErrUnknownBlockCode, "Unknown block")
	// ErrUnknownContract represents an error with code [ErrUnknownContractCode].
	// Call that accepts as a parameter or searches for a contract as a part of its job can't find it.
	ErrUnknownContract = NewErrorWithCode(ErrUnknownContractCode, "Unknown contract")
	// ErrUnknownTransaction represents an error with code [ErrUnknownTransactionCode].
	// Call that accepts as a parameter or searches for a transaction as a part of its job can't find it.
	ErrUnknownTransaction = NewErrorWithCode(ErrUnknownTransactionCode, "Unknown transaction")
	// ErrUnknownStorageItem represents an error with code [ErrUnknownStorageItemCode].
	// Call that looks for an item in the contract storage as part of its job can't find it.
	ErrUnknownStorageItem = NewErrorWithCode(ErrUnknownStorageItemCode, "Unknown storage item")
	// ErrUnknownScriptContainer represents an error with code [ErrUnknownScriptContainerCode].
	// Call that accepts as a parameter or searches for a script container (a block or transaction)
	// as a part of its job can't find it (this error generalizes [ErrUnknownBlockCode] and [ErrUnknownTransactionCode]
	// in cases where it's needed).
	ErrUnknownScriptContainer = NewErrorWithCode(ErrUnknownScriptContainerCode, "Unknown script container")
	// ErrUnknownStateRoot represents an error with code [ErrUnknownStateRootCode].
	// Call that accepts as a parameter or searches for a state root as a part of its job can't find it.
	ErrUnknownStateRoot = NewErrorWithCode(ErrUnknownStateRootCode, "Unknown state root")
	// ErrUnknownSession represents an error with code [ErrUnknownSessionCode].
	// Call that accepts as a parameter or searches for an iterator session as a part of its job can't find it.
	ErrUnknownSession = NewErrorWithCode(ErrUnknownSessionCode, "Unknown session")
	// ErrUnknownIterator represents an error with code [ErrUnknownIteratorCode].
	// Call that accepts as a parameter or searches for a session iterator as a part of its job can't find it.
	ErrUnknownIterator = NewErrorWithCode(ErrUnknownIteratorCode, "Unknown iterator")
	// ErrUnknownHeight represents an error with code [ErrUnknownHeightCode].
	// Block or header height passed as parameter or calculated during call execution is not correct
	// (out of the range known to the node).
	ErrUnknownHeight = NewErrorWithCode(ErrUnknownHeightCode, "Unknown height")

	// ErrInsufficientFundsWallet represents an error with code [ErrInsufficientFundsWalletCode]. Can be returned only by the C# RPC server.
	// Transaction that sends some assets can't be created because it fails.
	ErrInsufficientFundsWallet = NewErrorWithCode(ErrInsufficientFundsWalletCode, "Insufficient funds")
	// ErrWalletFeeLimit represents an error with code [ErrWalletFeeLimitCode]. Can be returned only by the C# RPC server.
	// Transaction requires more network fee to be paid than is allowed by settings.
	ErrWalletFeeLimit = NewErrorWithCode(ErrWalletFeeLimitCode, "Fee limit exceeded")
	// ErrNoOpenedWallet represents an error with code [ErrNoOpenedWalletCode]. Can be returned only by the C# RPC server.
	// Server doesn't have any opened wallet to operate with.
	ErrNoOpenedWallet = NewErrorWithCode(ErrNoOpenedWalletCode, "No opened wallet")
	// ErrWalletNotFound represents an error with code [ErrWalletNotFoundCode]. Can be returned only by the C# RPC server.
	// Specified (or configured) wallet file path is invalid.
	ErrWalletNotFound = NewErrorWithCode(ErrWalletNotFoundCode, "Wallet not found")
	// ErrWalletNotSupported represents an error with code [ErrWalletNotSupportedCode]. Can be returned only by the C# RPC server.
	// Specified (or configured) file can't be opened as a wallet.
	ErrWalletNotSupported = NewErrorWithCode(ErrWalletNotSupportedCode, "Wallet not supported")

	// ErrVerificationFailed represents an error with code [ErrVerificationFailedCode].
	// Any verification error that can't be expressed by other codes.
	ErrVerificationFailed = NewErrorWithCode(ErrVerificationFailedCode, "Unclassified inventory verification error")
	// ErrAlreadyExists represents an error with code [ErrAlreadyExistsCode].
	// Block or transaction is already accepted and processed on chain.
	ErrAlreadyExists = NewErrorWithCode(ErrAlreadyExistsCode, "Inventory already exists on chain")
	// ErrMempoolCapReached represents an error with code [ErrMempoolCapReachedCode].
	// No more transactions can be accepted into the memory pool (unless they have a priority) as its full capacity is reached.
	ErrMempoolCapReached = NewErrorWithCode(ErrMempoolCapReachedCode, "The memory pool is full and no more transactions can be sent")
	// ErrAlreadyInPool represents an error with code [ErrAlreadyInPoolCode].
	// Transaction is already pooled, but not yet accepted into a block.
	ErrAlreadyInPool = NewErrorWithCode(ErrAlreadyInPoolCode, "Transaction already exists in the memory pool")
	// ErrInsufficientNetworkFee represents an error with code [ErrInsufficientNetworkFeeCode].
	// Transaction has incorrect (too small per Policy setting) network fee value.
	ErrInsufficientNetworkFee = NewErrorWithCode(ErrInsufficientNetworkFeeCode, "Insufficient network fee")
	// ErrPolicyFailed represents an error with code [ErrPolicyFailedCode].
	// Denied by the Policy contract (one of signers is blocked).
	ErrPolicyFailed = NewErrorWithCode(ErrPolicyFailedCode, "One of the Policy filters failed")
	// ErrInvalidScript represents an error with code [ErrInvalidScriptCode].
	// Transaction contains incorrect executable script.
	ErrInvalidScript = NewErrorWithCode(ErrInvalidScriptCode, "Invalid script")
	// ErrInvalidAttribute represents an error with code [ErrInvalidAttributeCode].
	// Transaction contains an invalid attribute.
	ErrInvalidAttribute = NewErrorWithCode(ErrInvalidAttributeCode, "Invalid transaction attribute")
	// ErrInvalidSignature represents an error with code [ErrInvalidSignatureCode].
	// One of the verification scripts failed.
	ErrInvalidSignature = NewErrorWithCode(ErrInvalidSignatureCode, "Invalid signature")
	// ErrInvalidSize represents an error with code [ErrInvalidSizeCode].
	// Transaction or its script is too big.
	ErrInvalidSize = NewErrorWithCode(ErrInvalidSizeCode, "Invalid inventory size")
	// ErrExpiredTransaction represents an error with code [ErrExpiredTransactionCode].
	// Transaction's ValidUntilBlock value is already in the past.
	ErrExpiredTransaction = NewErrorWithCode(ErrExpiredTransactionCode, "Expired transaction")
	// ErrInsufficientFunds represents an error with code [ErrInsufficientFundsCode].
	// Sender doesn't have enough GAS to pay for all currently pooled transactions.
	ErrInsufficientFunds = NewErrorWithCode(ErrInsufficientFundsCode, "Insufficient funds")
	// ErrInvalidVerificationFunction represents an error with code [ErrInvalidVerificationFunctionCode].
	// Contract doesn't have a verify method or this method doesn't return proper value.
	ErrInvalidVerificationFunction = NewErrorWithCode(ErrInvalidVerificationFunctionCode, "Invalid verification function")

	// ErrSessionsDisabled represents an error with code [ErrSessionsDisabledCode].
	// Iterator session support is not enabled on the server.
	ErrSessionsDisabled = NewErrorWithCode(ErrSessionsDisabledCode, "Sessions disabled")
	// ErrOracleDisabled represents an error with code [ErrOracleDisabledCode].
	// Service is not enabled in the configuration.
	ErrOracleDisabled = NewErrorWithCode(ErrOracleDisabledCode, "Oracle service is not running")
	// ErrOracleRequestFinished represents an error with code [ErrOracleRequestFinishedCode]. Can be returned only by the C# RPC server.
	// The oracle request submitted is already completely processed.
	ErrOracleRequestFinished = NewErrorWithCode(ErrOracleRequestFinishedCode, "Oracle request has already been finished")
	// ErrOracleRequestNotFound represents an error with code [ErrOracleRequestNotFoundCode]. Can be returned only by the C# RPC server.
	// The oracle request submitted is not known to this node.
	ErrOracleRequestNotFound = NewErrorWithCode(ErrOracleRequestNotFoundCode, "Oracle request is not found")
	// ErrOracleNotDesignatedNode represents an error with code [ErrOracleNotDesignatedNodeCode]. Can be returned only by the C# RPC server.
	// Oracle service is enabled, but this node is not designated to provide this functionality.
	ErrOracleNotDesignatedNode = NewErrorWithCode(ErrOracleNotDesignatedNodeCode, "Not a designated oracle node")
	// ErrUnsupportedState represents an error with code [ErrUnsupportedStateCode].
	// This node can't answer requests for old state because it's configured to keep only the latest one.
	ErrUnsupportedState = NewErrorWithCode(ErrUnsupportedStateCode, "Old state requests are not supported")
	// ErrInvalidProof represents an error with code [ErrInvalidProofCode].
	// State proof verification failed.
	ErrInvalidProof = NewErrorWithCode(ErrInvalidProofCode, "Invalid proof")
	// ErrExecutionFailed represents an error with code [ErrExecutionFailedCode].
	// Call made a VM execution, but it has failed.
	ErrExecutionFailed = NewErrorWithCode(ErrExecutionFailedCode, "Execution failed")
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
	return NewError(BadRequestCode, "Parse error", data)
}

// NewInvalidRequestError creates a new error with
// code -32600.
func NewInvalidRequestError(data string) *Error {
	return NewError(InvalidRequestCode, "Invalid request", data)
}

// NewMethodNotFoundError creates a new error with
// code -32601.
func NewMethodNotFoundError(data string) *Error {
	return NewError(MethodNotFoundCode, "Method not found", data)
}

// NewInvalidParamsError creates a new error with
// code -32602.
func NewInvalidParamsError(data string) *Error {
	return NewError(InvalidParamsCode, "Invalid params", data)
}

// NewInternalServerError creates a new error with
// code -32603.
func NewInternalServerError(data string) *Error {
	return NewError(InternalServerErrorCode, "Internal error", data)
}

// NewErrorWithCode creates a new error with
// specified error code and error message.
func NewErrorWithCode(code int64, message string) *Error {
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
	var clTarget *Error
	if errors.As(target, &clTarget) {
		return e.Code == clTarget.Code
	}
	return false
}
