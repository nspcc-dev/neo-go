package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// RegisterTX represents a register transaction.
// NOTE: This is deprecated.
type RegisterTX struct {
	// The type of the asset being registered.
	AssetType AssetType

	// Name of the asset being registered.
	Name string

	// Amount registered.
	// Unlimited mode -0.00000001.
	Amount util.Fixed8

	// Decimals.
	Precision uint8

	// Public key of the owner.
	Owner *keys.PublicKey

	Admin util.Uint160
}

// DecodeBinary implements Serializable interface.
func (tx *RegisterTX) DecodeBinary(br *io.BinReader) {
	br.ReadLE(&tx.AssetType)

	tx.Name = br.ReadString()

	br.ReadLE(&tx.Amount)
	br.ReadLE(&tx.Precision)

	tx.Owner = &keys.PublicKey{}
	tx.Owner.DecodeBinary(br)

	br.ReadLE(&tx.Admin)
}

// EncodeBinary implements Serializable interface.
func (tx *RegisterTX) EncodeBinary(bw *io.BinWriter) {
	bw.WriteLE(tx.AssetType)
	bw.WriteString(tx.Name)
	bw.WriteLE(tx.Amount)
	bw.WriteLE(tx.Precision)
	bw.WriteLE(tx.Owner.Bytes())
	bw.WriteLE(tx.Admin)
}
