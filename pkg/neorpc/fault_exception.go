package neorpc

import (
	"strings"
)

// gasLimitExceededException is a string representation of the
// [vm.ErrGASLimitExceeded] error. It's not imported from vm directly since
// neorpc package shouldn't be dependent from vm.
const gasLimitExceededException = "GAS limit exceeded"

// FaultException represents an exception that may happen during VM script
// invocation. FaultException implements the error interface and hence, may be
// used to distinguish various types of VM's fault exceptions in
// [state.Execution] or [result.Invoke]. Note that this exception is
// implementation-specific in most of the cases and can be used with either
// Go or C# node only since the exception message text depends on the node
// implementation. Also note that this exception can not be reliably
// distinguished from the one raised by smart contract if its text matches
// the one used by the smart contract.
type FaultException struct {
	Message string
}

// A set of named fault exceptions.
var (
	// GASLimitExceededException is raised whenever there's not enough GAS to
	// pay for the instruction execution. This exception is NeoGo-specific,
	// check [FaultException] documentation for more details.
	GASLimitExceededException = &FaultException{Message: gasLimitExceededException}
)

// Error implements the error interface.
func (e *FaultException) Error() string {
	return e.Message
}

// Is denotes whether the error matches the target one.
func (e *FaultException) Is(target error) bool {
	return strings.Contains(e.Message, target.Error())
}
