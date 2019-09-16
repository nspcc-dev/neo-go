package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
)

// ClaimTX represents a claim transaction.
type ClaimTX struct {
	Claims []*Input
}

// DecodeBinary implements Serializable interface.
func (tx *ClaimTX) DecodeBinary(br *io.BinReader) {
	lenClaims := br.ReadVarUint()
	tx.Claims = make([]*Input, lenClaims)
	for i := 0; i < int(lenClaims); i++ {
		tx.Claims[i] = &Input{}
		tx.Claims[i].DecodeBinary(br)
	}
}

// EncodeBinary implements Serializable interface.
func (tx *ClaimTX) EncodeBinary(bw *io.BinWriter) {
	bw.WriteVarUint(uint64(len(tx.Claims)))
	for _, claim := range tx.Claims {
		claim.EncodeBinary(bw)
	}
}
