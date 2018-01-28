package payload

import (
	"encoding/binary"
	"io"

	. "github.com/anthdm/neo-go/pkg/util"
)

// The node can broadcast the object information it owns by this message.
// The message can be sent automatically or can be used to answer getbloks messages.

// InventoryType is the type of an object in the Inventory message.
type InventoryType uint8

// String implements the Stringer interface.
func (i InventoryType) String() string {
	switch i {
	case 0x01:
		return "block"
	case 0x02:
		return "TX"
	case 0xe0:
		return "consensus"
	default:
		return "unknown inventory type"
	}
}

// List of valid InventoryTypes.
const (
	BlockType     InventoryType = 0x01 // 1
	TXType                      = 0x02 // 2
	ConsensusType               = 0xe0 // 224
)

// Inventory payload
type Inventory struct {
	// Type if the object hash.
	Type InventoryType
	// The hash of the object (uint256).
	Hash Uint256
}

// DecodeBinary implements the Payload interface.
func (p *Inventory) DecodeBinary(r io.Reader) error {
	// TODO: is there a list len?
	// The first byte is the type the second byte seems to be
	// always one on docker privnet.
	var listLen uint8
	err := binary.Read(r, binary.LittleEndian, &p.Type)
	err = binary.Read(r, binary.LittleEndian, &listLen)
	err = binary.Read(r, binary.LittleEndian, &p.Hash)

	return err
}

// EncodeBinary implements the Payload interface.
func (p *Inventory) EncodeBinary(w io.Writer) error {
	// TODO
	return nil
}

// Size implements the Payloader interface.
func (p *Inventory) Size() uint32 {
	return 1 + 1 + 32 // ?
}
