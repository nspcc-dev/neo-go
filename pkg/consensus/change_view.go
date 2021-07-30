package consensus

import (
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// changeView represents dBFT ChangeView message.
type changeView struct {
	newViewNumber byte
	timestamp     uint64
	reason        payload.ChangeViewReason
}

var _ payload.ChangeView = (*changeView)(nil)

// EncodeBinary implements io.Serializable interface.
func (c *changeView) EncodeBinary(w io.BinaryWriter) {
	w.WriteU64LE(c.timestamp)
	w.WriteB(byte(c.reason))
}

// DecodeBinary implements io.Serializable interface.
func (c *changeView) DecodeBinary(r *io.BinReader) {
	c.timestamp = r.ReadU64LE()
	c.reason = payload.ChangeViewReason(r.ReadB())
}

// NewViewNumber implements payload.ChangeView interface.
func (c changeView) NewViewNumber() byte { return c.newViewNumber }

// SetNewViewNumber implements payload.ChangeView interface.
func (c *changeView) SetNewViewNumber(view byte) { c.newViewNumber = view }

// Timestamp implements payload.ChangeView interface.
func (c changeView) Timestamp() uint64 { return c.timestamp * nsInMs }

// SetTimestamp implements payload.ChangeView interface.
func (c *changeView) SetTimestamp(ts uint64) { c.timestamp = ts / nsInMs }

// Reason implements payload.ChangeView interface.
func (c changeView) Reason() payload.ChangeViewReason { return c.reason }

// SetReason implements payload.ChangeView interface.
func (c *changeView) SetReason(reason payload.ChangeViewReason) { c.reason = reason }
