package result

import (
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NEP17Balances is a result for the getnep17balances RPC call.
type NEP17Balances struct {
	Balances []NEP17Balance `json:"balance"`
	Address  string         `json:"address"`
}

// NEP17Balance represents balance for the single token contract.
type NEP17Balance struct {
	Asset       util.Uint160 `json:"assethash"`
	Amount      string       `json:"amount"`
	LastUpdated uint32       `json:"lastupdatedblock"`
}

// NEP17Transfers is a result for the getnep17transfers RPC.
type NEP17Transfers struct {
	Sent     []NEP17Transfer `json:"sent"`
	Received []NEP17Transfer `json:"received"`
	Address  string          `json:"address"`
}

// NEP17Transfer represents single NEP17 transfer event.
type NEP17Transfer struct {
	Timestamp   uint64       `json:"timestamp"`
	Asset       util.Uint160 `json:"assethash"`
	Address     string       `json:"transferaddress,omitempty"`
	Amount      string       `json:"amount"`
	Index       uint32       `json:"blockindex"`
	NotifyIndex uint32       `json:"transfernotifyindex"`
	TxHash      util.Uint256 `json:"txhash"`
}
