package result

import (
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NEP11Balances is a result for the getnep11balances RPC call.
type NEP11Balances struct {
	Balances []NEP11AssetBalance `json:"balance"`
	Address  string              `json:"address"`
}

// NEP11Balance is a structure holding balance of a NEP-11 asset.
type NEP11AssetBalance struct {
	Asset  util.Uint160        `json:"assethash"`
	Tokens []NEP11TokenBalance `json:"tokens"`
}

// NEP11TokenBalance represents balance of a single NFT.
type NEP11TokenBalance struct {
	ID          string `json:"tokenid"`
	Amount      string `json:"amount"`
	LastUpdated uint32 `json:"lastupdatedblock"`
}

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

// NEP11Transfers is a result for the getnep11transfers RPC.
type NEP11Transfers struct {
	Sent     []NEP11Transfer `json:"sent"`
	Received []NEP11Transfer `json:"received"`
	Address  string          `json:"address"`
}

// NEP11Transfer represents single NEP-11 transfer event.
type NEP11Transfer struct {
	Timestamp   uint64       `json:"timestamp"`
	Asset       util.Uint160 `json:"assethash"`
	Address     string       `json:"transferaddress,omitempty"`
	ID          string       `json:"tokenid"`
	Amount      string       `json:"amount"`
	Index       uint32       `json:"blockindex"`
	NotifyIndex uint32       `json:"transfernotifyindex"`
	TxHash      util.Uint256 `json:"txhash"`
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

// KnownNEP11Properties contains a list of well-known NEP-11 token property names.
var KnownNEP11Properties = map[string]bool{
	"description": true,
	"image":       true,
	"name":        true,
	"tokenURI":    true,
}
