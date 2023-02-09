package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Conflicts represents attribute for refund gas transaction.
type RefundableSystemFee struct {
}

// DecodeBinary implements the io.Serializable interface.
func (c *RefundableSystemFee) DecodeBinary(br *io.BinReader) {
}

// EncodeBinary implements the io.Serializable interface.
func (c *RefundableSystemFee) EncodeBinary(w *io.BinWriter) {
}

func (c *RefundableSystemFee) toJSONMap(m map[string]interface{}) {
}
