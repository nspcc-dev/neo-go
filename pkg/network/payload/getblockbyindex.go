package payload

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// GetBlockByIndex payload.
type GetBlockByIndex struct {
	IndexStart uint32
	Count      int16
}

// NewGetBlockByIndex returns GetBlockByIndex payload with specified start index and count.
func NewGetBlockByIndex(indexStart uint32, count int16) *GetBlockByIndex {
	return &GetBlockByIndex{
		IndexStart: indexStart,
		Count:      count,
	}
}

// DecodeBinary implements Serializable interface.
func (d *GetBlockByIndex) DecodeBinary(br *io.BinReader) {
	d.IndexStart = br.ReadU32LE()
	d.Count = int16(br.ReadU16LE())
	if d.Count < -1 || d.Count == 0 || d.Count > MaxHeadersAllowed {
		br.Err = errors.New("invalid block count")
	}
}

// EncodeBinary implements Serializable interface.
func (d *GetBlockByIndex) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(d.IndexStart)
	bw.WriteU16LE(uint16(d.Count))
}
