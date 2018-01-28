package payload

import (
	"bytes"
	"encoding/binary"
	"fmt"

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

// UnmarshalBinary implements the Payloader interface.
func (p *Inventory) UnmarshalBinary(b []byte) error {
	// TODO: what byte is [1:2] ?
	// We have 1 byte for the type which is uint8 and 32 for the hash.
	// There is 1 byte left over.
	fmt.Println(b[0:1])
	fmt.Println(b[1:2])
	binary.Read(bytes.NewReader(b), binary.LittleEndian, &p.Type)
	p.Hash.UnmarshalBinary(b[2:len(b)])
	return nil
}

// MarshalBinary implements the Payloader interface.
func (p *Inventory) MarshalBinary() ([]byte, error) {
	return nil, nil
}

// Size implements the Payloader interface.
func (p *Inventory) Size() uint32 {
	return 1 + 1 + 32 // ?
}
