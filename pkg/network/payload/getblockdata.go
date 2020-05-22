package payload

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// maximum number of blocks to query about
const maxBlockCount = 500

// GetBlockData payload
type GetBlockData struct {
	IndexStart uint32
	Count      uint16
}

// NewGetBlockData returns GetBlockData payload with specified start index and count
func NewGetBlockData(indexStart uint32, count uint16) *GetBlockData {
	return &GetBlockData{
		IndexStart: indexStart,
		Count:      count,
	}
}

// DecodeBinary implements Serializable interface.
func (d *GetBlockData) DecodeBinary(br *io.BinReader) {
	d.IndexStart = br.ReadU32LE()
	d.Count = br.ReadU16LE()
	if d.Count == 0 || d.Count > maxBlockCount {
		br.Err = errors.New("invalid block count")
	}
}

// EncodeBinary implements Serializable interface.
func (d *GetBlockData) EncodeBinary(bw *io.BinWriter) {
	bw.WriteU32LE(d.IndexStart)
	bw.WriteU16LE(d.Count)
}
