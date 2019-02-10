package wrappers

import (
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// TransactionOutputRaw is used as a wrapper to represents
// a Transaction.
type TransactionOutputRaw struct {
	*transaction.Transaction
	TxHash        util.Uint256 `json:"txid"`
	Size          int          `json:"size"`
	SysFee        util.Fixed8  `json:"sys_fee"`
	NetFee        util.Fixed8  `json:"net_fee"`
	Blockhash     util.Uint256 `json:"blockhash"`
	Confirmations int          `json:"confirmations"`
	Timestamp     uint32       `json:"blocktime"`
}

func NewTransactionOutputRaw(tx *transaction.Transaction, header *core.Header, chain core.Blockchainer) TransactionOutputRaw {

	confirmations := int(chain.BlockHeight() - header.BlockBase.Index + 1)

	for i, o := range tx.Outputs {
		o.Position = i
	}

	return TransactionOutputRaw{
		tx,
		tx.Hash(),
		tx.Size(),
		core.SystemFee(tx),
		core.NetworkFee(tx),
		header.Hash(),
		confirmations,
		header.Timestamp,
	}
}
