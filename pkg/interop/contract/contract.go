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

// IsStandard checks if contract with provided hash is a standard signature/multisig contract.
// This function uses `System.Contract.IsStandard` syscall.
func IsStandard(h interop.Hash160) bool {
	return neogointernal.Syscall1("System.Contract.IsStandard", h).(bool)
}

// CreateMultisigAccount calculates script hash of an m out of n multisignature
// script using given m and a set of public keys bytes. This function uses
// `System.Contract.CreateMultisigAccount` syscall.
func CreateMultisigAccount(m int, pubs []interop.PublicKey) []byte {
	return neogointernal.Syscall2("System.Contract.CreateMultisigAccount", m, pubs).([]byte)
}

// CreateStandardAccount calculates script hash of a given public key.
// This function uses `System.Contract.CreateStandardAccount` syscall.
func CreateStandardAccount(pub interop.PublicKey) []byte {
	return neogointernal.Syscall1("System.Contract.CreateStandardAccount", pub).([]byte)
}

// GetCallFlags returns calling flags which execution context was created with.
// This function uses `System.Contract.GetCallFlags` syscall.
func GetCallFlags() CallFlag {
	return neogointernal.Syscall0("System.Contract.GetCallFlags").(CallFlag)
}

// Call executes previously deployed blockchain contract with specified hash
// (20 bytes in BE form) using provided arguments and call flags.
// It returns whatever this contract returns. This function uses
// `System.Contract.Call` syscall.
func Call(scriptHash interop.Hash160, method string, f CallFlag, args ...interface{}) interface{} {
	return neogointernal.Syscall4("System.Contract.Call", scriptHash, method, f, args)
}
