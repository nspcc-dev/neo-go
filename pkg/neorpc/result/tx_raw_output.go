package result

import (
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// TransactionOutputRaw is used as a wrapper to represents
// a Transaction.
type TransactionOutputRaw struct {
	transaction.Transaction
	TransactionMetadata
}

// TransactionMetadata is an auxiliary struct for proper TransactionOutputRaw marshaling.
type TransactionMetadata struct {
	Blockhash     util.Uint256 `json:"blockhash,omitzero"`
	Confirmations int          `json:"confirmations,omitzero"`
	Timestamp     uint64       `json:"blocktime,omitzero"`
	VMState       string       `json:"vmstate,omitzero"`
}

// MarshalJSON implements the json.Marshaler interface.
func (t TransactionOutputRaw) MarshalJSON() ([]byte, error) {
	output, err := json.Marshal(t.TransactionMetadata)
	if err != nil {
		return nil, err
	}
	txBytes, err := json.Marshal(&t.Transaction)
	if err != nil {
		return nil, err
	}

	// We have to keep both transaction.Transaction and tranactionOutputRaw at the same level in json
	// in order to match C# API, so there's no way to marshall Tx correctly with standard json.Marshaller tool.
	if string(output) == "{}" {
		// Empty meta -> return tx data.
		return txBytes, nil
	}
	if output[len(output)-1] != '}' || txBytes[0] != '{' {
		return nil, errors.New("can't merge internal jsons")
	}
	output[len(output)-1] = ','
	output = append(output, txBytes[1:]...)
	return output, nil
}

// UnmarshalJSON implements the json.Marshaler interface.
func (t *TransactionOutputRaw) UnmarshalJSON(data []byte) error {
	// As transaction.Transaction and tranactionOutputRaw are at the same level in json,
	// do unmarshalling separately for both structs.
	output := new(TransactionMetadata)
	err := json.Unmarshal(data, output)
	if err != nil {
		return err
	}
	t.TransactionMetadata = *output
	return json.Unmarshal(data, &t.Transaction)
}
