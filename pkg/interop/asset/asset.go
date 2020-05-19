/*
Package asset provides functions to work with regular UTXO assets (like NEO or GAS).
Mostly these are getters for Asset structure, but you can also create new assets
and renew them (although it's recommended to use NEP-5 standard for new tokens).
*/
package asset

// Asset represents NEO asset type that is used in interop functions, it's
// an opaque data structure that you can get data from only using functions from
// this package. It's similar in function to the Asset class in the Neo .net
// framework. To be able to use it you either need to get an existing Asset via
// blockchain.GetAsset function or create a new one via Create.
type Asset struct{}

// GetAssetID returns ID (256-bit ID of Register transaction for this asset in BE
// representation) of the given asset. It uses `Neo.Asset.GetAssetId` syscall
// internally.
func GetAssetID(a Asset) []byte {
	return nil
}

// GetAssetType returns type of the given asset as a byte value. The value
// returned can be interpreted as a bit field with the following meaning:
//     CreditFlag = 0x40
//     DutyFlag = 0x80
//     SystemShare = 0x00
//     SystemCoin = 0x01
//     Currency = 0x08
//     Share = DutyFlag | 0x10
//     Invoice = DutyFlag | 0x18
//     Token = CreditFlag | 0x20
// It uses `Neo.Asset.GetAssetType` syscall internally.
func GetAssetType(a Asset) byte {
	return 0x00
}

// GetAmount returns the total amount of the given asset as an integer
// multiplied by 10⁸. This value is the maximum possible circulating quantity of
// Asset. The function uses `Neo.Asset.GetAmount` syscall internally.
func GetAmount(a Asset) int {
	return 0
}

// GetAvailable returns the amount of Asset currently available on the
// blockchain. It uses the same encoding as the result of GetAmount and its
// value can never exceed the value returned by GetAmount. This function uses
// `Neo.Asset.GetAvailable` syscall internally.
func GetAvailable(a Asset) int {
	return 0
}

// GetPrecision returns precision of the given Asset. It uses
// `Neo.Asset.GetPrecision` syscall internally.
func GetPrecision(a Asset) byte {
	return 0x00
}

// GetOwner returns the owner of the given Asset. It's represented as a
// serialized (in compressed form) public key (33 bytes long). This function
// uses `Neo.Asset.GetOwner` syscall internally.
func GetOwner(a Asset) []byte {
	return nil
}

// GetAdmin returns the admin of the given Asset represented as a 160 bit hash
// in BE form (contract script hash). Admin can modify attributes of this Asset.
// This function uses `Neo.Asset.GetAdmin` syscall internally.
func GetAdmin(a Asset) []byte {
	return nil
}

// GetIssuer returns the issuer of the given Asset represented as a 160 bit hash
// in BE form (contract script hash). Issuer can issue new tokens for this Asset.
// This function uses `Neo.Asset.GetIssuer` syscall internally.
func GetIssuer(a Asset) []byte {
	return nil
}

// Create registers a new asset on the blockchain (similar to old Register
// transaction). `assetType` parameter has the same set of possible values as
// GetAssetType result, `amount` must be multiplied by 10⁸, `precision` limits
// the smallest possible amount of new Asset to 10⁻ⁿ (where n is precision which
// can't exceed 8), `owner` is a public key of the owner in compressed serialized
// form (33 bytes), `admin` and `issuer` should be represented as 20-byte slices
// storing 160-bit hash in BE form. Created Asset is set to expire in one year,
// so you need to renew it in time. If successful, this function returns a new
// Asset. It uses `Neo.Asset.Create` syscall internally.
func Create(assetType byte, name string, amount int, precision byte, owner, admin, issuer []byte) Asset {
	return Asset{}
}

// Renew renews (make available for use) existing asset by the specified number
// of years. It returns the last block number when this asset will be active.
// It uses `Neo.Asset.Renew` syscall internally.
func Renew(asset Asset, years int) int {
	return 0
}
