package transaction

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Output represents a Transaction output.
type Output struct {
	// The NEO asset id used in the transaction.
	AssetID util.Uint256

	// Amount of AssetType send or received.
	Amount util.Fixed8

	// The address of the remittee.
	ScriptHash util.Uint160
}

// NewOutput returns a new transaction output.
func NewOutput(assetID util.Uint256, amount util.Fixed8, scriptHash util.Uint160) *Output {
	return &Output{
		AssetID:    assetID,
		Amount:     amount,
		ScriptHash: scriptHash,
	}
}

// DecodeBinary implements the Payload interface.
func (out *Output) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &out.AssetID); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &out.Amount); err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &out.ScriptHash)
}

// EncodeBinary implements the Payload interface.
func (out *Output) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, out.AssetID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, out.Amount); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, out.ScriptHash)
}
