package iterator

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Create creates an iterator from array-like or map stack item.
func Create(ic *interop.Context) error {
	return vm.IteratorCreate(ic.VM)
}

// Key returns current iterator key.
func Key(ic *interop.Context) error {
	return vm.IteratorKey(ic.VM)
}

// Keys returns keys of the iterator.
func Keys(ic *interop.Context) error {
	return vm.IteratorKeys(ic.VM)
}

// Values returns values of the iterator.
func Values(ic *interop.Context) error {
	return vm.IteratorValues(ic.VM)
}
