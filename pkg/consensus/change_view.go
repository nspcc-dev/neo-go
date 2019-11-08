package consensus

import "github.com/CityOfZion/neo-go/pkg/io"

// changeView represents dBFT ChangeView message.
type changeView struct {
	NewViewNumber byte
	Timestamp     uint32
}

// EncodeBinary implements io.Serializable interface.
func (c *changeView) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(c.Timestamp)
}

// DecodeBinary implements io.Serializable interface.
func (c *changeView) DecodeBinary(r *io.BinReader) {
	r.ReadLE(&c.Timestamp)
}
