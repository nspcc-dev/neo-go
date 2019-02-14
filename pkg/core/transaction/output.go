package transaction

import (
	"encoding/binary"
	"encoding/json"
	"io"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Output represents a Transaction output.
type Output struct {
	// The NEO asset id used in the transaction.
	AssetID util.Uint256

	// Amount of AssetType send or received.
	Amount util.Fixed8

	// The address of the recipient.
	ScriptHash util.Uint160

	// The position of the Output in slice []Output. This is actually set in NewTransactionOutputRaw
	// and used for diplaying purposes.
	Position int
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

// Size returns the size in bytes of the Output
func (out *Output) Size() int {
	return out.AssetID.Size() + out.Amount.Size() + out.ScriptHash.Size()
}

// MarshalJSON implements the Marshaler interface
func (out *Output) MarshalJSON() ([]byte, error) {
	j, err := json.Marshal(
		struct {
			Asset   util.Uint256 `json:"asset"`
			Value   util.Fixed8  `json:"value"`
			Address string       `json:"address"`
			N       int          `json:"n"`
		}{out.AssetID,
			out.Amount,
			crypto.AddressFromUint160(out.ScriptHash),
			out.Position})
	if err != nil {
		return nil, err
	}
	return j, nil
}
