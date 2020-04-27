package native

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Deploy deploys native contract.
func Deploy(ic *interop.Context, _ *vm.VM) error {
	if ic.Block.Index != 0 {
		return errors.New("native contracts can be deployed only at 0 block")
	}

	for _, native := range ic.Natives {
		if err := native.Initialize(ic); err != nil {
			md := native.Metadata()
			return fmt.Errorf("initializing %s native contract: %v", md.ServiceName, err)
		}
	}
	return nil
}
