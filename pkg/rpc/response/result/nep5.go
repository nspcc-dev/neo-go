package result

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NEP5Balances is a result for the getnep5balances RPC call.
type NEP5Balances struct {
	Balances []NEP5Balance `json:"balance"`
	Address  string        `json:"address"`
}

// NEP5Balance represents balance for the single token contract.
type NEP5Balance struct {
	Asset       util.Uint160 `json:"asset_hash"`
	Amount      string       `json:"amount"`
	LastUpdated uint32       `json:"last_updated_block"`
}

// nep5Balance is an auxilliary struct for proper Asset marshaling.
type nep5Balance struct {
	Asset       string `json:"asset_hash"`
	Amount      string `json:"amount"`
	LastUpdated uint32 `json:"last_updated_block"`
}

// NEP5Transfers is a result for the getnep5transfers RPC.
type NEP5Transfers struct {
	Sent     []NEP5Transfer `json:"sent"`
	Received []NEP5Transfer `json:"received"`
	Address  string         `json:"address"`
}

// NEP5Transfer represents single NEP5 transfer event.
type NEP5Transfer struct {
	Timestamp   uint64       `json:"timestamp"`
	Asset       util.Uint160 `json:"asset_hash"`
	Address     string       `json:"transfer_address,omitempty"`
	Amount      string       `json:"amount"`
	Index       uint32       `json:"block_index"`
	NotifyIndex uint32       `json:"transfer_notify_index"`
	TxHash      util.Uint256 `json:"tx_hash"`
}

// MarshalJSON implements json.Marshaler interface.
func (b *NEP5Balance) MarshalJSON() ([]byte, error) {
	s := &nep5Balance{
		Asset:       b.Asset.StringLE(),
		Amount:      b.Amount,
		LastUpdated: b.LastUpdated,
	}
	return json.Marshal(s)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (b *NEP5Balance) UnmarshalJSON(data []byte) error {
	s := new(nep5Balance)
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	asset, err := util.Uint160DecodeStringLE(s.Asset)
	if err != nil {
		return err
	}
	b.Amount = s.Amount
	b.Asset = asset
	b.LastUpdated = s.LastUpdated
	return nil
}
