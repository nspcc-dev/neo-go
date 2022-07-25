package result

import "github.com/nspcc-dev/neo-go/pkg/util"

// RelayResult ia a result of `sendrawtransaction` or `submitblock` RPC calls.
type RelayResult struct {
	Hash util.Uint256 `json:"hash"`
}
