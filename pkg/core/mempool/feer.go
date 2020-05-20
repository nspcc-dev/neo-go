package mempool

import (
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Feer is an interface that abstract the implementation of the fee calculation.
type Feer interface {
	IsLowPriority(util.Fixed8) bool
	FeePerByte() util.Fixed8
	GetUtilityTokenBalance(util.Uint160) util.Fixed8
}
