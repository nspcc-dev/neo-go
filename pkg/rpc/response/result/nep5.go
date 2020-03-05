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
