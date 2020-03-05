package result

import "github.com/nspcc-dev/neo-go/pkg/util"

// NEP5Balances is a result for the getnep5balances RPC call.
type NEP5Balances struct {
	Balances []NEP5Balance `json:"balances"`
	Address  string        `json:"address"`
}

// NEP5Balance represents balance for the single token contract.
type NEP5Balance struct {
	Asset       util.Uint160 `json:"asset_hash"`
	Amount      string       `json:"amount"`
	LastUpdated uint32       `json:"last_updated_block"`
}

// NEP5Transfers is a result for the getnep5transfers RPC.
type NEP5Transfers struct {
	Sent     []NEP5Transfer `json:"sent"`
	Received []NEP5Transfer `json:"received"`
	Address  string         `json:"address"`
}

// NEP5Transfer represents single NEP5 transfer event.
type NEP5Transfer struct {
	Timestamp   uint32       `json:"timestamp"`
	Asset       util.Uint160 `json:"asset_hash"`
	Address     string       `json:"transfer_address,omitempty"`
	Amount      string       `json:"amount"`
	Index       uint32       `json:"block_index"`
	NotifyIndex uint32       `json:"transfer_notify_index"`
	TxHash      util.Uint256 `json:"tx_hash"`
}
