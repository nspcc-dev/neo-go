package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// ApplicationLog wrapper used for the representation of the
// state.AppExecResult based on the specific tx on the RPC Server.
type ApplicationLog struct {
	TxHash     util.Uint256 `json:"txid"`
	Executions []Execution  `json:"executions"`
}

// Execution response wrapper
type Execution struct {
	Trigger     string                    `json:"trigger"`
	ScriptHash  util.Uint160              `json:"contract"`
	VMState     string                    `json:"vmstate"`
	GasConsumed util.Fixed8               `json:"gas_consumed"`
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
	seen := make(map[vm.StackItem]bool)
	item := event.Item.ToContractParameter(seen)
	return NotificationEvent{
		Contract: event.ScriptHash,
		Item:     item,
	}
}

// NewApplicationLog creates a new ApplicationLog wrapper.
func NewApplicationLog(appExecRes *state.AppExecResult, scriptHash util.Uint160) ApplicationLog {
	events := make([]NotificationEvent, 0, len(appExecRes.Events))
	for _, e := range appExecRes.Events {
		events = append(events, StateEventToResultNotification(e))
	}

	triggerString := appExecRes.Trigger.String()

	executions := []Execution{{
		Trigger:     triggerString,
		ScriptHash:  scriptHash,
		VMState:     appExecRes.VMState,
		GasConsumed: appExecRes.GasConsumed,
		Stack:       appExecRes.Stack,
		Events:      events,
	}}

	return ApplicationLog{
		TxHash:     appExecRes.TxHash,
		Executions: executions,
	}
}
