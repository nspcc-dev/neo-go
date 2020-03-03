package consensus

import (
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/io"
)

// recoveryRequest represents dBFT RecoveryRequest message.
type recoveryRequest struct {
	timestamp uint32
}

var _ payload.RecoveryRequest = (*recoveryRequest)(nil)

// DecodeBinary implements io.Serializable interface.
func (m *recoveryRequest) DecodeBinary(r *io.BinReader) {
	m.timestamp = r.ReadU32LE()
}

// EncodeBinary implements io.Serializable interface.
func (m *recoveryRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteU32LE(m.timestamp)
}

// Timestamp implements payload.RecoveryRequest interface.
func (m *recoveryRequest) Timestamp() uint32 { return m.timestamp }

// SetTimestamp implements payload.RecoveryRequest interface.
func (m *recoveryRequest) SetTimestamp(ts uint32) { m.timestamp = ts }
