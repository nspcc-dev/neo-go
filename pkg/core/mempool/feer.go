package mempool

import (
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Feer is an interface that abstract the implementation of the fee calculation.
type Feer interface {
	BlockHeight() uint32
	NetworkFee(t *transaction.Transaction) util.Fixed8
	IsLowPriority(util.Fixed8) bool
	FeePerByte(t *transaction.Transaction) util.Fixed8
	SystemFee(t *transaction.Transaction) util.Fixed8
}
