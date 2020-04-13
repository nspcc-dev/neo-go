package enumerator

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Concat concatenates 2 enumerators into a single one.
func Concat(_ *interop.Context, v *vm.VM) error {
	return vm.EnumeratorConcat(v)
}

// Create creates an enumerator from an array-like stack item.
func Create(_ *interop.Context, v *vm.VM) error {
	return vm.EnumeratorCreate(v)
}

// Next advances the enumerator, pushes true if is it was successful
// and false otherwise.
func Next(_ *interop.Context, v *vm.VM) error {
	return vm.EnumeratorNext(v)
}

// Value returns the current value of the enumerator.
func Value(_ *interop.Context, v *vm.VM) error {
	return vm.EnumeratorValue(v)
}
