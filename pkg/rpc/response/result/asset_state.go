package result

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// AssetState wrapper used for the representation of
// state.Asset on the RPC Server.
type AssetState struct {
	ID         util.Uint256          `json:"id"`
	AssetType  transaction.AssetType `json:"type"`
	Name       string                `json:"name"`
	Amount     util.Fixed8           `json:"amount"`
	Available  util.Fixed8           `json:"available"`
	Precision  uint8                 `json:"precision"`
	Owner      string                `json:"owner"`
	Admin      string                `json:"admin"`
	Issuer     string                `json:"issuer"`
	Expiration uint32                `json:"expiration"`
	IsFrozen   bool                  `json:"frozen"`
}

// NewAssetState creates a new Asset wrapper.
func NewAssetState(a *state.Asset) AssetState {
	return AssetState{
		ID:         a.ID,
		AssetType:  a.AssetType,
		Name:       a.GetName(),
		Amount:     a.Amount,
		Available:  a.Available,
		Precision:  a.Precision,
		Owner:      a.Owner.String(),
		Admin:      address.Uint160ToString(a.Admin),
		Issuer:     address.Uint160ToString(a.Issuer),
		Expiration: a.Expiration,
		IsFrozen:   a.IsFrozen,
	}
}
