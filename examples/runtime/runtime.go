package runtimecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/util"
)

// Check if the invoker of the contract is the specified owner
var owner = util.FromAddress("Nis7Cu1Qn6iBb8kbeQ5HgdZT7AsQPqywTC")

// Main is something to be ran from outside.
func Main(operation string, args []interface{}) bool {
	trigger := runtime.GetTrigger()

	// Log owner upon Verification trigger
	if trigger == runtime.Verification {
		return CheckWitness()
	}

	// Discerns between log and notify for this test
	if trigger == runtime.Application {
		return handleOperation(operation, args)
	}

	return false
}

func handleOperation(operation string, args []interface{}) bool {
	if operation == "log" {
		return Log(args)
	}

	if operation == "notify" {
		return Notify(args)
	}

	return false
}

// CheckWitness checks owner's witness
func CheckWitness() bool {
	if runtime.CheckWitness(owner) {
		runtime.Log("Verified Owner")
	}
	return true
}

// Log logs given message
func Log(args []interface{}) bool {
	message := args[0].(string)
	runtime.Log(message)
	return true
}

// Notify notifies about given message
func Notify(args []interface{}) bool {
	runtime.Notify("Event", args[0])
	return true
}
