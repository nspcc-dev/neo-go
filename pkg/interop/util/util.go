/*
Package util contains some special useful functions that are provided by compiler and VM.
*/
package util

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Abort terminates current execution, unlike exception throwing with panic() it
// can't be recovered from.
func Abort() {
	neogointernal.Opcode0NoReturn("ABORT")
}

// Equals compares a with b and will return true when a and b are equal. It's
// implemented as an EQUAL VM opcode, so the rules of comparison are those
// of EQUAL.
func Equals(a, b interface{}) bool {
	return neogointernal.Opcode2("EQUAL", a, b).(bool)
}

// Remove removes element with index i from slice.
// This is done in place and slice must have type other than `[]byte`.
func Remove(slice interface{}, i int) {
	neogointernal.Opcode2NoReturn("REMOVE", slice, i)
}
