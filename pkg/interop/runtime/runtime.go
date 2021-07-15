/*
Package runtime provides various service functions related to execution environment.
It has similar function to Runtime class in .net framwork for Neo.
*/
package runtime

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
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
func Notify(name string, args ...interface{}) {
	neogointernal.Syscall2NoReturn("System.Runtime.Notify", name, args)
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

// GetNotifications returns notifications emitted by contract h.
// 'nil' literal means no filtering. It returns slice consisting of following elements:
// [  scripthash of notification's contract  ,  emitted item  ].
// This function uses `System.Runtime.GetNotifications` syscall.
func GetNotifications(h interop.Hash160) [][]interface{} {
	return neogointernal.Syscall1("System.Runtime.GetNotifications", h).([][]interface{})
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
