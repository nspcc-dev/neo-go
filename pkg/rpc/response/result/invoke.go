package result

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Invoke represents code invocation result and is used by several RPC calls
// that invoke functions, scripts and generic bytecode.
type Invoke struct {
	State          string
	GasConsumed    int64
	Script         []byte
	Stack          []stackitem.Item
	FaultException string
	Transaction    *transaction.Transaction
}

type invokeAux struct {
	State          string          `json:"state"`
	GasConsumed    int64           `json:"gasconsumed,string"`
	Script         []byte          `json:"script"`
	Stack          json.RawMessage `json:"stack"`
	FaultException string          `json:"exception,omitempty"`
	Transaction    []byte          `json:"tx,omitempty"`
}

// MarshalJSON implements json.Marshaler.
func (r Invoke) MarshalJSON() ([]byte, error) {
	var st json.RawMessage
	arr := make([]json.RawMessage, len(r.Stack))
	for i := range arr {
		data, err := stackitem.ToJSONWithTypes(r.Stack[i])
		if err != nil {
			st = []byte(`"error: recursive reference"`)
			break
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
