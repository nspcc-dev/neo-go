package payload

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Maximum inventory hashes number is limited to 500.
const (
	MaxHashesCount = 500
)

// GetBlocks contains getblocks message payload fields.
type GetBlocks struct {
	// Hash of the latest block that node requests.
	HashStart util.Uint256
	Count     int16
}

// NewGetBlocks returns a pointer to a GetBlocks object.
func NewGetBlocks(start util.Uint256, count int16) *GetBlocks {
	return &GetBlocks{
		HashStart: start,
		Count:     count,
	}
}

// DecodeBinary implements Serializable interface.
func (p *GetBlocks) DecodeBinary(br *io.BinReader) {
	p.HashStart.DecodeBinary(br)
	p.Count = int16(br.ReadU16LE())
	if p.Count < -1 || p.Count == 0 {
		br.Err = errors.New("invalid count")
	}
}

// EncodeBinary implements Serializable interface.
func (p *GetBlocks) EncodeBinary(bw *io.BinWriter) {
	p.HashStart.EncodeBinary(bw)
	bw.WriteU16LE(uint16(p.Count))
}
