package vm

import (
	"errors"

	"github.com/holiman/uint256"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// exceptionHandlingState represents state of the exception handling process.
type exceptionHandlingState byte

const (
	eTry exceptionHandlingState = iota
	eCatch
	eFinally
)

// exceptionHandlingContext represents context of the exception handling process.
type exceptionHandlingContext struct {
	CatchOffset   int
	FinallyOffset int
	EndOffset     int
	State         exceptionHandlingState
}

func newExceptionHandlingContext(cOffset, fOffset int) *exceptionHandlingContext {
	return &exceptionHandlingContext{
		CatchOffset:   cOffset,
		FinallyOffset: fOffset,
		EndOffset:     -1,
		State:         eTry,
	}
}

// HasCatch returns true iff the context has a `catch` block.
func (c *exceptionHandlingContext) HasCatch() bool { return c.CatchOffset >= 0 }

// HasFinally returns true iff the context has a `finally` block.
func (c *exceptionHandlingContext) HasFinally() bool { return c.FinallyOffset >= 0 }

// String implements the stackitem.Item interface.
func (c *exceptionHandlingContext) String() string {
	return "exception handling context"
}

// Value implements the stackitem.Item interface.
func (c *exceptionHandlingContext) Value() interface{} {
	return c
}

// Dup implements the stackitem.Item interface.
func (c *exceptionHandlingContext) Dup() stackitem.Item {
	return c
}

// TryBool implements the stackitem.Item interface.
func (c *exceptionHandlingContext) TryBool() (bool, error) {
	panic("can't convert exceptionHandlingContext to Bool")
}

// TryBytes implements the stackitem.Item interface.
func (c *exceptionHandlingContext) TryBytes() ([]byte, error) {
	return nil, errors.New("can't convert exceptionHandlingContext to ByteArray")
}

// TryInteger implements the stackitem.Item interface.
func (c *exceptionHandlingContext) TryInteger() (*uint256.Int, error) {
	return nil, errors.New("can't convert exceptionHandlingContext to Integer")
}

// Type implements the stackitem.Item interface.
func (c *exceptionHandlingContext) Type() stackitem.Type {
	panic("exceptionHandlingContext cannot appear on evaluation stack")
}

// Convert implements the stackitem.Item interface.
func (c *exceptionHandlingContext) Convert(_ stackitem.Type) (stackitem.Item, error) {
	panic("exceptionHandlingContext cannot be converted to anything")
}

// Equals implements the stackitem.Item interface.
func (c *exceptionHandlingContext) Equals(s stackitem.Item) bool {
	return c == s
}
