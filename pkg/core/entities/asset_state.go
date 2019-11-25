package entities

import (
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

const feeMode = 0x0

// AssetState represents the state of an NEO registered Asset.
type AssetState struct {
	ID         util.Uint256
	AssetType  transaction.AssetType
	Name       string
	Amount     util.Fixed8
	Available  util.Fixed8
	Precision  uint8
	FeeMode    uint8
	FeeAddress util.Uint160
	Owner      keys.PublicKey
	Admin      util.Uint160
	Issuer     util.Uint160
	Expiration uint32
	IsFrozen   bool
}

// DecodeBinary implements Serializable interface.
func (a *AssetState) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(a.ID[:])
	br.ReadLE(&a.AssetType)

	a.Name = br.ReadString()

	br.ReadLE(&a.Amount)
	br.ReadLE(&a.Available)
	br.ReadLE(&a.Precision)
	br.ReadLE(&a.FeeMode)
	br.ReadBytes(a.FeeAddress[:])

	a.Owner.DecodeBinary(br)
	br.ReadBytes(a.Admin[:])
	br.ReadBytes(a.Issuer[:])
	br.ReadLE(&a.Expiration)
	br.ReadLE(&a.IsFrozen)
}

// EncodeBinary implements Serializable interface.
func (a *AssetState) EncodeBinary(bw *io.BinWriter) {
	bw.WriteBytes(a.ID[:])
	bw.WriteLE(a.AssetType)
	bw.WriteString(a.Name)
	bw.WriteLE(a.Amount)
	bw.WriteLE(a.Available)
	bw.WriteLE(a.Precision)
	bw.WriteLE(a.FeeMode)
	bw.WriteBytes(a.FeeAddress[:])

	a.Owner.EncodeBinary(bw)

	bw.WriteBytes(a.Admin[:])
	bw.WriteBytes(a.Issuer[:])
	bw.WriteLE(a.Expiration)
	bw.WriteLE(a.IsFrozen)
}

// GetName returns the asset name based on its type.
func (a *AssetState) GetName() string {

	if a.AssetType == transaction.GoverningToken {
		return "NEO"
	} else if a.AssetType == transaction.UtilityToken {
		return "NEOGas"
	}

	return a.Name
}
