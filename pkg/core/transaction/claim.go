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
	lenClaims := util.ReadVarUint(r)
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
	if err := util.WriteVarUint(w, uint64(len(tx.Claims))); err != nil {
		return err
	}
	for _, claim := range tx.Claims {
		if err := claim.EncodeBinary(w); err != nil {
			return err
		}
	}
	return nil
}
