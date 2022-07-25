package rpcsrv

import (
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
)

// tokenTransfers is a generic type used to represent NEP-11 and NEP-17 transfers.
type tokenTransfers struct {
	Sent     []interface{} `json:"sent"`
	Received []interface{} `json:"received"`
	Address  string        `json:"address"`
}

// nep17TransferToNEP11 adds an ID to the provided NEP-17 transfer and returns a new
// NEP-11 structure.
func nep17TransferToNEP11(t17 *result.NEP17Transfer, id string) result.NEP11Transfer {
	return result.NEP11Transfer{
		Timestamp:   t17.Timestamp,
		Asset:       t17.Asset,
		Address:     t17.Address,
		ID:          id,
		Amount:      t17.Amount,
		Index:       t17.Index,
		NotifyIndex: t17.NotifyIndex,
		TxHash:      t17.TxHash,
	}
}
