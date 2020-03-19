package native

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Deploy deploys native contract.
func Deploy(ic *interop.Context, _ *vm.VM) error {
	if ic.Block.Index != 0 {
		return errors.New("native contracts can be deployed only at 0 block")
	}
	return nil
}
