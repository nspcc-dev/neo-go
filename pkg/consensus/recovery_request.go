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
func (m *recoveryRequest) Timestamp() uint64 { return uint64(m.timestamp) * nanoInSec }

// SetTimestamp implements payload.RecoveryRequest interface.
func (m *recoveryRequest) SetTimestamp(ts uint64) { m.timestamp = uint32(ts / nanoInSec) }
