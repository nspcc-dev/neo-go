package standard

import "github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"

// Standard represents smart-contract standard.
type Standard struct {
	// Manifest describes mandatory methods and events.
	manifest.Manifest
	// Base contains base standard.
	Base *Standard
	// Optional contains optional contract methods.
	// If contract contains method with the same name and parameter count,
	// it must have signature declared by this contract.
	Optional []manifest.Method
	// Required contains standards that are required for this standard.
	Required []string
}
