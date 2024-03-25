package consensus

import (
	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// prepareResponse represents dBFT PrepareResponse message.
type prepareResponse struct {
	preparationHash util.Uint256
}

var _ dbft.PrepareResponse[util.Uint256] = (*prepareResponse)(nil)

// EncodeBinary implements the io.Serializable interface.
func (p *prepareResponse) EncodeBinary(w *io.BinWriter) {
	w.WriteBytes(p.preparationHash[:])
}

// DecodeBinary implements the io.Serializable interface.
func (p *prepareResponse) DecodeBinary(r *io.BinReader) {
	r.ReadBytes(p.preparationHash[:])
}

// PreparationHash implements the payload.PrepareResponse interface.
func (p *prepareResponse) PreparationHash() util.Uint256 { return p.preparationHash }
