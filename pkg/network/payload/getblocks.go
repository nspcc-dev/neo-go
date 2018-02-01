package payload

import (
	"encoding/binary"
	"io"

	. "github.com/anthdm/neo-go/pkg/util"
)

// GetBlocks payload
type GetBlocks struct {
	// hash of latest block that node requests
	HashStart []Uint256
	// hash of last block that node requests
	HashStop Uint256
}

// NewGetBlocks return a pointer to a GetBlocks object.
func NewGetBlocks(start []Uint256, stop Uint256) *GetBlocks {
	return &GetBlocks{
		HashStart: start,
		HashStop:  stop,
	}
}

// DecodeBinary implements the payload interface.
func (p *GetBlocks) DecodeBinary(r io.Reader) error {
	var lenStart uint8

	err := binary.Read(r, binary.LittleEndian, &lenStart)
	p.HashStart = make([]Uint256, lenStart)
	err = binary.Read(r, binary.LittleEndian, &p.HashStart)
	err = binary.Read(r, binary.LittleEndian, &p.HashStop)

	return err
}

// EncodeBinary implements the payload interface.
func (p *GetBlocks) EncodeBinary(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, uint8(len(p.HashStart)))
	err = binary.Write(w, binary.LittleEndian, p.HashStart)
	err = binary.Write(w, binary.LittleEndian, p.HashStop)

	return err
}

// Size implements the payload interface.
func (p *GetBlocks) Size() uint32 { return 0 }
