package runtime

// CheckWitness verifies if the given hash is the invoker of the contract
func CheckWitness(hash []byte) bool {
	return true
}

// Notify passes data to the VM
func Notify(arg interface{}) int {
	return 0
}

// Log passes a message to the VM
func Log(message string) {}

// Application returns the Application trigger type
func Application() byte {
	return 0x10
}

// Verification returns the Verification trigger type
func Verification() byte {
	return 0x00
}

// GetTrigger return the current trigger type. The return in this function
// Doesn't really matter, this is just an interop placeholder
func GetTrigger() interface{} {
	return 0
}
