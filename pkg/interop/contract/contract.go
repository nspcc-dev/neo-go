/*
Package contract provides functions to work with contracts.
*/
package contract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// CallFlag specifies valid call flags.
type CallFlag byte

// Using `smartcontract` package from compiled contract requires moderate
// compiler refactoring, thus all flags are mirrored here.
const (
	ReadStates CallFlag = 1 << iota
	WriteStates
	AllowCall
	AllowNotify
	States            = ReadStates | WriteStates
	ReadOnly          = ReadStates | AllowCall
	All               = States | AllowCall | AllowNotify
	NoneFlag CallFlag = 0
)

// CreateMultisigAccount calculates a script hash of an m out of n multisignature
// script using the given m and a set of public keys bytes. This function uses
// `System.Contract.CreateMultisigAccount` syscall.
func CreateMultisigAccount(m int, pubs []interop.PublicKey) []byte {
	return neogointernal.Syscall2("System.Contract.CreateMultisigAccount", m, pubs).([]byte)
}

// CreateStandardAccount calculates a script hash of the given public key.
// This function uses `System.Contract.CreateStandardAccount` syscall.
func CreateStandardAccount(pub interop.PublicKey) []byte {
	return neogointernal.Syscall1("System.Contract.CreateStandardAccount", pub).([]byte)
}

// GetCallFlags returns the calling flags which execution context was created with.
// This function uses `System.Contract.GetCallFlags` syscall.
func GetCallFlags() CallFlag {
	return neogointernal.Syscall0("System.Contract.GetCallFlags").(CallFlag)
}

// Call executes the previously deployed blockchain contract with the specified hash
// (20 bytes in BE form) using the provided arguments and call flags.
// It returns whatever this contract returns. This function uses
// `System.Contract.Call` syscall.
func Call(scriptHash interop.Hash160, method string, f CallFlag, args ...any) any {
	return neogointernal.Syscall4("System.Contract.Call", scriptHash, method, f, args)
}
