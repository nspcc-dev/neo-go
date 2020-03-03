package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// ClaimTX represents a claim transaction.
type ClaimTX struct {
	Claims []Input
}

// DecodeBinary implements Serializable interface.
func (tx *ClaimTX) DecodeBinary(br *io.BinReader) {
	br.ReadArray(&tx.Claims)
}

// EncodeBinary implements Serializable interface.
func (tx *ClaimTX) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(tx.Claims)
}
