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

// NewGetBlocks return a pointer to a GetBlocks object.
func NewGetBlocks(start []util.Uint256, stop util.Uint256) *GetBlocks {
	return &GetBlocks{
		HashStart: start,
		HashStop:  stop,
	}
}

// DecodeBinary implements the payload interface.
func (p *GetBlocks) DecodeBinary(br *io.BinReader) error {
	lenStart := br.ReadVarUint()
	p.HashStart = make([]util.Uint256, lenStart)

	br.ReadLE(&p.HashStart)
	br.ReadLE(&p.HashStop)
	return br.Err
}

// EncodeBinary implements the payload interface.
func (p *GetBlocks) EncodeBinary(bw *io.BinWriter) error {
	bw.WriteVarUint(uint64(len(p.HashStart)))
	bw.WriteLE(p.HashStart)
	bw.WriteLE(p.HashStop)
	return bw.Err
}
