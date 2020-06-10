/*
Package runtime provides various service functions related to execution environment.
It has similar function to Runtime class in .net framwork for Neo.
*/
package runtime

// CheckWitness verifies if the given script hash (160-bit BE value in a 20 byte
// slice) or key (compressed serialized 33-byte form) is one of the signers of
// this invocation. It uses `System.Runtime.CheckWitness` syscall.
func CheckWitness(hashOrKey []byte) bool {
	return true
}

// Log instructs VM to log the given message. It's mostly used for debugging
// purposes as these messages are not saved anywhere normally and usually are
// only visible in the VM logs. This function uses `System.Runtime.Log` syscall.
func Log(message string) {}

// Notify sends a notification (collecting all arguments in an array) to the
// executing environment. Unlike Log it can accept any data and resulting
// notification is saved in application log. It's intended to be used as a
// part of contract's API to external systems, these events can be monitored
// from outside and act upon accordingly. This function uses
// `System.Runtime.Notify` syscall.
func Notify(arg ...interface{}) {}

// GetTime returns the timestamp of the most recent block. Note that when running
// script in test mode this would be the last accepted (persisted) block in the
// chain, but when running as a part of the new block the time returned is the
// time of this (currently being processed) block. This function uses
// `System.Runtime.GetTime` syscall.
func GetTime() int {
	return 0
}

// GetTrigger returns the smart contract invocation trigger which can be either
// verification or application. It can be used to differentiate running contract
// as a part of verification process from running it as a regular application.
// Some interop functions (especially ones that change the state in any way) are
// not available when running with verification trigger. This function uses
// `System.Runtime.GetTrigger` syscall.
func GetTrigger() byte {
	return 0x00
}

// Application returns the Application trigger type value to compare with
// GetTrigger return value.
func Application() byte {
	return 0x10
}

// Verification returns the Verification trigger type value to compare with
// GetTrigger return value.
func Verification() byte {
	return 0x00
}

// Serialize serializes any given item into a byte slice. It works for all
// regular VM types (not ones from interop package) and allows to save them in
// storage or pass into Notify and then Deserialize them on the next run or in
// the external event receiver. It uses `System.Runtime.Serialize` syscall.
func Serialize(item interface{}) []byte {
	return nil
}

// Deserialize unpacks previously serialized value from a byte slice, it's the
// opposite of Serialize. It uses `System.Runtime.Deserialize` syscall.
func Deserialize(b []byte) interface{} {
	return nil
}
