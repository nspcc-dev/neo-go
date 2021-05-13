package trigger

import (
	"fmt"
	"strings"
)

//go:generate stringer -type=Type -output=trigger_type_string.go

// Type represents trigger type used in C# reference node: https://github.com/neo-project/neo/blob/c64748ecbac3baeb8045b16af0d518398a6ced24/neo/SmartContract/TriggerType.cs#L3
type Type byte

// Viable list of supported trigger type constants.
const (
	// OnPersist is a trigger type that indicates that script is being invoked
	// internally by the system during block persistence (before transaction
	// processing).
	OnPersist Type = 0x01

	// PostPersist is a trigger type that indicates that script is being invoked
	// by the system after block persistence (transcation processing) has
	// finished.
	PostPersist Type = 0x02

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
	All Type = OnPersist | PostPersist | Verification | Application
)

// FromString converts string to trigger Type.
func FromString(str string) (Type, error) {
	triggers := []Type{OnPersist, PostPersist, Verification, Application, All}
	str = strings.ToLower(str)
	for _, t := range triggers {
		if strings.ToLower(t.String()) == str {
			return t, nil
		}
	}
	return 0, fmt.Errorf("unknown trigger type: %s", str)
}
