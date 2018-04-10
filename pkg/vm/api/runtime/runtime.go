package runtime

import "github.com/CityOfZion/neo-go/pkg/vm/api/types"

// CheckWitness verifies if the invoker is the owner of the contract.
func CheckWitness(hash []byte) bool {
	return true
}

// GetCurrentBlock returns the current block.
func GetCurrentBlock() types.Block { return types.Block{} }

// GetTime returns the timestamp of the most recent block.
func GetTime() int {
	return 0
}

// Notify an event to the VM.
func Notify(arg interface{}) int {
	return 0
}

// Log intructs the VM to log the given message.
func Log(message string) {}

// Application returns the application trigger type.
func Application() byte {
	return 0x10
}

// Verification returns the verification trigger type.
func Verification() byte {
	return 0x00
}

// GetTrigger return the current trigger type. The return in this function
// doesn't really mather, this is just an interop placeholder.
func GetTrigger() interface{} {
	return 0
}
