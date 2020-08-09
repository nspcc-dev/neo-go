package consensus

import (
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// changeView represents dBFT ChangeView message.
type changeView struct {
	newViewNumber byte
	timestamp     uint32
}

var _ payload.ChangeView = (*changeView)(nil)

// EncodeBinary implements io.Serializable interface.
func (c *changeView) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(c.timestamp)
}

// DecodeBinary implements io.Serializable interface.
func (c *changeView) DecodeBinary(r *io.BinReader) {
	c.timestamp = r.ReadU32LE()
}

// NewViewNumber implements payload.ChangeView interface.
func (c changeView) NewViewNumber() byte { return c.newViewNumber }

// SetNewViewNumber implements payload.ChangeView interface.
func (c *changeView) SetNewViewNumber(view byte) { c.newViewNumber = view }

// Timestamp implements payload.ChangeView interface.
func (c changeView) Timestamp() uint64 { return uint64(c.timestamp) * nanoInSec }

// SetTimestamp implements payload.ChangeView interface.
func (c *changeView) SetTimestamp(ts uint64) { c.timestamp = uint32(ts / nanoInSec) }

// Reason implements payload.ChangeView interface.
func (c changeView) Reason() payload.ChangeViewReason { return payload.CVUnknown }

// SetReason implements payload.ChangeView interface.
func (c *changeView) SetReason(_ payload.ChangeViewReason) {}
