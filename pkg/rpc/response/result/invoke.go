package result

import (
	"encoding/json"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Invoke represents code invocation result and is used by several RPC calls
// that invoke functions, scripts and generic bytecode.
type Invoke struct {
	State                  string
	GasConsumed            int64
	Script                 []byte
	Stack                  []stackitem.Item
	FaultException         string
	Transaction            *transaction.Transaction
	maxIteratorResultItems int
}

// NewInvoke returns new Invoke structure with the given fields set.
func NewInvoke(vm *vm.VM, script []byte, faultException string, maxIteratorResultItems int) *Invoke {
	return &Invoke{
		State:                  vm.State().String(),
		GasConsumed:            vm.GasConsumed(),
		Script:                 script,
		Stack:                  vm.Estack().ToArray(),
		FaultException:         faultException,
		maxIteratorResultItems: maxIteratorResultItems,
	}
}

type invokeAux struct {
	State          string          `json:"state"`
	GasConsumed    int64           `json:"gasconsumed,string"`
	Script         []byte          `json:"script"`
	Stack          json.RawMessage `json:"stack"`
	FaultException string          `json:"exception,omitempty"`
	Transaction    []byte          `json:"tx,omitempty"`
}

type iteratorAux struct {
	Type      string            `json:"type"`
	Value     []json.RawMessage `json:"iterator"`
	Truncated bool              `json:"truncated"`
}

// Iterator represents deserialized VM iterator values with truncated flag.
type Iterator struct {
	Values    []stackitem.Item
	Truncated bool
}

// MarshalJSON implements json.Marshaler.
func (r Invoke) MarshalJSON() ([]byte, error) {
	var st json.RawMessage
	arr := make([]json.RawMessage, len(r.Stack))
	for i := range arr {
		var (
			data []byte
			err  error
		)
		if (r.Stack[i].Type() == stackitem.InteropT) && vm.IsIterator(r.Stack[i]) {
			iteratorValues, truncated := vm.IteratorValues(r.Stack[i], r.maxIteratorResultItems)
			value := make([]json.RawMessage, len(iteratorValues))
			for j := range iteratorValues {
				value[j], err = stackitem.ToJSONWithTypes(iteratorValues[j])
				if err != nil {
					st = []byte(`"error: recursive reference"`)
					break
				}
			}
			data, err = json.Marshal(iteratorAux{
				Type:      stackitem.InteropT.String(),
				Value:     value,
				Truncated: truncated,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to marshal iterator: %w", err)
			}
		} else {
			data, err = stackitem.ToJSONWithTypes(r.Stack[i])
			if err != nil {
				st = []byte(`"error: recursive reference"`)
				break
			}
		}
		arr[i] = data
	}

	var err error
	if st == nil {
		st, err = json.Marshal(arr)
		if err != nil {
			return nil, err
		}
	}
	var txbytes []byte
	if r.Transaction != nil {
		txbytes = r.Transaction.Bytes()
	}
	return json.Marshal(&invokeAux{
		GasConsumed:    r.GasConsumed,
		Script:         r.Script,
		State:          r.State,
		Stack:          st,
		FaultException: r.FaultException,
		Transaction:    txbytes,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (r *Invoke) UnmarshalJSON(data []byte) error {
	var err error
	aux := new(invokeAux)
	if err = json.Unmarshal(data, aux); err != nil {
		return err
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
					iteratorValues := make([]stackitem.Item, len(iteratorAux.Value))
					for j := range iteratorValues {
						iteratorValues[j], err = stackitem.FromJSONWithTypes(iteratorAux.Value[j])
						if err != nil {
							err = fmt.Errorf("failed to unmarshal iterator values: %w", err)
							break
						}
					}

					// it's impossible to restore initial iterator type; also iterator is almost
					// useless outside of the VM, thus let's replace it with a special structure.
					st[i] = stackitem.NewInterop(Iterator{
						Values:    iteratorValues,
						Truncated: iteratorAux.Truncated,
					})
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
	r.FaultException = aux.FaultException
	r.Transaction = tx
	return nil
}
