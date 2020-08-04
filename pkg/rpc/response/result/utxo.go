package result

import "github.com/nspcc-dev/neo-go/pkg/util"

// UTXO represents single output for a single asset.
type UTXO struct {
	Index     uint32       `json:"block_index"`
	Timestamp uint32       `json:"timestamp"`
	TxHash    util.Uint256 `json:"txid"`
	Address   util.Uint160 `json:"transfer_address"`
	Amount    int64        `json:"amount,string"`
}

// AssetUTXO represents UTXO for a specific asset.
type AssetUTXO struct {
	AssetHash    util.Uint256 `json:"asset_hash"`
	AssetName    string       `json:"asset"`
	TotalAmount  int64        `json:"total_amount,string"`
	Transactions []UTXO       `json:"transactions"`
}

// GetUTXO is a result of the `getutxotransfers` RPC.
type GetUTXO struct {
	Address  string      `json:"address"`
	Sent     []AssetUTXO `json:"sent"`
	Received []AssetUTXO `json:"received"`
}
