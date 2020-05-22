package transaction

import "github.com/nspcc-dev/neo-go/pkg/util"

// Result represents the Result of a transaction.
type Result struct {
	AssetID util.Uint256
	Amount  util.Fixed8
}
