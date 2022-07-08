package result

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Invoke represents a code invocation result and is used by several RPC calls
// that invoke functions, scripts and generic bytecode.
type Invoke struct {
	State                  string
	GasConsumed            int64
	Script                 []byte
	Stack                  []stackitem.Item
	FaultException         string
	Notifications          []state.NotificationEvent
	Transaction            *transaction.Transaction
	Diagnostics            *InvokeDiag
	maxIteratorResultItems int
	Session                uuid.UUID
	finalize               func()
	registerIterator       RegisterIterator
}

// RegisterIterator is a callback used to register new iterator on the server side.
type RegisterIterator func(sessionID string, item stackitem.Item, id int, finalize func()) (uuid.UUID, error)

// InvokeDiag is an additional diagnostic data for invocation.
type InvokeDiag struct {
	Changes     []storage.Operation  `json:"storagechanges"`
	Invocations []*vm.InvocationTree `json:"invokedcontracts"`
}

// NewInvoke returns a new Invoke structure with the given fields set.
func NewInvoke(ic *interop.Context, script []byte, faultException string, registerIterator RegisterIterator, maxIteratorResultItems int) *Invoke {
	var diag *InvokeDiag
	tree := ic.VM.GetInvocationTree()
	if tree != nil {
		diag = &InvokeDiag{
			Invocations: tree.Calls,
			Changes:     storage.BatchToOperations(ic.DAO.GetBatch()),
		}
	}
	notifications := ic.Notifications
	if notifications == nil {
		notifications = make([]state.NotificationEvent, 0)
	}
	return &Invoke{
		State:                  ic.VM.State().String(),
		GasConsumed:            ic.VM.GasConsumed(),
		Script:                 script,
		Stack:                  ic.VM.Estack().ToArray(),
		FaultException:         faultException,
		Notifications:          notifications,
		Diagnostics:            diag,
		finalize:               ic.Finalize,
		maxIteratorResultItems: maxIteratorResultItems,
		registerIterator:       registerIterator,
	}
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

// Finalize releases resources occupied by Iterators created at the script invocation.
// This method will be called automatically on Invoke marshalling or by the Server's
// sessions handler.
func (r *Invoke) Finalize() {
	if r.finalize != nil {
		r.finalize()
	}
}

// MarshalJSON implements the json.Marshaler.
func (r Invoke) MarshalJSON() ([]byte, error) {
	var (
		st              json.RawMessage
		err             error
		faultSep        string
		arr             = make([]json.RawMessage, len(r.Stack))
		sessionsEnabled = r.registerIterator != nil
		sessionID       string
	)
	if len(r.FaultException) != 0 {
		faultSep = " / "
	}
arrloop:
	for i := range arr {
		var data []byte
		if (r.Stack[i].Type() == stackitem.InteropT) && iterator.IsIterator(r.Stack[i]) {
			if sessionsEnabled {
				if sessionID == "" {
					sessionID = uuid.NewString()
				}
				iteratorID, err := r.registerIterator(sessionID, r.Stack[i], i, r.finalize)
				if err != nil {
					// Call finalizer immediately, there can't be race between server and marshaller because session wasn't added to server's session pool.
					r.Finalize()
					return nil, fmt.Errorf("failed to register iterator session: %w", err)
				}
				data, err = json.Marshal(iteratorAux{
					Type:      stackitem.InteropT.String(),
					Interface: iteratorInterfaceName,
					ID:        iteratorID.String(),
				})
				if err != nil {
					r.FaultException += fmt.Sprintf("%sjson error: failed to marshal iterator: %v", faultSep, err)
					break
				}
			} else {
				iteratorValues, truncated := iterator.ValuesTruncated(r.Stack[i], r.maxIteratorResultItems)
				value := make([]json.RawMessage, len(iteratorValues))
				for j := range iteratorValues {
					value[j], err = stackitem.ToJSONWithTypes(iteratorValues[j])
					if err != nil {
						r.FaultException += fmt.Sprintf("%sjson error: %v", faultSep, err)
						break arrloop
					}
				}
				data, err = json.Marshal(iteratorAux{
					Type:      stackitem.InteropT.String(),
					Value:     value,
					Truncated: truncated,
				})
				if err != nil {
					r.FaultException += fmt.Sprintf("%sjson error: %v", faultSep, err)
					break
				}
			}
		} else {
			data, err = stackitem.ToJSONWithTypes(r.Stack[i])
			if err != nil {
				r.FaultException += fmt.Sprintf("%sjson error: %v", faultSep, err)
				break
			}
		}
		arr[i] = data
	}

	if !sessionsEnabled || sessionID == "" {
		// Call finalizer manually if iterators are disabled or there's no unnested iterators on estack.
		defer r.Finalize()
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
		if err == nil {
			r.Stack = st
		}
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
