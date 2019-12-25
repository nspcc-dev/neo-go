package wrappers

import (
	"github.com/CityOfZion/neo-go/pkg/core/state"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/encoding/address"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// AssetState wrapper used for the representation of
// state.Asset on the RPC Server.
type AssetState struct {
	ID         util.Uint256          `json:"assetID"`
	AssetType  transaction.AssetType `json:"assetType"`
	Name       string                `json:"name"`
	Amount     util.Fixed8           `json:"amount"`
	Available  util.Fixed8           `json:"available"`
	Precision  uint8                 `json:"precision"`
	FeeMode    uint8                 `json:"fee"`
	FeeAddress util.Uint160          `json:"address"`
	Owner      string                `json:"owner"`
	Admin      string                `json:"admin"`
	Issuer     string                `json:"issuer"`
	Expiration uint32                `json:"expiration"`
	IsFrozen   bool                  `json:"is_frozen"`
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
		FeeMode:    a.FeeMode,
		FeeAddress: a.FeeAddress,
		Owner:      a.Owner.String(),
		Admin:      address.EncodeUint160(a.Admin),
		Issuer:     address.EncodeUint160(a.Issuer),
		Expiration: a.Expiration,
		IsFrozen:   a.IsFrozen,
	}
}
