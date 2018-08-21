package asset

// Package asset provides function signatures that can be used inside
// smart contracts that are written in the neo-go-sc framework.

// Asset stubs a NEO asset type.
type Asset struct{}

// GetAssetID returns the id of the given asset.
func GetAssetID(a Asset) []byte {
	return nil
}

// GetAssetType returns the type of the given asset.
func GetAssetType(a Asset) byte {
	return 0x00
}

// GetAmount returns the amount of the given asset.
func GetAmount(a Asset) int {
	return 0
}

// GetAvailable returns the available of the given asset.
func GetAvailable(a Asset) int {
	return 0
}

// GetPrecision returns the precision of the given asset.
func GetPrecision(a Asset) byte {
	return 0x00
}

// GetOwner returns the owner of the given asset.
func GetOwner(a Asset) []byte {
	return nil
}

// GetAdmin returns the admin of the given asset.
func GetAdmin(a Asset) []byte {
	return nil
}

// GetIssuer returns the issuer of the given asset.
func GetIssuer(a Asset) []byte {
	return nil
}

// Create registers a new asset on the blockchain.
func Create(assetType byte, name string, amount int, precision byte, owner, admin, issuer []byte) {}

// Renew renews the existance of an asset by the given years.
func Renew(asset Asset, years int) {}
