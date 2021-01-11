package result

import (
	"errors"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Unclaimed wrapper is used to represent getunclaimed return result.
type Unclaimed struct {
	Available   util.Fixed8 `json:"available"`
	Unavailable util.Fixed8 `json:"unavailable"`
	Unclaimed   util.Fixed8 `json:"unclaimed"`
}

// NewUnclaimed creates a new Unclaimed wrapper using given Blockchainer.
func NewUnclaimed(a *state.Account, chain core.Blockchainer) (*Unclaimed, error) {
	var (
		available   util.Fixed8
		unavailable util.Fixed8
	)

	err := a.Unclaimed.ForEach(func(ucb *state.UnclaimedBalance) error {
		gen, sys, err := chain.CalculateClaimable(ucb.Value, ucb.Start, ucb.End)
		if err != nil {
			return err
		}
		available += gen + sys
		return nil
	})
	if err != nil {
		return nil, err
	}

	blockHeight := chain.BlockHeight()
	for _, usb := range a.Balances[core.GoverningTokenID()] {
		_, txHeight, err := chain.GetTransaction(usb.Tx)
		if err != nil {
			return nil, err
		}
		if txHeight == math.MaxUint32 {
			return nil, errors.New("wrong transaction stored in account data")
		}
		gen, sys, err := chain.CalculateClaimable(usb.Value, txHeight, blockHeight)
		if err != nil {
			return nil, err
		}
		unavailable += gen + sys
	}

	return &Unclaimed{
		Available:   available,
		Unavailable: unavailable,
		Unclaimed:   available + unavailable,
	}, nil
}
