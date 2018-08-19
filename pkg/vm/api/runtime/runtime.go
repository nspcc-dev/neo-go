package runtime

// CheckWitness verifies if the invoker is the owner of the contract.
func CheckWitness(hash []byte) bool {
	return true
}

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

// Serialize serializes and item into a bytearray.
func Serialize(item interface{}) []byte {
	return nil
}

// Deserializes an item from a bytearray.
func Deserialize(b []byte) interface{} {
	return nil
}
