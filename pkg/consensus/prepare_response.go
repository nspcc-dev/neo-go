package consensus

import (
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// prepareResponse represents dBFT PrepareResponse message.
type prepareResponse struct {
	preparationHash util.Uint256
}

var _ payload.PrepareResponse = (*prepareResponse)(nil)

// EncodeBinary implements io.Serializable interface.
func (p *prepareResponse) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(p.preparationHash[:])
}

// DecodeBinary implements io.Serializable interface.
func (p *prepareResponse) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(p.preparationHash[:])
}

// PreparationHash implements payload.PrepareResponse interface.
func (p *prepareResponse) PreparationHash() util.Uint256 { return p.preparationHash }

// SetPreparationHash implements payload.PrepareResponse interface.
func (p *prepareResponse) SetPreparationHash(h util.Uint256) { p.preparationHash = h }
