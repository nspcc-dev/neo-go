package enumerator

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Create creates an enumerator from an array-like or bytearray-like stack item.
func Create(ic *interop.Context) error {
	return vm.EnumeratorCreate(ic.VM)
}

// Next advances the enumerator, pushes true if is it was successful
// and false otherwise.
func Next(ic *interop.Context) error {
	return vm.EnumeratorNext(ic.VM)
}

// Value returns the current value of the enumerator.
func Value(ic *interop.Context) error {
	return vm.EnumeratorValue(ic.VM)
}
