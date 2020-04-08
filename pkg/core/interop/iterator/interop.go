package iterator

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Concat concatenates 2 iterators into a single one.
func Concat(_ *interop.Context, v *vm.VM) error {
	return vm.IteratorConcat(v)
}

// Create creates an iterator from array-like or map stack item.
func Create(_ *interop.Context, v *vm.VM) error {
	return vm.IteratorCreate(v)
}

// Key returns current iterator key.
func Key(_ *interop.Context, v *vm.VM) error {
	return vm.IteratorKey(v)
}

// Keys returns keys of the iterator.
func Keys(_ *interop.Context, v *vm.VM) error {
	return vm.IteratorKeys(v)
}

// Values returns values of the iterator.
func Values(_ *interop.Context, v *vm.VM) error {
	return vm.IteratorValues(v)
}
