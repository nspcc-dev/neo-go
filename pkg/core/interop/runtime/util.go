package runtime

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/pkg/errors"
)

// GasLeft returns remaining amount of GAS.
func GasLeft(_ *interop.Context, v *vm.VM) error {
	v.Estack().PushVal(v.GasLimit - v.GasConsumed())
	return nil
}

// GetNotifications returns notifications emitted by current contract execution.
func GetNotifications(ic *interop.Context, v *vm.VM) error {
	item := v.Estack().Pop().Item()
	notifications := ic.Notifications
	if _, ok := item.(stackitem.Null); !ok {
		b, err := item.TryBytes()
		if err != nil {
			return err
		}
		u, err := util.Uint160DecodeBytesBE(b)
		if err != nil {
			return err
		}
		notifications = []state.NotificationEvent{}
		for i := range ic.Notifications {
			if ic.Notifications[i].ScriptHash.Equals(u) {
				notifications = append(notifications, ic.Notifications[i])
			}
		}
	}
	if len(notifications) > vm.MaxStackSize {
		return errors.New("too many notifications")
	}
	arr := stackitem.NewArray(make([]stackitem.Item, 0, len(notifications)))
	for i := range notifications {
		ev := stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(notifications[i].ScriptHash.BytesBE()),
			notifications[i].Item,
		})
		arr.Append(ev)
	}
	v.Estack().PushVal(arr)
	return nil
}

// GetInvocationCounter returns how many times current contract was invoked during current tx execution.
func GetInvocationCounter(ic *interop.Context, v *vm.VM) error {
	count, ok := ic.Invocations[v.GetCurrentScriptHash()]
	if !ok {
		return errors.New("current contract wasn't invoked from others")
	}
	v.Estack().PushVal(count)
	return nil
}
