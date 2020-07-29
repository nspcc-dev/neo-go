package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// ApplicationLog wrapper used for the representation of the
// state.AppExecResult based on the specific tx on the RPC Server.
type ApplicationLog struct {
	TxHash      util.Uint256              `json:"txid"`
	Trigger     string                    `json:"trigger"`
	VMState     string                    `json:"vmstate"`
	GasConsumed int64                     `json:"gasconsumed,string"`
	Stack       []smartcontract.Parameter `json:"stack"`
	Events      []NotificationEvent       `json:"notifications"`
}

//NotificationEvent response wrapper
type NotificationEvent struct {
	Contract util.Uint160            `json:"contract"`
	Item     smartcontract.Parameter `json:"state"`
}

// StateEventToResultNotification converts state.NotificationEvent to
// result.NotificationEvent.
func StateEventToResultNotification(event state.NotificationEvent) NotificationEvent {
	seen := make(map[stackitem.Item]bool)
	args := stackitem.NewArray([]stackitem.Item{
		stackitem.Make(event.Name),
		event.Item,
	})
	item := smartcontract.ParameterFromStackItem(args, seen)
	return NotificationEvent{
		Contract: event.ScriptHash,
		Item:     item,
	}
}

// NewApplicationLog creates a new ApplicationLog wrapper.
func NewApplicationLog(appExecRes *state.AppExecResult) ApplicationLog {
	events := make([]NotificationEvent, 0, len(appExecRes.Events))
	for _, e := range appExecRes.Events {
		events = append(events, StateEventToResultNotification(e))
	}

	return ApplicationLog{
		TxHash:      appExecRes.TxHash,
		Trigger:     appExecRes.Trigger.String(),
		VMState:     appExecRes.VMState.String(),
		GasConsumed: appExecRes.GasConsumed,
		Stack:       appExecRes.Stack,
		Events:      events,
	}
}
