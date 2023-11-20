package transaction

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// RefundableSystemFee represents attribute for system fee refundable transaction.
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
