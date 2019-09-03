package transaction

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// ClaimTX represents a claim transaction.
type ClaimTX struct {
	Claims []*Input
}

// DecodeBinary implements the Payload interface.
func (tx *ClaimTX) DecodeBinary(r io.Reader) error {
	br := util.BinReader{R: r}
	lenClaims := br.ReadVarUint()
	if br.Err != nil {
		return br.Err
	}
	tx.Claims = make([]*Input, lenClaims)
	for i := 0; i < int(lenClaims); i++ {
		tx.Claims[i] = &Input{}
		if err := tx.Claims[i].DecodeBinary(r); err != nil {
			return err
		}
	}
	return nil
}

// EncodeBinary implements the Payload interface.
func (tx *ClaimTX) EncodeBinary(w io.Writer) error {
	bw := util.BinWriter{W: w}
	bw.WriteVarUint(uint64(len(tx.Claims)))
	if bw.Err != nil {
		return bw.Err
	}
	for _, claim := range tx.Claims {
		if err := claim.EncodeBinary(w); err != nil {
			return err
		}
	}
	return nil
}

// Size returns serialized binary size for this transaction.
func (tx *ClaimTX) Size() int {
	sz := util.GetVarSize(uint64(len(tx.Claims)))
	for _, claim := range tx.Claims {
		sz += claim.Size()
	}
	return sz
}
