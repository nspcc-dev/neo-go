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

// Iterator represents VM iterator identifier. It either has ID set (for those JSON-RPC servers
// that support sessions) or non-nil Values and Truncated set (for those JSON-RPC servers that
// doesn't support sessions but perform in-place iterator traversing) or doesn't have ID, Values
// and Truncated set at all (for those JSON-RPC servers that doesn't support iterator sessions
// and doesn't perform in-place iterator traversing).
type Iterator struct {
	// ID represents iterator ID. It is non-nil iff JSON-RPC server support session mechanism.
	ID *uuid.UUID

	// Values contains deserialized VM iterator values with a truncated flag. It is non-nil
	// iff JSON-RPC server does not support sessions mechanism and able to traverse iterator.
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
	} else {
		value := make([]json.RawMessage, len(r.Values))
		for i := range r.Values {
			var err error
			value[i], err = stackitem.ToJSONWithTypes(r.Values[i])
			if err != nil {
				return nil, err
			}
		}
		iaux.Value = value
		iaux.Truncated = r.Truncated
	}
	return json.Marshal(iaux)
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
				iteratorAux := new(iteratorAux)
				if json.Unmarshal(arr[i], iteratorAux) == nil {
					if len(iteratorAux.Interface) != 0 {
						if iteratorAux.Interface != iteratorInterfaceName {
							err = fmt.Errorf("unknown InteropInterface: %s", iteratorAux.Interface)
							break
						}
						var iID uuid.UUID
						iID, err = uuid.Parse(iteratorAux.ID) // iteratorAux.ID is always non-empty, see https://github.com/neo-project/neo-modules/pull/715#discussion_r897635424.
						if err != nil {
							err = fmt.Errorf("failed to unmarshal iterator ID: %w", err)
							break
						}
						// It's impossible to restore initial iterator type; also iterator is almost
						// useless outside the VM, thus let's replace it with a special structure.
						st[i] = stackitem.NewInterop(Iterator{
							ID: &iID,
						})
					} else {
						iteratorValues := make([]stackitem.Item, len(iteratorAux.Value))
						for j := range iteratorValues {
							iteratorValues[j], err = stackitem.FromJSONWithTypes(iteratorAux.Value[j])
							if err != nil {
								err = fmt.Errorf("failed to unmarshal iterator values: %w", err)
								break
							}
						}
						// It's impossible to restore initial iterator type; also iterator is almost
						// useless outside the VM, thus let's replace it with a special structure.
						st[i] = stackitem.NewInterop(Iterator{
							Values:    iteratorValues,
							Truncated: iteratorAux.Truncated,
						})
					}
				}
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
