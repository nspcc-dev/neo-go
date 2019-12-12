package state

import (
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

const feeMode = 0x0

// Asset represents the state of an NEO registered Asset.
type Asset struct {
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
func (a *Asset) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(a.ID[:])
	a.AssetType = transaction.AssetType(br.ReadB())

	a.Name = br.ReadString()

	a.Amount.DecodeBinary(br)
	a.Available.DecodeBinary(br)
	a.Precision = uint8(br.ReadB())
	a.FeeMode = uint8(br.ReadB())
	br.ReadBytes(a.FeeAddress[:])

	a.Owner.DecodeBinary(br)
	br.ReadBytes(a.Admin[:])
	br.ReadBytes(a.Issuer[:])
	a.Expiration = br.ReadU32LE()
	a.IsFrozen = br.ReadBool()
}

// EncodeBinary implements Serializable interface.
func (a *Asset) EncodeBinary(bw *io.BinWriter) {
	bw.WriteBytes(a.ID[:])
	bw.WriteB(byte(a.AssetType))
	bw.WriteString(a.Name)
	a.Amount.EncodeBinary(bw)
	a.Available.EncodeBinary(bw)
	bw.WriteB(byte(a.Precision))
	bw.WriteB(byte(a.FeeMode))
	bw.WriteBytes(a.FeeAddress[:])

	a.Owner.EncodeBinary(bw)

	bw.WriteBytes(a.Admin[:])
	bw.WriteBytes(a.Issuer[:])
	bw.WriteU32LE(a.Expiration)
	bw.WriteBool(a.IsFrozen)
}

// GetName returns the asset name based on its type.
func (a *Asset) GetName() string {

	if a.AssetType == transaction.GoverningToken {
		return "NEO"
	} else if a.AssetType == transaction.UtilityToken {
		return "NEOGas"
	}

	return a.Name
}
