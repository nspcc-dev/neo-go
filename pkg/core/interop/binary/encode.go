package binary

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Serialize serializes top stack item into a ByteArray.
func Serialize(ic *interop.Context) error {
	return vm.RuntimeSerialize(ic.VM)
}

// Deserialize deserializes ByteArray from a stack into an item.
func Deserialize(ic *interop.Context) error {
	return vm.RuntimeDeserialize(ic.VM)
}
