package payload

import (
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// The node can broadcast the object information it owns by this message.
// The message can be sent automatically or can be used to answer getblock messages.

// InventoryType is the type of an object in the Inventory message.
type InventoryType uint8

// String implements the Stringer interface.
func (i InventoryType) String() string {
	switch i {
	case 0x01:
		return "TX"
	case 0x02:
		return "block"
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
	TXType        InventoryType = 0x01 // 1
	BlockType     InventoryType = 0x02 // 2
	ConsensusType InventoryType = 0xe0 // 224
)

// Inventory payload.
type Inventory struct {
	// Type if the object hash.
	Type InventoryType

	// A list of hashes.
	Hashes []util.Uint256
}

// NewInventory return a pointer to an Inventory.
func NewInventory(typ InventoryType, hashes []util.Uint256) *Inventory {
	return &Inventory{
		Type:   typ,
		Hashes: hashes,
	}
}

// DecodeBinary implements Serializable interface.
func (p *Inventory) DecodeBinary(br *io.BinReader) {
	br.ReadLE(&p.Type)

	listLen := br.ReadVarUint()
	p.Hashes = make([]util.Uint256, listLen)
	for i := 0; i < int(listLen); i++ {
		br.ReadLE(&p.Hashes[i])
	}
}

// EncodeBinary implements Serializable interface.
func (p *Inventory) EncodeBinary(bw *io.BinWriter) {
	bw.WriteLE(p.Type)

	listLen := len(p.Hashes)
	bw.WriteVarUint(uint64(listLen))
	for i := 0; i < listLen; i++ {
		bw.WriteLE(p.Hashes[i])
	}
}
