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
	gas := NewGAS()
	neo := NewNEO()
	neo.GAS = gas
	gas.NEO = neo

	if err := gas.Initialize(ic); err != nil {
		return fmt.Errorf("can't initialize GAS native contract: %v", err)
	}
	if err := neo.Initialize(ic); err != nil {
		return fmt.Errorf("can't initialize NEO native contract: %v", err)
	}
	return nil
}
