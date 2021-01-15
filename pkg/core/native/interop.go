package native

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

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
	m, ok := c.Metadata().Methods[operation]
	if !ok {
		return fmt.Errorf("method %s not found", operation)
	}
	if !ic.VM.Context().GetCallFlags().Has(m.RequiredFlags) {
		return fmt.Errorf("missing call flags for native %s `%s` operation call: %05b vs %05b", name, operation, ic.VM.Context().GetCallFlags(), m.RequiredFlags)
	}
	// Native contract prices are not multiplied by `BaseExecFee`.
	if !ic.VM.AddGas(m.Price) {
		return errors.New("gas limit exceeded")
	}
	ctx := ic.VM.Context()
	args := make([]stackitem.Item, len(m.MD.Parameters))
	for i := range args {
		args[i] = ic.VM.Estack().Pop().Item()
	}
	result := m.Func(ic, args)
	if m.MD.ReturnType != smartcontract.VoidType {
		ctx.Estack().PushVal(result)
	}
	return nil
}

// OnPersist calls OnPersist methods for all native contracts.
func OnPersist(ic *interop.Context) error {
	if ic.Trigger != trigger.OnPersist {
		return errors.New("onPersist must be trigered by system")
	}
	for _, c := range ic.Natives {
		err := c.OnPersist(ic)
		if err != nil {
			return err
		}
	}
	return nil
}

// PostPersist calls PostPersist methods for all native contracts.
func PostPersist(ic *interop.Context) error {
	if ic.Trigger != trigger.PostPersist {
		return errors.New("postPersist must be trigered by system")
	}
	for _, c := range ic.Natives {
		err := c.PostPersist(ic)
		if err != nil {
			return err
		}
	}
	return nil
}
