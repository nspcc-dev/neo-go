/*
Package util contains some special useful functions that are provided by compiler and VM.
*/
package util

import "github.com/nspcc-dev/neo-go/pkg/interop"

// FromAddress is an utility function that converts a Neo address to its hash
// (160 bit BE value in a 20 byte slice). It can only be used for strings known
// at compilation time, because the conversion is actually being done by the
// compiler.
func FromAddress(address string) interop.Hash160 {
	return nil
}

// Equals compares a with b and will return true when a and b are equal. It's
// implemented as an EQUAL VM opcode, so the rules of comparison are those
// of EQUAL.
func Equals(a, b interface{}) bool {
	return false
}
