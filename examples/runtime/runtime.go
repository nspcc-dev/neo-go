package runtimecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/lib/address"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

var (
	// Check if the invoker of the contract is the specified owner
	owner = address.ToHash160("NbrUYaZgyhSkNoRo9ugRyEMdUZxrhkNaWB")
)

// init is transformed into _initialize method that is called whenever contract
// is being loaded (so you'll see this log entry with every invocation).
func init() {
	// No events and logging allowed in verification context.
	if runtime.GetTrigger() != runtime.Verification {
		runtime.Log("init called")
	}
}

// _deploy is called after contract deployment or update, it'll be called
// in deployment transaction and if call update method of this contract.
func _deploy(_ any, isUpdate bool) {
	if isUpdate {
		Log("_deploy method called after contract update")
		return
	}
	Log("_deploy method called after contract creation")
}

// CheckWitness checks owner's witness. It returns true if invoked by the owner
// and false otherwise.
func CheckWitness() bool {
	if runtime.CheckWitness(owner) {
		runtime.Log("Verified Owner")
		return true
	}
	return false
}

// Log logs the given message.
func Log(message string) {
	runtime.Log(message)
}

// Notify emits an event with the specified data.
func Notify(event any) {
	runtime.Notify("Event", event)
}

// Verify method is used when the contract is being used as a signer of transaction,
// it can have parameters (that then need to be present in invocation script)
// and it returns simple pass/fail result. This implementation just checks for
// the owner's signature presence.
func Verify() bool {
	// Technically, this restriction is not needed, but you can see the difference
	// between invokefunction and invokecontractverify RPC methods with it.
	if runtime.GetTrigger() != runtime.Verification {
		return false
	}
	return CheckWitness()
}

// Destroy destroys the contract, only the owner can do that.
func Destroy() {
	if !CheckWitness() {
		panic("only owner can destroy")
	}
	management.Destroy()
}

// Update updates the contract, only the owner can do that. _deploy will be called
// after update.
func Update(nef, manifest []byte) {
	if !CheckWitness() {
		panic("only owner can update")
	}
	management.Update(nef, manifest)
}
