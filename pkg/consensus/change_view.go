package consensus

import (
	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// changeView represents dBFT ChangeView message.
type changeView struct {
	newViewNumber byte
	timestamp     uint64
	reason        dbft.ChangeViewReason
}

var _ dbft.ChangeView = (*changeView)(nil)

// EncodeBinary implements the io.Serializable interface.
func (c *changeView) EncodeBinary(w *io.BinWriter) {
	w.WriteU64LE(c.timestamp)
	w.WriteB(byte(c.reason))
}

// DecodeBinary implements the io.Serializable interface.
func (c *changeView) DecodeBinary(r *io.BinReader) {
	c.timestamp = r.ReadU64LE()
	c.reason = dbft.ChangeViewReason(r.ReadB())
}

// NewViewNumber implements the payload.ChangeView interface.
func (c changeView) NewViewNumber() byte { return c.newViewNumber }

// SetNewViewNumber implements the payload.ChangeView interface.
func (c *changeView) SetNewViewNumber(view byte) { c.newViewNumber = view }

// Timestamp implements the payload.ChangeView interface.
func (c changeView) Timestamp() uint64 { return c.timestamp * nsInMs }

// SetTimestamp implements the payload.ChangeView interface.
func (c *changeView) SetTimestamp(ts uint64) { c.timestamp = ts / nsInMs }

// Reason implements the payload.ChangeView interface.
func (c changeView) Reason() dbft.ChangeViewReason { return c.reason }

// SetReason implements the payload.ChangeView interface.
func (c *changeView) SetReason(reason dbft.ChangeViewReason) { c.reason = reason }
