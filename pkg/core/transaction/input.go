package transaction

import (
	"encoding/binary"
	"io"
	"unsafe"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Input represents a Transaction input (CoinReference).
type Input struct {
	// The hash of the previous transaction.
	PrevHash util.Uint256 `json:"txid"`

	// The index of the previous transaction.
	PrevIndex uint16 `json:"vout"`
}

// DecodeBinary implements the Payload interface.
func (in *Input) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &in.PrevHash); err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &in.PrevIndex)
}

// EncodeBinary implements the Payload interface.
func (in *Input) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, in.PrevHash); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, in.PrevIndex); err != nil {
		return err
	}
	return nil
}

// Size returns the size in bytes of the Input
func (in *Input) Size() int {
	var ui16 uint16
	return in.PrevHash.Size() + int(unsafe.Sizeof(ui16))
}
