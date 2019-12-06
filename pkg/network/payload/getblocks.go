package payload

import (
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// GetBlocks contains fields and methods to be shared with the
type GetBlocks struct {
	// hash of latest block that node requests
	HashStart []util.Uint256
	// hash of last block that node requests
	HashStop util.Uint256
}

// NewGetBlocks returns a pointer to a GetBlocks object.
func NewGetBlocks(start []util.Uint256, stop util.Uint256) *GetBlocks {
	return &GetBlocks{
		HashStart: start,
		HashStop:  stop,
	}
}

// DecodeBinary implements Serializable interface.
func (p *GetBlocks) DecodeBinary(br *io.BinReader) {
	br.ReadArray(&p.HashStart)
	br.ReadLE(&p.HashStop)
}

// EncodeBinary implements Serializable interface.
func (p *GetBlocks) EncodeBinary(bw *io.BinWriter) {
	bw.WriteArray(p.HashStart)
	bw.WriteBytes(p.HashStop[:])
}
