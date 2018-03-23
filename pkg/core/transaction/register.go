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
	Name string

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

	var err error
	tx.Name, err = util.ReadVarString(r)
	if err != nil {
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
	if err := binary.Write(w, binary.LittleEndian, tx.AssetType); err != nil {
		return err
	}
	if err := util.WriteVarString(w, tx.Name); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, tx.Amount); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, tx.Precision); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, tx.Owner.Bytes()); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, tx.Admin)
}
