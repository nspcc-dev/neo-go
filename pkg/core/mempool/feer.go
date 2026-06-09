package mempool

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Feer is an interface that abstracts the implementation of the fee calculation.
type Feer interface {
	FeePerByte() int64
	// GetUtilityTokenBalance returns the balance of the utility token. For the
	// sponsored transactions it's expected to return the amount of GAS deposited
	// to the native Notary contract by the secondary account.
	GetUtilityTokenBalance(primary, secondary util.Uint160) *big.Int
	BlockHeight() uint32
}
