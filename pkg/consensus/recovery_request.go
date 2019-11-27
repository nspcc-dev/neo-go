package consensus

import (
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/nspcc-dev/dbft/payload"
)

// recoveryRequest represents dBFT RecoveryRequest message.
type recoveryRequest struct {
	timestamp uint32
}

var _ payload.RecoveryRequest = (*recoveryRequest)(nil)

// DecodeBinary implements io.Serializable interface.
func (m *recoveryRequest) DecodeBinary(r *io.BinReader) {
	r.ReadLE(&m.timestamp)
}

// EncodeBinary implements io.Serializable interface.
func (m *recoveryRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(m.timestamp)
}

// Timestamp implements payload.RecoveryRequest interface.
func (m *recoveryRequest) Timestamp() uint32 { return m.timestamp }

// SetTimestamp implements payload.RecoveryRequest interface.
func (m *recoveryRequest) SetTimestamp(ts uint32) { m.timestamp = ts }
