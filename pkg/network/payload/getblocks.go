package payload

import (
	"encoding/binary"
	"io"

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
func (p *GetBlocks) DecodeBinary(r io.Reader) error {
	lenStart := util.ReadVarUint(r)
	p.HashStart = make([]util.Uint256, lenStart)

	if err := binary.Read(r, binary.LittleEndian, &p.HashStart); err != nil {
		return err
	}

	// If the reader returns EOF we know the hashStop is not encoded.
	err := binary.Read(r, binary.LittleEndian, &p.HashStop)
	if err == io.EOF {
		return nil
	}

	return err
}

// EncodeBinary implements the payload interface.
func (p *GetBlocks) EncodeBinary(w io.Writer) error {
	if err := util.WriteVarUint(w, uint64(len(p.HashStart))); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, p.HashStart); err != nil {
		return err
	}

	// Only write hashStop if its not filled with zero bytes.
	var emtpy util.Uint256
	if p.HashStop != emtpy {
		return binary.Write(w, binary.LittleEndian, p.HashStop)
	}

	return nil
}

// Size implements the payload interface.
func (p *GetBlocks) Size() uint32 { return 0 }
