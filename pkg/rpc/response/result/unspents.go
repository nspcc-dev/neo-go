package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// UnspentBalanceInfo wrapper is used to represent single unspent asset entry
// in `getunspents` output.
type UnspentBalanceInfo struct {
	Unspents    []state.UnspentBalance `json:"unspent"`
	AssetHash   util.Uint256           `json:"asset_hash"`
	Asset       string                 `json:"asset"`
	AssetSymbol string                 `json:"asset_symbol"`
	Amount      util.Fixed8            `json:"amount"`
}

// Unspents wrapper is used to represent getunspents return result.
type Unspents struct {
	Balance []UnspentBalanceInfo `json:"balance"`
	Address string               `json:"address"`
}

// GlobalAssets stores a map of asset IDs to user-friendly strings ("NEO"/"GAS").
var GlobalAssets = map[string]string{
	"c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b": "NEO",
	"602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7": "GAS",
}

// NewUnspents creates a new Account wrapper using given Blockchainer.
func NewUnspents(a *state.Account, chain blockchainer.Blockchainer, addr string) Unspents {
	res := Unspents{
		Address: addr,
		Balance: make([]UnspentBalanceInfo, 0, len(a.Balances)),
	}
	balanceValues := a.GetBalanceValues()
	for k, v := range a.Balances {
		name, ok := GlobalAssets[k.StringLE()]
		if !ok {
			as := chain.GetAssetState(k)
			if as != nil {
				name = as.Name
			}
		}

		res.Balance = append(res.Balance, UnspentBalanceInfo{
			Unspents:    v,
			AssetHash:   k,
			Asset:       name,
			AssetSymbol: name,
			Amount:      balanceValues[k],
		})
	}

	return res
}
