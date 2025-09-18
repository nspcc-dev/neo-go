/*
Package runtime provides various service functions related to execution environment.
It has similar function to Runtime class in .net framwork for Neo.
*/
package runtime

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/ledger"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Trigger values to compare with GetTrigger result.
const (
	OnPersist    byte = 0x01
	PostPersist  byte = 0x02
	Application  byte = 0x40
	Verification byte = 0x20
)

// BurnGas burns provided amount of GAS. It uses `System.Runtime.BurnGas` syscall.
func BurnGas(gas int) {
	neogointernal.Syscall1NoReturn("System.Runtime.BurnGas", gas)
}

// CheckWitness verifies if the given script hash (160-bit BE value in a 20 byte
// slice) or key (compressed serialized 33-byte form) is one of the signers of
// this invocation. It uses `System.Runtime.CheckWitness` syscall.
func CheckWitness(hashOrKey []byte) bool {
	return neogointernal.Syscall1("System.Runtime.CheckWitness", hashOrKey).(bool)
}

// CurrentSigners returns signers of the currently loaded transaction or nil if
// executing script container is not a transaction. It uses
// `System.Runtime.CurrentSigners` syscall.
func CurrentSigners() []ledger.TransactionSigner {
	return neogointernal.Syscall0("System.Runtime.CurrentSigners").([]ledger.TransactionSigner)
}

// LoadScript loads the given bytecode into the VM and executes it with the
// given call flags and arguments. This bytecode is executed as is from byte 0,
// it's not a deployed contract that can have methods. The execution context is
// limited to read only actions ([contract.ReadOnly]) irrespective of provided
// call flags (you can only restrict them further with this option). An item is
// always returned from this call, either it's the one returned from the script
// (and it can only return one) or it's a Null stack item if the script returns
// nothing. Note that this is somewhat similar to [contract.Call], so the
// script can ABORT the transaction or THROW an exception, make sure you
// appropriately handle exceptions if bytecode comes from untrusted source.
// This function uses `System.Runtime.LoadScript` syscall.
func LoadScript(script []byte, f contract.CallFlag, args ...any) any {
	return neogointernal.Syscall3("System.Runtime.LoadScript", script, f, args)
}

// Log instructs VM to log the given message. It's mostly used for debugging
// purposes as these messages are not saved anywhere normally and usually are
// only visible in the VM logs. This function uses `System.Runtime.Log` syscall.
func Log(message string) {
	neogointernal.Syscall1NoReturn("System.Runtime.Log", message)
}

// Notify sends a notification (collecting all arguments in an array) to the
// executing environment. Unlike Log it can accept any data along with the event name
// and resulting notification is saved in application log. It's intended to be used as a
// part of contract's API to external systems, these events can be monitored
// from outside and act upon accordingly. This function uses
// `System.Runtime.Notify` syscall.
func Notify(name string, args ...any) {
	neogointernal.Syscall2NoReturn("System.Runtime.Notify", name, args)
}

// GetAddressVersion returns the address version of the current protocol. The
// address version represents the byte used to prepend to Neo addresses when
// encoding them. The default value for Neo3 is 53 (0x35). This function uses
// `System.Runtime.GetAddressVersion` syscall.
func GetAddressVersion() int {
	return neogointernal.Syscall0("System.Runtime.GetAddressVersion").(int)
}

// GetNetwork returns network magic number. This function uses
// `System.Runtime.GetNetwork` syscall.
func GetNetwork() int {
	return neogointernal.Syscall0("System.Runtime.GetNetwork").(int)
}

// GetTime returns the timestamp of the most recent block. Note that when running
// script in test mode this would be the last accepted (persisted) block in the
// chain, but when running as a part of the new block the time returned is the
// time of this (currently being processed) block. This function uses
// `System.Runtime.GetTime` syscall.
func GetTime() int {
	return neogointernal.Syscall0("System.Runtime.GetTime").(int)
}

// GetTrigger returns the smart contract invocation trigger which can be either
// verification or application. It can be used to differentiate running contract
// as a part of verification process from running it as a regular application.
// Some interop functions (especially ones that change the state in any way) are
// not available when running with verification trigger. This function uses
// `System.Runtime.GetTrigger` syscall.
func GetTrigger() byte {
	return neogointernal.Syscall0("System.Runtime.GetTrigger").(byte)
}

// GasLeft returns the amount of gas available for the current execution.
// This function uses `System.Runtime.GasLeft` syscall.
func GasLeft() int {
	return neogointernal.Syscall0("System.Runtime.GasLeft").(int)
}

func GasUsed() int {
	return neogointernal.Syscall0("System.Runtime.GasUsed").(int)
}

// GetNotifications returns notifications emitted by contract h.
// 'nil' literal means no filtering. It returns slice consisting of following elements:
// [  scripthash of notification's contract  ,  emitted item  ].
// This function uses `System.Runtime.GetNotifications` syscall.
func GetNotifications(h interop.Hash160) [][]any {
	return neogointernal.Syscall1("System.Runtime.GetNotifications", h).([][]any)
}

// GetInvocationCounter returns how many times current contract was invoked during current tx execution.
// This function uses `System.Runtime.GetInvocationCounter` syscall.
func GetInvocationCounter() int {
	return neogointernal.Syscall0("System.Runtime.GetInvocationCounter").(int)
}

// Platform returns the platform name, which is set to be `NEO`. This function uses
// `System.Runtime.Platform` syscall.
func Platform() []byte {
	return neogointernal.Syscall0("System.Runtime.Platform").([]byte)
}

// GetRandom returns pseudo-random number which depends on block nonce and tx hash.
// Each invocation will return a different number. This function uses
// `System.Runtime.GetRandom` syscall.
func GetRandom() int {
	return neogointernal.Syscall0("System.Runtime.GetRandom").(int)
}
