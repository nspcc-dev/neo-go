package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// TransactionOutputRaw is used as a wrapper to represents
// a Transaction.
type TransactionOutputRaw struct {
	*transaction.Transaction
	TxHash        util.Uint256 `json:"txid"`
	Size          int          `json:"size"`
	SysFee        util.Fixed8  `json:"sys_fee"`
	NetFee        util.Fixed8  `json:"net_fee"`
	Blockhash     util.Uint256 `json:"blockhash,omitempty"`
	Confirmations int          `json:"confirmations,omitempty"`
	Timestamp     uint32       `json:"blocktime,omitempty"`
}

// NewTransactionOutputRaw returns a new ransactionOutputRaw object.
func NewTransactionOutputRaw(tx *transaction.Transaction, header *block.Header, chain core.Blockchainer) TransactionOutputRaw {
	// confirmations formula
	confirmations := int(chain.BlockHeight() - header.Base.Index + 1)
	// set index position
	for i, o := range tx.Outputs {
		o.Position = i
	}
	return TransactionOutputRaw{
		Transaction:   tx,
		TxHash:        tx.Hash(),
		Size:          io.GetVarSize(tx),
		SysFee:        chain.SystemFee(tx),
		NetFee:        chain.NetworkFee(tx),
		Blockhash:     header.Hash(),
		Confirmations: confirmations,
		Timestamp:     header.Timestamp,
	}
}
