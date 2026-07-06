package consensus

import (
	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// changeView represents dBFT ChangeView message.
type changeView struct {
	newViewNumber  byte
	timestamp      uint64
	reason         dbft.ChangeViewReason
	rejectedHashes []util.Uint256
}

var _ dbft.ChangeView = (*changeView)(nil)

// EncodeBinary implements the io.Serializable interface.
func (c *changeView) EncodeBinary(w *io.BinWriter) {
	w.WriteU64LE(c.timestamp)
	w.WriteB(byte(c.reason))
	if c.reason == dbft.CVTxInvalid || c.reason == dbft.CVTxRejectedByPolicy {
		w.WriteArray(c.rejectedHashes)
	}
}

// DecodeBinary implements the io.Serializable interface.
func (c *changeView) DecodeBinary(r *io.BinReader) {
	c.timestamp = r.ReadU64LE()
	c.reason = dbft.ChangeViewReason(r.ReadB())
	if c.reason == dbft.CVTxInvalid || c.reason == dbft.CVTxRejectedByPolicy {
		r.ReadArray(&c.rejectedHashes)
	}
}

// NewViewNumber implements the payload.ChangeView interface.
func (c changeView) NewViewNumber() byte { return c.newViewNumber }

// Reason implements the payload.ChangeView interface.
func (c changeView) Reason() dbft.ChangeViewReason { return c.reason }
