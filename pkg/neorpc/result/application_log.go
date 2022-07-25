package result

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// ApplicationLog represent the results of the script executions for a block or a transaction.
type ApplicationLog struct {
	Container     util.Uint256
	IsTransaction bool
	Executions    []state.Execution
}

// applicationLogAux is an auxiliary struct for ApplicationLog JSON marshalling.
type applicationLogAux struct {
	TxHash     *util.Uint256     `json:"txid,omitempty"`
	BlockHash  *util.Uint256     `json:"blockhash,omitempty"`
	Executions []json.RawMessage `json:"executions"`
}

// MarshalJSON implements the json.Marshaler interface.
func (l ApplicationLog) MarshalJSON() ([]byte, error) {
	result := &applicationLogAux{
		Executions: make([]json.RawMessage, len(l.Executions)),
	}
	if l.IsTransaction {
		result.TxHash = &l.Container
	} else {
		result.BlockHash = &l.Container
	}
	var err error
	for i := range result.Executions {
		result.Executions[i], err = json.Marshal(l.Executions[i])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal execution #%d: %w", i, err)
		}
	}
	return json.Marshal(result)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (l *ApplicationLog) UnmarshalJSON(data []byte) error {
	aux := new(applicationLogAux)
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.TxHash != nil {
		l.Container = *aux.TxHash
	} else if aux.BlockHash != nil {
		l.Container = *aux.BlockHash
	} else {
		return errors.New("no block or transaction hash")
	}
	l.Executions = make([]state.Execution, len(aux.Executions))
	for i := range l.Executions {
		err := json.Unmarshal(aux.Executions[i], &l.Executions[i])
		if err != nil {
			return fmt.Errorf("failed to unmarshal execution #%d: %w", i, err)
		}
	}

	return nil
}

// NewApplicationLog creates an ApplicationLog from a set of several application execution results
// including only the results with the specified trigger.
func NewApplicationLog(hash util.Uint256, aers []state.AppExecResult, trig trigger.Type) ApplicationLog {
	result := ApplicationLog{
		Container:     hash,
		IsTransaction: aers[0].Trigger == trigger.Application,
	}
	for _, aer := range aers {
		if aer.Trigger&trig != 0 {
			result.Executions = append(result.Executions, aer.Execution)
		}
	}
	return result
}
