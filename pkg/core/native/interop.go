package native

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Call calls the specified native contract method.
func Call(ic *interop.Context) error {
	version := ic.VM.Estack().Pop().BigInt().Int64()
	if version != 0 {
		return fmt.Errorf("native contract of version %d is not active", version)
	}
	var c interop.Contract
	curr := ic.VM.GetCurrentScriptHash()
	for _, ctr := range ic.Natives {
		if ctr.Metadata().Hash == curr {
			c = ctr
			break
		}
	}
	if c == nil {
		return fmt.Errorf("native contract %s (version %d) not found", curr.StringLE(), version)
	}
	history := c.Metadata().UpdateHistory
	if len(history) == 0 {
		return fmt.Errorf("native contract %s is disabled", c.Metadata().Name)
	}
	if history[0] > ic.BlockHeight() {
		return fmt.Errorf("native contract %s is active after height = %d", c.Metadata().Name, history[0])
	}
	m, ok := c.Metadata().GetMethodByOffset(ic.VM.Context().IP())
	if !ok {
		return fmt.Errorf("method not found")
	}
	if !ic.VM.Context().GetCallFlags().Has(m.RequiredFlags) {
		return fmt.Errorf("missing call flags for native %d `%s` operation call: %05b vs %05b",
			version, m.MD.Name, ic.VM.Context().GetCallFlags(), m.RequiredFlags)
	}
	invokeFee := m.CPUFee*ic.BaseExecFee() +
		m.StorageFee*ic.BaseStorageFee()
	if !ic.VM.AddGas(invokeFee) {
		return errors.New("gas limit exceeded")
	}
	ctx := ic.VM.Context()
	args := make([]stackitem.Item, len(m.MD.Parameters))
	for i := range args {
		args[i] = ic.VM.Estack().Pop().Item()
	}
	result := m.Func(ic, args)
	if m.MD.ReturnType != smartcontract.VoidType {
		ctx.Estack().PushItem(result)
	}
	return nil
}

// OnPersist calls OnPersist methods for all native contracts.
func OnPersist(ic *interop.Context) error {
	if ic.Trigger != trigger.OnPersist {
		return errors.New("onPersist must be trigered by system")
	}
	for _, c := range ic.Natives {
		if !c.Metadata().IsActive(ic.Block.Index) {
			continue
		}
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
		if !c.Metadata().IsActive(ic.Block.Index) {
			continue
		}
		err := c.PostPersist(ic)
		if err != nil {
			return err
		}
	}
	return nil
}
