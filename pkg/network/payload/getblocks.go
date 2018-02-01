package payload

import (
	"encoding/binary"
	"io"

	. "github.com/CityOfZion/neo-go/pkg/util"
)

// HashStartStop contains fields and methods to be shared with the
// "GetBlocks" and "GetHeaders" payload.
type HashStartStop struct {
	// hash of latest block that node requests
	HashStart []Uint256
	// hash of last block that node requests
	HashStop Uint256
}

// DecodeBinary implements the payload interface.
func (p *HashStartStop) DecodeBinary(r io.Reader) error {
	var lenStart uint8

	err := binary.Read(r, binary.LittleEndian, &lenStart)
	p.HashStart = make([]Uint256, lenStart)
	err = binary.Read(r, binary.LittleEndian, &p.HashStart)
	err = binary.Read(r, binary.LittleEndian, &p.HashStop)

	return err
}

// EncodeBinary implements the payload interface.
func (p *HashStartStop) EncodeBinary(w io.Writer) error {
	err := binary.Write(w, binary.LittleEndian, uint8(len(p.HashStart)))
	err = binary.Write(w, binary.LittleEndian, p.HashStart)
	err = binary.Write(w, binary.LittleEndian, p.HashStop)

	return err
}

// Size implements the payload interface.
func (p *HashStartStop) Size() uint32 { return 0 }

// GetBlocks payload
type GetBlocks struct {
	HashStartStop
}

// NewGetBlocks return a pointer to a GetBlocks object.
func NewGetBlocks(start []Uint256, stop Uint256) *GetBlocks {
	p := &GetBlocks{}
	p.HashStart = start
	p.HashStop = stop
	return p
}
