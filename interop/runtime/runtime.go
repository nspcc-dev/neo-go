package runtime

// Package runtime provides function signatures that can be used inside
// smart contracts that are written in the neo-storm framework.

// CheckWitness verifies if the given hash is the invoker of the contract.
func CheckWitness(hash []byte) bool {
	return true
}

// Log instucts the VM to log the given message.
func Log(message string) {}

// Notify an event to the VM.
func Notify(arg ...interface{}) int {
	return 0
}

// GetTime returns the timestamp of the most recent block.
func GetTime() int {
	return 0
}

// GetTrigger returns the smart contract invoke trigger which can be either
// verification or application.
func GetTrigger() byte {
	return 0x00
}

// Application returns the Application trigger type
func Application() byte {
	return 0x10
}

// Verification returns the Verification trigger type
func Verification() byte {
	return 0x00
}

// Serialize serializes and item into a bytearray.
func Serialize(item interface{}) []byte {
	return nil
}

// Deserialize an item from a bytearray.
func Deserialize(b []byte) interface{} {
	return nil
}
