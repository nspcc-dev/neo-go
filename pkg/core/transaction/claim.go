package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
)

// ClaimTX represents a claim transaction.
type ClaimTX struct {
	Claims []*Input
}

// DecodeBinary implements the Payload interface.
func (tx *ClaimTX) DecodeBinary(br *io.BinReader) error {
	lenClaims := br.ReadVarUint()
	if br.Err != nil {
		return br.Err
	}
	tx.Claims = make([]*Input, lenClaims)
	for i := 0; i < int(lenClaims); i++ {
		tx.Claims[i] = &Input{}
		if err := tx.Claims[i].DecodeBinary(br); err != nil {
			return err
		}
	}
	return nil
}

// EncodeBinary implements the Payload interface.
func (tx *ClaimTX) EncodeBinary(bw *io.BinWriter) error {
	bw.WriteVarUint(uint64(len(tx.Claims)))
	if bw.Err != nil {
		return bw.Err
	}
	for _, claim := range tx.Claims {
		if err := claim.EncodeBinary(bw); err != nil {
			return err
		}
	}
	return nil
}
