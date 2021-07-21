package payload

import (
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// MaxMPTHashesCount is the maximum number of requested MPT nodes hashes.
const MaxMPTHashesCount = 32

// MPTInventory payload.
type MPTInventory struct {
	// A list of requested MPT nodes hashes.
	Hashes []util.Uint256
}

// NewMPTInventory return a pointer to an MPTInventory.
func NewMPTInventory(hashes []util.Uint256) *MPTInventory {
	return &MPTInventory{
		Hashes: hashes,
	}
}

// DecodeBinary implements Serializable interface.
func (p *MPTInventory) DecodeBinary(br *io.BinReader) {
	br.ReadArray(&p.Hashes, MaxMPTHashesCount)
}

// EncodeBinary implements Serializable interface.
func (p *MPTInventory) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(p.Hashes)
}
