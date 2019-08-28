package transaction

import (
	"io"

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
	br := util.BinReader{R: r}
	br.ReadLE(&in.PrevHash)
	br.ReadLE(&in.PrevIndex)
	return br.Err
}

// EncodeBinary implements the Payload interface.
func (in *Input) EncodeBinary(w io.Writer) error {
	bw := util.BinWriter{W: w}
	bw.WriteLE(in.PrevHash)
	bw.WriteLE(in.PrevIndex)
	return bw.Err
}

// Size returns the size in bytes of the Input
func (in Input) Size() int {
	return in.PrevHash.Size() + 2 // 2 = sizeOf uint16
}
