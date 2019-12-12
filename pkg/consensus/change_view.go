package consensus

import (
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/nspcc-dev/dbft/payload"
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
func (c changeView) Timestamp() uint32 { return c.timestamp }

// SetTimestamp implements payload.ChangeView interface.
func (c *changeView) SetTimestamp(ts uint32) { c.timestamp = ts }
