package runtimecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

var (
	// Check if the invoker of the contract is the specified owner
	owner   = util.FromAddress("NULwe3UAHckN2fzNdcVg31tDiaYtMDwANt")
	trigger byte
)

// init initializes trigger before any other contract method is called
func init() {
	trigger = runtime.GetTrigger()
}

func _deploy(_ interface{}, isUpdate bool) {
	if isUpdate {
		Log("_deploy method called before contract update")
		return
	}
	Log("_deploy method called before contract creation")
}

// CheckWitness checks owner's witness
func CheckWitness() bool {
	// Log owner upon Verification trigger
	if trigger != runtime.Verification {
		return false
	}
	if runtime.CheckWitness(owner) {
		runtime.Log("Verified Owner")
	}
	return true
}

// Log logs given message
func Log(message string) bool {
	if trigger != runtime.Application {
		return false
	}
	runtime.Log(message)
	return true
}

// Notify notifies about given message
func Notify(event interface{}) bool {
	if trigger != runtime.Application {
		return false
	}
	runtime.Notify("Event", event)
	return true
}
