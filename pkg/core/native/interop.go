package native

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// Deploy deploys native contract.
func Deploy(ic *interop.Context, _ *vm.VM) error {
	if ic.Block.Index != 0 {
		return errors.New("native contracts can be deployed only at 0 block")
	}

	for _, native := range ic.Natives {
		md := native.Metadata()

		ps := md.Manifest.ABI.EntryPoint.Parameters
		params := make([]smartcontract.ParamType, len(ps))
		for i := range ps {
			params[i] = ps[i].Type
		}

		cs := &state.Contract{
			ID:       md.ContractID,
			Script:   md.Script,
			Manifest: md.Manifest,
		}
		if err := ic.DAO.PutContractState(cs); err != nil {
			return err
		}
		if err := native.Initialize(ic); err != nil {
			return fmt.Errorf("initializing %s native contract: %v", md.ServiceName, err)
		}
	}
	return nil
}
