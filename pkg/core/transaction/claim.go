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
	tx.Claims = br.ReadArray(Input{}).([]*Input)
}

// EncodeBinary implements Serializable interface.
func (tx *ClaimTX) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(tx.Claims)
}
