package result

import (
	"encoding/json"

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
	Trigger     string              `json:"trigger"`
	ScriptHash  util.Uint160        `json:"contract"`
	VMState     string              `json:"vmstate"`
	GasConsumed util.Fixed8         `json:"gas_consumed"`
	Stack       json.RawMessage     `json:"stack"`
	Events      []NotificationEvent `json:"notifications"`
}

//NotificationEvent response wrapper
type NotificationEvent struct {
	Contract util.Uint160            `json:"contract"`
	Item     smartcontract.Parameter `json:"state"`
}

// NewApplicationLog creates a new ApplicationLog wrapper.
func NewApplicationLog(appExecRes *state.AppExecResult, scriptHash util.Uint160) ApplicationLog {
	events := make([]NotificationEvent, 0, len(appExecRes.Events))
	for _, e := range appExecRes.Events {
		seen := make(map[vm.StackItem]bool)
		item := e.Item.ToContractParameter(seen)
		events = append(events, NotificationEvent{
			Contract: e.ScriptHash,
			Item:     item,
		})
	}

	triggerString := appExecRes.Trigger.String()

	var rawStack json.RawMessage
	if len(appExecRes.Stack) != 0 {
		rawStack = json.RawMessage(appExecRes.Stack)
	} else {
		rawStack = json.RawMessage("[]")
	}
	executions := []Execution{{
		Trigger:     triggerString,
		ScriptHash:  scriptHash,
		VMState:     appExecRes.VMState,
		GasConsumed: appExecRes.GasConsumed,
		Stack:       rawStack,
		Events:      events,
	}}

	return ApplicationLog{
		TxHash:     appExecRes.TxHash,
		Executions: executions,
	}
}
