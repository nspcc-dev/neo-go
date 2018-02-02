package payload

import (
	"encoding/binary"
	"fmt"
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
	p := &GetBlocks{}
	p.HashStart = start
	p.HashStop = stop
	return p
}

// DecodeBinary implements the payload interface.
func (p *GetBlocks) DecodeBinary(r io.Reader) error {
	lenStart := util.ReadVarUint(r)
	fmt.Println(lenStart)
	p.HashStart = make([]util.Uint256, lenStart)
	err := binary.Read(r, binary.LittleEndian, &p.HashStart)
	err = binary.Read(r, binary.LittleEndian, &p.HashStop)

	fmt.Println(p)
	if err == io.EOF {
		return nil
	}

	return err
}

// EncodeBinary implements the payload interface.
func (p *GetBlocks) EncodeBinary(w io.Writer) error {
	err := util.WriteVarUint(w, uint64(len(p.HashStart)))
	err = binary.Write(w, binary.LittleEndian, p.HashStart)
	//err = binary.Write(w, binary.LittleEndian, p.HashStop)

	return err
}

// Size implements the payload interface.
func (p *GetBlocks) Size() uint32 { return 0 }
