package trigger

import "fmt"

//go:generate stringer -type=Type -output=trigger_type_string.go

// Type represents trigger type used in C# reference node: https://github.com/neo-project/neo/blob/c64748ecbac3baeb8045b16af0d518398a6ced24/neo/SmartContract/TriggerType.cs#L3
type Type byte

// Viable list of supported trigger type constants.
const (
	// System is trigger type that indicates that script is being invoke internally by the system.
	System Type = 0x01

	// The verification trigger indicates that the contract is being invoked as a verification function.
	// The verification function can accept multiple parameters, and should return a boolean value that indicates the validity of the transaction or block.
	// The entry point of the contract will be invoked if the contract is triggered by Verification:
	//     main(...);
	// The entry point of the contract must be able to handle this type of invocation.
	Verification Type = 0x20

	// The application trigger indicates that the contract is being invoked as an application function.
	// The application function can accept multiple parameters, change the states of the blockchain, and return any type of value.
	// The contract can have any form of entry point, but we recommend that all contracts should have the following entry point:
	//     public byte[] main(string operation, params object[] args)
	// The functions can be invoked by creating an InvocationTransaction.
	Application Type = 0x40

	// All represents any trigger type.
	All Type = System | Verification | Application
)

// FromString converts string to trigger Type
func FromString(str string) (Type, error) {
	triggers := []Type{System, Verification, Application, All}
	for _, t := range triggers {
		if t.String() == str {
			return t, nil
		}
	}
	return 0, fmt.Errorf("unknown trigger type: %s", str)
}
