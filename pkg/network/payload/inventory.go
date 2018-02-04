package payload

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
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

// Valid returns true if the inventory (type) is known.
func (i InventoryType) Valid() bool {
	return i == BlockType || i == TXType || i == ConsensusType
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
	Hashes []util.Uint256
}

// NewInventory return a pointer to an Inventory.
func NewInventory(typ InventoryType, hashes []util.Uint256) *Inventory {
	return &Inventory{
		Type:   typ,
		Hashes: hashes,
	}
}

// DecodeBinary implements the Payload interface.
func (p *Inventory) DecodeBinary(r io.Reader) error {
	err := binary.Read(r, binary.LittleEndian, &p.Type)
	listLen := util.ReadVarUint(r)

	p.Hashes = make([]util.Uint256, listLen)
	for i := 0; i < int(listLen); i++ {
		if err := binary.Read(r, binary.LittleEndian, &p.Hashes[i]); err != nil {
			return err
		}
	}

	return err
}

// EncodeBinary implements the Payload interface.
func (p *Inventory) EncodeBinary(w io.Writer) error {
	listLen := len(p.Hashes)
	err := binary.Write(w, binary.LittleEndian, p.Type)
	err = util.WriteVarUint(w, uint64(listLen))

	for i := 0; i < len(p.Hashes); i++ {
		if err := binary.Write(w, binary.LittleEndian, p.Hashes[i]); err != nil {
			return err
		}
	}

	return err
}

// Size implements the Payloader interface.
func (p *Inventory) Size() uint32 {
	return 1 + 1 + 32 // ?
}
