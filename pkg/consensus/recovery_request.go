package consensus

import "github.com/CityOfZion/neo-go/pkg/io"

// recoveryRequest represents dBFT RecoveryRequest message.
type recoveryRequest struct {
	Timestamp uint32
}

// DecodeBinary implements io.Serializable interface.
func (m *recoveryRequest) DecodeBinary(r *io.BinReader) {
	r.ReadLE(&m.Timestamp)
}

// EncodeBinary implements io.Serializable interface.
func (m *recoveryRequest) EncodeBinary(w *io.BinWriter) {
	w.WriteLE(m.Timestamp)
}
