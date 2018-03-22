package transaction

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// RegisterTX represents a register transaction.
// NOTE: This is deprecated.
type RegisterTX struct {
	// The type of the asset being registered.
	AssetType AssetType

	// Name of the asset being registered.
	Name []byte

	// Amount registered
	// Unlimited mode -0.00000001
	Amount util.Fixed8

	// Decimals
	Precision uint8

	// Public key of the owner
	Owner *crypto.PublicKey

	Admin util.Uint160
}

// DecodeBinary implements the Payload interface.
func (tx *RegisterTX) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &tx.AssetType); err != nil {
		return err
	}
	lenName := util.ReadVarUint(r)
	tx.Name = make([]byte, lenName)
	if err := binary.Read(r, binary.LittleEndian, &tx.Name); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &tx.Amount); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &tx.Precision); err != nil {
		return err
	}

	tx.Owner = &crypto.PublicKey{}
	if err := tx.Owner.DecodeBinary(r); err != nil {
		return err
	}

	return binary.Read(r, binary.LittleEndian, &tx.Admin)
}

// EncodeBinary implements the Payload interface.
func (tx *RegisterTX) EncodeBinary(w io.Writer) error {
	return nil
}
