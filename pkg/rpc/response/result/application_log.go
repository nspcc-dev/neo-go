package result

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// ApplicationLog wrapper used for the representation of the
// state.AppExecResult based on the specific tx on the RPC Server.
type ApplicationLog struct {
	TxHash      util.Uint256
	Trigger     string
	VMState     string
	GasConsumed int64
	Stack       []stackitem.Item
	Events      []NotificationEvent
}

//NotificationEvent response wrapper
type NotificationEvent struct {
	Contract util.Uint160            `json:"contract"`
	Name     string                  `json:"eventname"`
	Item     smartcontract.Parameter `json:"state"`
}

type applicationLogAux struct {
	TxHash      util.Uint256        `json:"txid"`
	Trigger     string              `json:"trigger"`
	VMState     string              `json:"vmstate"`
	GasConsumed int64               `json:"gasconsumed,string"`
	Stack       []json.RawMessage   `json:"stack"`
	Events      []NotificationEvent `json:"notifications"`
}

// MarshalJSON implements json.Marshaler.
func (l ApplicationLog) MarshalJSON() ([]byte, error) {
	arr := make([]json.RawMessage, len(l.Stack))
	for i := range arr {
		data, err := stackitem.ToJSONWithTypes(l.Stack[i])
		if err != nil {
			return nil, err
		}
		arr[i] = data
	}
	return json.Marshal(&applicationLogAux{
		TxHash:      l.TxHash,
		Trigger:     l.Trigger,
		VMState:     l.VMState,
		GasConsumed: l.GasConsumed,
		Stack:       arr,
		Events:      l.Events,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (l *ApplicationLog) UnmarshalJSON(data []byte) error {
	aux := new(applicationLogAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	st := make([]stackitem.Item, len(aux.Stack))
	var err error
	for i := range st {
		st[i], err = stackitem.FromJSONWithTypes(aux.Stack[i])
		if err != nil {
			return err
		}
	}
	l.Stack = st
	l.Trigger = aux.Trigger
	l.TxHash = aux.TxHash
	l.VMState = aux.VMState
	l.Events = aux.Events
	l.GasConsumed = aux.GasConsumed

	return nil
}

// StateEventToResultNotification converts state.NotificationEvent to
// result.NotificationEvent.
func StateEventToResultNotification(event state.NotificationEvent) NotificationEvent {
	seen := make(map[stackitem.Item]bool)
	item := smartcontract.ParameterFromStackItem(event.Item, seen)
	return NotificationEvent{
		Contract: event.ScriptHash,
		Name:     event.Name,
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
