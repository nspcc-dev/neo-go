package network

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NotaryFeer implements mempool.Feer interface for Notary balance handling.
type NotaryFeer struct {
	bc Ledger
}

// FeePerByte implements mempool.Feer interface.
func (f NotaryFeer) FeePerByte() int64 {
	return f.bc.FeePerByte()
}

// GetUtilityTokenBalance implements mempool.Feer interface.
func (f NotaryFeer) GetUtilityTokenBalance(acc util.Uint160) *big.Int {
	return f.bc.GetNotaryBalance(acc)
}

// BlockHeight implements mempool.Feer interface.
func (f NotaryFeer) BlockHeight() uint32 {
	return f.bc.BlockHeight()
}

// NewNotaryFeer returns new NotaryFeer instance.
func NewNotaryFeer(bc Ledger) NotaryFeer {
	return NotaryFeer{
		bc: bc,
	}
}
