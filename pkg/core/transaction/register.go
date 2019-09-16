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

	// Amount registered
	// Unlimited mode -0.00000001
	Amount util.Fixed8

	// Decimals
	Precision uint8

	// Public key of the owner
	Owner *keys.PublicKey

	Admin util.Uint160
}

// DecodeBinary implements the Payload interface.
func (tx *RegisterTX) DecodeBinary(br *io.BinReader) error {
	br.ReadLE(&tx.AssetType)

	tx.Name = br.ReadString()

	br.ReadLE(&tx.Amount)
	br.ReadLE(&tx.Precision)
	if br.Err != nil {
		return br.Err
	}

	tx.Owner = &keys.PublicKey{}
	if err := tx.Owner.DecodeBinary(br); err != nil {
		return err
	}

	br.ReadLE(&tx.Admin)
	return br.Err
}

// EncodeBinary implements the Payload interface.
func (tx *RegisterTX) EncodeBinary(bw *io.BinWriter) error {
	bw.WriteLE(tx.AssetType)
	bw.WriteString(tx.Name)
	bw.WriteLE(tx.Amount)
	bw.WriteLE(tx.Precision)
	bw.WriteLE(tx.Owner.Bytes())
	bw.WriteLE(tx.Admin)
	return bw.Err
}
