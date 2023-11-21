package native

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Call calls the specified native contract method.
func Call(ic *interop.Context) error {
	version := ic.VM.Estack().Pop().BigInt().Int64()
	if version != 0 {
		return fmt.Errorf("native contract of version %d is not active", version)
	}
	var (
		c    interop.Contract
		curr = ic.VM.GetCurrentScriptHash()
	)
	for _, ctr := range ic.Natives {
		if ctr.Metadata().Hash == curr {
			c = ctr
			break
		}
	}
	if c == nil {
		return fmt.Errorf("native contract %s (version %d) not found", curr.StringLE(), version)
	}
	var (
		meta     = c.Metadata()
		activeIn = c.ActiveIn()
	)
	if activeIn != nil {
		height, ok := ic.Hardforks[activeIn.String()]
		// Persisting block must not be taken into account, native contract can be called
		// only AFTER its initialization block persist, thus, can't use ic.IsHardforkEnabled.
		if !ok || ic.BlockHeight() < height {
			return fmt.Errorf("native contract %s is active after hardfork %s", meta.Name, activeIn.String())
		}
	}
	m, ok := meta.GetMethodByOffset(ic.VM.Context().IP())
	if !ok {
		return fmt.Errorf("method not found")
	}
	reqFlags := m.RequiredFlags
	if !ic.IsHardforkEnabled(config.HFAspidochelone) && meta.ID == ManagementContractID &&
		(m.MD.Name == "deploy" || m.MD.Name == "update") {
		reqFlags &= callflag.States | callflag.AllowNotify
	}
	if !ic.VM.Context().GetCallFlags().Has(reqFlags) {
		return fmt.Errorf("missing call flags for native %d `%s` operation call: %05b vs %05b",
			version, m.MD.Name, ic.VM.Context().GetCallFlags(), reqFlags)
	}
	invokeFee := m.CPUFee*ic.BaseExecFee() +
		m.StorageFee*ic.BaseStorageFee()
	if !ic.VM.AddGas(invokeFee) {
		return errors.New("gas limit exceeded")
	}
	ctx := ic.VM.Context()
	args := make([]stackitem.Item, len(m.MD.Parameters))
	for i := range args {
		args[i] = ic.VM.Estack().Peek(i).Item()
	}
	result := m.Func(ic, args)
	for range m.MD.Parameters {
		ic.VM.Estack().Pop()
	}
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
		activeIn := c.ActiveIn()
		if !(activeIn == nil || ic.IsHardforkEnabled(*activeIn)) {
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
		activeIn := c.ActiveIn()
		if !(activeIn == nil || ic.IsHardforkEnabled(*activeIn)) {
			continue
		}
		err := c.PostPersist(ic)
		if err != nil {
			return err
		}
	}
	return nil
}
