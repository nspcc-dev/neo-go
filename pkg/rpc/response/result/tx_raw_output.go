package result

import (
	"encoding/json"
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// TransactionOutputRaw is used as a wrapper to represents
// a Transaction.
type TransactionOutputRaw struct {
	*transaction.Transaction
	TransactionMetadata
}

// TransactionMetadata is an auxilliary struct for proper TransactionOutputRaw marshaling.
type TransactionMetadata struct {
	SysFee        util.Fixed8  `json:"sys_fee"`
	NetFee        util.Fixed8  `json:"net_fee"`
	Blockhash     util.Uint256 `json:"blockhash,omitempty"`
	Confirmations int          `json:"confirmations,omitempty"`
	Timestamp     uint32       `json:"blocktime,omitempty"`
}

// NewTransactionOutputRaw returns a new ransactionOutputRaw object.
func NewTransactionOutputRaw(tx *transaction.Transaction, header *block.Header, chain core.Blockchainer) TransactionOutputRaw {
	result := TransactionOutputRaw{
		Transaction: tx,
		TransactionMetadata: TransactionMetadata{
			SysFee: chain.SystemFee(tx),
			NetFee: chain.NetworkFee(tx),
		},
	}
	if header == nil {
		return result
	}
	// confirmations formula
	confirmations := int(chain.BlockHeight() - header.Base.Index + 1)
	result.TransactionMetadata.Blockhash = header.Hash()
	result.TransactionMetadata.Confirmations = confirmations
	result.TransactionMetadata.Timestamp = header.Timestamp
	return result
}

// MarshalJSON implements json.Marshaler interface.
func (t TransactionOutputRaw) MarshalJSON() ([]byte, error) {
	output, err := json.Marshal(t.TransactionMetadata)
	if err != nil {
		return nil, err
	}
	txBytes, err := json.Marshal(t.Transaction)
	if err != nil {
		return nil, err
	}

	// We have to keep both transaction.Transaction and tranactionOutputRaw at the same level in json
	// in order to match C# API, so there's no way to marshall Tx correctly with standard json.Marshaller tool.
	if output[len(output)-1] != '}' || txBytes[0] != '{' {
		return nil, errors.New("can't merge internal jsons")
	}
	output[len(output)-1] = ','
	output = append(output, txBytes[1:]...)
	return output, nil
}

// UnmarshalJSON implements json.Marshaler interface.
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
