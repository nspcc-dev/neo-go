package network

import (
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NotaryFeer implements mempool.Feer interface for Notary balance handling.
type NotaryFeer struct {
	bc blockchainer.Blockchainer
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

// P2PSigExtensionsEnabled implements mempool.Feer interface.
func (f NotaryFeer) P2PSigExtensionsEnabled() bool {
	return f.bc.P2PSigExtensionsEnabled()
}

// P2PNotaryModuleEnabled implements mempool.Feer interface.
func (f NotaryFeer) P2PNotaryModuleEnabled() bool {
	return f.bc.P2PNotaryModuleEnabled()
}

// NewNotaryFeer returns new NotaryFeer instance.
func NewNotaryFeer(bc blockchainer.Blockchainer) NotaryFeer {
	return NotaryFeer{
		bc: bc,
	}
}
