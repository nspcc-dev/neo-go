package state

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
)

const (
	// saveInvocationsBit acts as a marker using the VMState to indicate whether contract
	// invocations (tracked by the VM) have been stored into the DB and thus whether they
	// should be deserialized upon retrieval. This approach saves 1 byte for all
	// applicationlogs over	using WriteVarBytes. The original discussion can be found here
	// https://github.com/nspcc-dev/neo-go/pull/3569#discussion_r1909357541
	saveInvocationsBit = 0x80
	// cleanSaveInvocationsBitMask is used to remove the save invocations marker bit from
	// the VMState.
	cleanSaveInvocationsBitMask = saveInvocationsBit ^ 0xFF
)

// NotificationEvent is a tuple of the scripthash that has emitted the Item as a
// notification and the item itself.
type NotificationEvent struct {
	ScriptHash util.Uint160     `json:"contract"`
	Name       string           `json:"eventname"`
	Item       *stackitem.Array `json:"state"`
}

// AppExecResult represents the result of the script execution, gathering together
// all resulting notifications, state, stack and other metadata.
type AppExecResult struct {
	Container util.Uint256
	Execution
}

// ContainedNotificationEvent represents a wrapper for a notification from script execution.
type ContainedNotificationEvent struct {
	// Container hash is the hash of script container which is either a block or a transaction.
	Container util.Uint256
	NotificationEvent
}

// EncodeBinary implements the Serializable interface.
func (ne *NotificationEvent) EncodeBinary(w *io.BinWriter) {
	ne.EncodeBinaryWithContext(w, stackitem.NewSerializationContext())
}

// EncodeBinaryWithContext is the same as EncodeBinary, but allows to efficiently reuse
// stack item serialization context.
func (ne *NotificationEvent) EncodeBinaryWithContext(w *io.BinWriter, sc *stackitem.SerializationContext) {
	ne.ScriptHash.EncodeBinary(w)
	w.WriteString(ne.Name)
	b, err := sc.Serialize(ne.Item, false)
	if err != nil {
		w.Err = err
		return
	}
	w.WriteBytes(b)
}

// DecodeBinary implements the Serializable interface.
func (ne *NotificationEvent) DecodeBinary(r *io.BinReader) {
	ne.ScriptHash.DecodeBinary(r)
	ne.Name = r.ReadString()
	item := stackitem.DecodeBinary(r)
	if r.Err != nil {
		return
	}
	arr, ok := item.Value().([]stackitem.Item)
	if !ok {
		r.Err = errors.New("Array or Struct expected")
		return
	}
	ne.Item = stackitem.NewArray(arr)
}

// EncodeBinary implements the Serializable interface.
func (aer *AppExecResult) EncodeBinary(w *io.BinWriter) {
	aer.EncodeBinaryWithContext(w, stackitem.NewSerializationContext())
}

// EncodeBinaryWithContext is the same as EncodeBinary, but allows to efficiently reuse
// stack item serialization context.
func (aer *AppExecResult) EncodeBinaryWithContext(w *io.BinWriter, sc *stackitem.SerializationContext) {
	w.WriteBytes(aer.Container[:])
	w.WriteB(byte(aer.Trigger))
	invocLen := len(aer.Invocations)
	if invocLen > 0 {
		aer.VMState |= saveInvocationsBit
	}
	w.WriteB(byte(aer.VMState))
	w.WriteU64LE(uint64(aer.GasConsumed))
	// Stack items are expected to be marshaled one by one.
	w.WriteVarUint(uint64(len(aer.Stack)))
	for _, it := range aer.Stack {
		b, err := sc.Serialize(it, true)
		if err != nil {
			w.Err = err
			return
		}
		w.WriteBytes(b)
	}
	w.WriteVarUint(uint64(len(aer.Events)))
	for i := range aer.Events {
		aer.Events[i].EncodeBinaryWithContext(w, sc)
	}
	w.WriteVarBytes([]byte(aer.FaultException))
	if invocLen > 0 {
		w.WriteVarUint(uint64(invocLen))
		for i := range aer.Invocations {
			aer.Invocations[i].EncodeBinaryWithContext(w, sc)
		}
	}
}

// DecodeBinary implements the Serializable interface.
func (aer *AppExecResult) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(aer.Container[:])
	aer.Trigger = trigger.Type(r.ReadB())
	aer.VMState = vmstate.State(r.ReadB())
	aer.GasConsumed = int64(r.ReadU64LE())
	sz := r.ReadVarUint()
	if stackitem.MaxDeserialized < sz && r.Err == nil {
		r.Err = errors.New("invalid format")
	}
	if r.Err != nil {
		return
	}
	arr := make([]stackitem.Item, sz)
	for i := range arr {
		arr[i] = stackitem.DecodeBinaryProtected(r)
		if r.Err != nil {
			return
		}
	}
	aer.Stack = arr
	r.ReadArray(&aer.Events)
	aer.FaultException = r.ReadString()
	if aer.VMState&saveInvocationsBit != 0 {
		r.ReadArray(&aer.Invocations)
		aer.VMState &= cleanSaveInvocationsBitMask
	}
}

// notificationEventAux is an auxiliary struct for NotificationEvent JSON marshalling.
type notificationEventAux struct {
	ScriptHash util.Uint160    `json:"contract"`
	Name       string          `json:"eventname"`
	Item       json.RawMessage `json:"state"`
}

// MarshalJSON implements the json.Marshaler interface.
func (ne NotificationEvent) MarshalJSON() ([]byte, error) {
	item, err := stackitem.ToJSONWithTypes(ne.Item)
	if err != nil {
		item = []byte(fmt.Sprintf(`"error: %v"`, err))
	}
	return json.Marshal(&notificationEventAux{
		ScriptHash: ne.ScriptHash,
		Name:       ne.Name,
		Item:       item,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (ne *NotificationEvent) UnmarshalJSON(data []byte) error {
	aux := new(notificationEventAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	item, err := stackitem.FromJSONWithTypes(aux.Item)
	if err != nil {
		return err
	}
	if t := item.Type(); t != stackitem.ArrayT {
		return fmt.Errorf("failed to convert notification event state of type %s to array", t.String())
	}
	ne.Item = item.(*stackitem.Array)
	ne.Name = aux.Name
	ne.ScriptHash = aux.ScriptHash
	return nil
}

// appExecResultAux is an auxiliary struct for JSON marshalling.
type appExecResultAux struct {
	Container util.Uint256 `json:"container"`
}

// MarshalJSON implements the json.Marshaler interface.
func (aer *AppExecResult) MarshalJSON() ([]byte, error) {
	h, err := json.Marshal(&appExecResultAux{
		Container: aer.Container,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hash: %w", err)
	}
	exec, err := json.Marshal(aer.Execution)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal execution: %w", err)
	}

	if h[len(h)-1] != '}' || exec[0] != '{' {
		return nil, errors.New("can't merge internal jsons")
	}
	h[len(h)-1] = ','
	h = append(h, exec[1:]...)
	return h, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (aer *AppExecResult) UnmarshalJSON(data []byte) error {
	aux := new(appExecResultAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &aer.Execution); err != nil {
		return err
	}
	aer.Container = aux.Container
	return nil
}

// Execution represents the result of a single script execution, gathering together
// all resulting notifications, state, stack and other metadata.
type Execution struct {
	Trigger        trigger.Type
	VMState        vmstate.State
	GasConsumed    int64
	Stack          []stackitem.Item
	Events         []NotificationEvent
	FaultException string
	Invocations    []ContractInvocation
}

// executionAux represents an auxiliary struct for Execution JSON marshalling.
type executionAux struct {
	Trigger        string               `json:"trigger"`
	VMState        string               `json:"vmstate"`
	GasConsumed    int64                `json:"gasconsumed,string"`
	Stack          json.RawMessage      `json:"stack"`
	Events         []NotificationEvent  `json:"notifications"`
	FaultException *string              `json:"exception"`
	Invocations    []ContractInvocation `json:"invocations"`
}

// MarshalJSON implements the json.Marshaler interface.
func (e Execution) MarshalJSON() ([]byte, error) {
	arr := make([]json.RawMessage, len(e.Stack))
	for i := range arr {
		data, err := stackitem.ToJSONWithTypes(e.Stack[i])
		if err != nil {
			data = []byte(fmt.Sprintf(`"error: %v"`, err))
		}
		arr[i] = data
	}
	st, err := json.Marshal(arr)
	if err != nil {
		return nil, err
	}
	var exception *string
	if e.FaultException != "" {
		exception = &e.FaultException
	}
	return json.Marshal(&executionAux{
		Trigger:        e.Trigger.String(),
		VMState:        e.VMState.String(),
		GasConsumed:    e.GasConsumed,
		Stack:          st,
		Events:         e.Events,
		FaultException: exception,
		Invocations:    e.Invocations,
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (e *Execution) UnmarshalJSON(data []byte) error {
	aux := new(executionAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(aux.Stack, &arr); err == nil {
		st := make([]stackitem.Item, len(arr))
		for i := range arr {
			st[i], err = stackitem.FromJSONWithTypes(arr[i])
			if err != nil {
				var s string
				if json.Unmarshal(arr[i], &s) != nil {
					break
				}
				err = nil
			}
		}
		if err == nil {
			e.Stack = st
		}
	}
	trigger, err := trigger.FromString(aux.Trigger)
	if err != nil {
		return err
	}
	e.Trigger = trigger
	state, err := vmstate.FromString(aux.VMState)
	if err != nil {
		return err
	}
	e.VMState = state
	e.Events = aux.Events
	e.GasConsumed = aux.GasConsumed
	if aux.FaultException != nil {
		e.FaultException = *aux.FaultException
	}
	e.Invocations = aux.Invocations
	return nil
}

// containedNotificationEventAux is an auxiliary struct for JSON marshalling.
type containedNotificationEventAux struct {
	Container util.Uint256 `json:"container"`
}

// MarshalJSON implements the json.Marshaler interface.
func (ne *ContainedNotificationEvent) MarshalJSON() ([]byte, error) {
	h, err := json.Marshal(&containedNotificationEventAux{
		Container: ne.Container,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hash: %w", err)
	}
	exec, err := json.Marshal(ne.NotificationEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal execution: %w", err)
	}

	if h[len(h)-1] != '}' || exec[0] != '{' {
		return nil, errors.New("can't merge internal jsons")
	}
	h[len(h)-1] = ','
	h = append(h, exec[1:]...)
	return h, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (ne *ContainedNotificationEvent) UnmarshalJSON(data []byte) error {
	aux := new(containedNotificationEventAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &ne.NotificationEvent); err != nil {
		return err
	}
	ne.Container = aux.Container
	return nil
}
