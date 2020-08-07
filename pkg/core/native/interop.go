package native

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
)

// Deploy deploys native contract.
func Deploy(ic *interop.Context) error {
	if ic.Block == nil || ic.Block.Index != 0 {
		return errors.New("native contracts can be deployed only at 0 block")
	}

	for _, native := range ic.Natives {
		md := native.Metadata()

		cs := &state.Contract{
			ID:       md.ContractID,
			Script:   md.Script,
			Manifest: md.Manifest,
		}
		if err := ic.DAO.PutContractState(cs); err != nil {
			return err
		}
		if err := native.Initialize(ic); err != nil {
			return fmt.Errorf("initializing %s native contract: %v", md.Name, err)
		}
	}
	return nil
}

// Call calls specified native contract method.
func Call(ic *interop.Context) error {
	name := ic.VM.Estack().Pop().String()
	var c interop.Contract
	for _, ctr := range ic.Natives {
		if ctr.Metadata().Name == name {
			c = ctr
			break
		}
	}
	if c == nil {
		return fmt.Errorf("native contract %s not found", name)
	}
	h := ic.VM.GetCurrentScriptHash()
	if !h.Equals(c.Metadata().Hash) {
		return errors.New("it is not allowed to use Neo.Native.Call directly to call native contracts. System.Contract.Call should be used")
	}
	operation := ic.VM.Estack().Pop().String()
	args := ic.VM.Estack().Pop().Array()
	m, ok := c.Metadata().Methods[operation]
	if !ok {
		return fmt.Errorf("method %s not found", operation)
	}
	if !ic.VM.Context().GetCallFlags().Has(m.RequiredFlags) {
		return errors.New("missing call flags")
	}
	if !ic.VM.AddGas(m.Price) {
		return errors.New("gas limit exceeded")
	}
	result := m.Func(ic, args)
	ic.VM.Estack().PushVal(result)
	return nil
}
