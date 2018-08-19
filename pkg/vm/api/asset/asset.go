package asset

import "github.com/CityOfZion/neo-go/pkg/core"

// GetAssetID returns the id of the given asset.
func GetAssetID(asset *core.AssetState) []byte { return nil }

// TODO: Verify if we need to return a uint8 here.
// GetAssetType returns the type of the given asset.
func GetAssetType(asset *core.AssetState) uint8 { return 0x00 }

// GetAmount returns the amount of the given asset.
func GetAmount(asset *core.AssetState) uint64 { return 0 }

// GetAvailable returns the available amount of the given asset.
func GetAvailable(asset *core.AssetState) uint64 { return 0 }

// GetPrecision returns the precision the given asset.
func GetPrecision(asset *core.AssetState) uint8 { return 0 }

// GetOwner returns the owner the given asset.
func GetOwner(asset *core.AssetState) []byte { return nil }

// GetIssuer returns the issuer the given asset.
func GetIssuer(asset *core.AssetState) []byte { return nil }

// Create a new asset specified by the given parameters.
func Create(typ uint8, name string, amount uint64, owner, admin, issuer []byte) {}

// Renew the given asset for the given x years.
func Renew(asset *core.AssetState, years uint32) {}
