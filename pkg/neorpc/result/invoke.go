package result

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dboper"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/vm/invocations"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Invoke represents a code invocation result and is used by several RPC calls
// that invoke functions, scripts and generic bytecode.
type Invoke struct {
	State          string
	GasConsumed    int64
	Script         []byte
	Stack          []stackitem.Item
	FaultException string
	Notifications  []state.NotificationEvent
	Transaction    *transaction.Transaction
	Diagnostics    *InvokeDiag
	Session        uuid.UUID
}

// InvokeDiag is an additional diagnostic data for invocation.
type InvokeDiag struct {
	Changes     []dboper.Operation  `json:"storagechanges"`
	Invocations []*invocations.Tree `json:"invokedcontracts"`
}

type invokeAux struct {
	State          string                    `json:"state"`
	GasConsumed    int64                     `json:"gasconsumed,string"`
	Script         []byte                    `json:"script"`
	Stack          json.RawMessage           `json:"stack"`
	FaultException *string                   `json:"exception"`
	Notifications  []state.NotificationEvent `json:"notifications"`
	Transaction    []byte                    `json:"tx,omitempty"`
	Diagnostics    *InvokeDiag               `json:"diagnostics,omitempty"`
	Session        string                    `json:"session,omitempty"`
}

// iteratorInterfaceName is a string used to mark Iterator inside the InteropInterface.
const iteratorInterfaceName = "IIterator"

type iteratorAux struct {
	Type      string            `json:"type"`
	Interface string            `json:"interface,omitempty"`
	ID        string            `json:"id,omitempty"`
	Value     []json.RawMessage `json:"iterator,omitempty"`
	Truncated bool              `json:"truncated,omitempty"`
}

// Iterator represents a VM iterator identifier. It can be in one of three
// states: 1. If the JSON-RPC server supports session-based iterators, the
// ID field is set, and the Values field may contain a partial expansion of
// the iterator if session expansion is enabled. 2. If the JSON-RPC server
// does not support sessions but allows in-place iteration, the Values field
// will be populated, and Truncated will indicate whether more items exist.
// 3. If the JSON-RPC server neither supports sessions nor in-place iteration,
// all fields will be unset.
type Iterator struct {
	// ID represents iterator ID. It is non-nil iff JSON-RPC server support session mechanism.
	ID *uuid.UUID

	// Values contains deserialized VM iterator values with a truncated flag. It may be non-nil
	// if session expansion is enabled (even when ID is set), or if the JSON-RPC server does not
	// support sessions but allows in-place iterator traversal.
	Values    []stackitem.Item
	Truncated bool
}

// MarshalJSON implements the json.Marshaler.
func (r Iterator) MarshalJSON() ([]byte, error) {
	var iaux iteratorAux
	iaux.Type = stackitem.InteropT.String()
	if r.ID != nil {
		iaux.Interface = iteratorInterfaceName
		iaux.ID = r.ID.String()
	}
	if r.Values != nil {
		value := make([]json.RawMessage, len(r.Values))
		for i := range r.Values {
			var err error
			value[i], err = stackitem.ToJSONWithTypes(r.Values[i])
			if err != nil {
				return nil, err
			}
		}
		iaux.Value = value
	}
	iaux.Truncated = r.Truncated
	return json.Marshal(iaux)
}

// UnmarshalJSON implements the json.Unmarshaler.
func (r *Iterator) UnmarshalJSON(data []byte) error {
	iteratorAux := new(iteratorAux)
	err := json.Unmarshal(data, iteratorAux)
	if err != nil {
		return err
	}
	if len(iteratorAux.Interface) != 0 {
		if iteratorAux.Interface != iteratorInterfaceName {
			return fmt.Errorf("unknown InteropInterface: %s", iteratorAux.Interface)
		}
		var iID uuid.UUID
		iID, err = uuid.Parse(iteratorAux.ID)
		if err != nil {
			return fmt.Errorf("failed to unmarshal iterator ID: %w", err)
		}
		r.ID = &iID
	}
	if iteratorAux.Value != nil {
		r.Values = make([]stackitem.Item, len(iteratorAux.Value))
		for j := range r.Values {
			r.Values[j], err = stackitem.FromJSONWithTypes(iteratorAux.Value[j])
			if err != nil {
				return fmt.Errorf("failed to unmarshal iterator values: %w", err)
			}
		}
	}
	r.Truncated = iteratorAux.Truncated
	return nil
}

// MarshalJSON implements the json.Marshaler.
func (r Invoke) MarshalJSON() ([]byte, error) {
	var (
		st       json.RawMessage
		err      error
		faultSep string
		arr      = make([]json.RawMessage, len(r.Stack))
	)
	if len(r.FaultException) != 0 {
		faultSep = " / "
	}
	for i := range arr {
		var data []byte

		iter, ok := r.Stack[i].Value().(Iterator)
		if (r.Stack[i].Type() == stackitem.InteropT) && ok {
			data, err = json.Marshal(iter)
		} else {
			data, err = stackitem.ToJSONWithTypes(r.Stack[i])
		}
		if err != nil {
			r.FaultException += fmt.Sprintf("%sjson error: %v", faultSep, err)
			break
		}
		arr[i] = data
	}

	if err == nil {
		st, err = json.Marshal(arr)
		if err != nil {
			return nil, err
		}
	}
	var txbytes []byte
	if r.Transaction != nil {
		txbytes = r.Transaction.Bytes()
	}
	var sessionID string
	if r.Session != (uuid.UUID{}) {
		sessionID = r.Session.String()
	}
	aux := &invokeAux{
		GasConsumed:   r.GasConsumed,
		Script:        r.Script,
		State:         r.State,
		Stack:         st,
		Notifications: r.Notifications,
		Transaction:   txbytes,
		Diagnostics:   r.Diagnostics,
		Session:       sessionID,
	}
	if len(r.FaultException) != 0 {
		aux.FaultException = &r.FaultException
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements the json.Unmarshaler.
func (r *Invoke) UnmarshalJSON(data []byte) error {
	var err error
	aux := new(invokeAux)
	if err = json.Unmarshal(data, aux); err != nil {
		return err
	}
	if len(aux.Session) != 0 {
		r.Session, err = uuid.Parse(aux.Session)
		if err != nil {
			return fmt.Errorf("failed to parse session ID: %w", err)
		}
	}
	var arr []json.RawMessage
	if err = json.Unmarshal(aux.Stack, &arr); err == nil {
		st := make([]stackitem.Item, len(arr))
		for i := range arr {
			st[i], err = stackitem.FromJSONWithTypes(arr[i])
			if err != nil {
				break
			}
			if st[i].Type() == stackitem.InteropT {
				var iter = Iterator{}
				err = json.Unmarshal(arr[i], &iter)
				if err != nil {
					break
				}
				st[i] = stackitem.NewInterop(iter)
			}
		}
		if err != nil {
			return fmt.Errorf("failed to unmarshal stack: %w", err)
		}
		r.Stack = st
	}
	var tx *transaction.Transaction
	if len(aux.Transaction) != 0 {
		tx, err = transaction.NewTransactionFromBytes(aux.Transaction)
		if err != nil {
			return err
		}
	}
	r.GasConsumed = aux.GasConsumed
	r.Script = aux.Script
	r.State = aux.State
	if aux.FaultException != nil {
		r.FaultException = *aux.FaultException
	}
	r.Notifications = aux.Notifications
	r.Transaction = tx
	r.Diagnostics = aux.Diagnostics
	return nil
}

// AppExecToInvocation converts state.AppExecResult to result.Invoke and can be used
// as a wrapper for actor.Wait. The result of AppExecToInvocation doesn't have all fields
// properly filled, it's limited by State, GasConsumed, Stack, FaultException and Notifications.
// The result of AppExecToInvocation can be passed to unwrap package helpers.
func AppExecToInvocation(aer *state.AppExecResult, err error) (*Invoke, error) {
	if err != nil {
		return nil, err
	}
	return &Invoke{
		State:          aer.VMState.String(),
		GasConsumed:    aer.GasConsumed,
		Stack:          aer.Stack,
		FaultException: aer.FaultException,
		Notifications:  aer.Events,
	}, nil
}
