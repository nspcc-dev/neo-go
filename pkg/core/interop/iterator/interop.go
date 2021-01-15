package iterator

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Create creates an iterator from array-like or map stack item.
func Create(ic *interop.Context) error {
	return vm.IteratorCreate(ic.VM)
}

// Next advances the iterator, pushes true on success and false otherwise.
func Next(ic *interop.Context) error {
	return vm.IteratorNext(ic.VM)
}

// Value returns current iterator value and depends on iterator type:
// For slices the result is just value.
// For maps the result is key-value pair packed in a struct.
func Value(ic *interop.Context) error {
	return vm.IteratorValue(ic.VM)
}
