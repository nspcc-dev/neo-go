package runtime

import "github.com/CityOfZion/neo-go/pkg/core"

// T is shorthand for interface{} which allows us to use the function signatures
// in a more elegant way.
type T interface{}

// GetTrigger return the current trigger type. The return in this function
// doesn't really mather, this is just an interop placeholder.
func GetTrigger() T { return 0 }

// CheckWitness verifies if the invoker is the owner of the contract.
func CheckWitness(hash T) T { return 0 }

// GetCurrentBlock returns the current block.
func GetCurrentBlock() core.Block { return core.Block{} }

// GetTime returns the timestamp of the most recent block.
func GetTime() int { return 0 }

// Notify an event to the VM.
func Notify(arg T) T { return 0 }

// Log intructs the VM to log the given message.
func Log(message string) T { return 0 }

// Verification returns the verification trigger type.
func Verification() byte {
	return 0x00
}

// Application returns the application trigger type.
func Application() byte {
	return 0x10
}
