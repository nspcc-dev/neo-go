package transaction

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Attribute represents a Transaction attribute.
type Attribute struct {
	Usage AttrUsage
	Data  []byte
}

// DecodeBinary implements the Payload interface.
func (attr *Attribute) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &attr.Usage); err != nil {
		return err
	}
	if attr.Usage == ContractHash ||
		attr.Usage == Vote ||
		(attr.Usage >= Hash1 && attr.Usage <= Hash15) {
		attr.Data = make([]byte, 32)
		return binary.Read(r, binary.LittleEndian, attr.Data)
	}
	if attr.Usage == ECDH02 || attr.Usage == ECDH03 {
		attr.Data = make([]byte, 33)
		attr.Data[0] = byte(attr.Usage)
		return binary.Read(r, binary.LittleEndian, attr.Data[1:])
	}
	if attr.Usage == Script {
		attr.Data = make([]byte, 20)
		return binary.Read(r, binary.LittleEndian, attr.Data)
	}
	if attr.Usage == DescriptionURL {
		attr.Data = make([]byte, 1)
		return binary.Read(r, binary.LittleEndian, attr.Data)
	}
	if attr.Usage == Description || attr.Usage >= Remark {
		lenData := util.ReadVarUint(r)
		attr.Data = make([]byte, lenData)
		return binary.Read(r, binary.LittleEndian, attr.Data)
	}
	return fmt.Errorf("failed decoding TX attribute usage: 0x%2x", attr.Usage)
}

// EncodeBinary implements the Payload interface.
func (attr *Attribute) EncodeBinary(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, &attr.Usage); err != nil {
		return err
	}
	if attr.Usage == ContractHash ||
		attr.Usage == Vote ||
		(attr.Usage >= Hash1 && attr.Usage <= Hash15) {
		return binary.Write(w, binary.LittleEndian, attr.Data)
	}
	if attr.Usage == ECDH02 || attr.Usage == ECDH03 {
		attr.Data[0] = byte(attr.Usage)
		return binary.Write(w, binary.LittleEndian, attr.Data[1:33])
	}
	if attr.Usage == Script {
		return binary.Write(w, binary.LittleEndian, attr.Data)
	}
	if attr.Usage == DescriptionURL {
		if err := util.WriteVarUint(w, uint64(len(attr.Data))); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, attr.Data)
	}
	if attr.Usage == Description || attr.Usage >= Remark {
		if err := util.WriteVarUint(w, uint64(len(attr.Data))); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, attr.Data)
	}
	return fmt.Errorf("failed encoding TX attribute usage: 0x%2x", attr.Usage)
}

// Size returns the size in number bytes of the Attribute
func (attr *Attribute) Size() int {
	switch attr.Usage {
	case ContractHash, ECDH02, ECDH03, Vote,
		Hash1, Hash2, Hash3, Hash4, Hash5, Hash6, Hash7, Hash8, Hash9, Hash10, Hash11, Hash12, Hash13, Hash14, Hash15:
		return 33 // uint8 + 32 = size(attrUsage) + 32
	case Script:
		return 21 // uint8 + 20 = size(attrUsage) + 20
	case Description:
		return 2 + len(attr.Data) // uint8 + uint8+ len of data = size(attrUsage) + size(byte) + len of data
	default:
		return 1 + len(attr.Data) // uint8 + len of data = size(attrUsage) + len of data
	}
}

// MarshalJSON implements the json Marschaller interface
func (attr *Attribute) MarshalJSON() ([]byte, error) {
	j, err := json.Marshal(
		struct {
			Usage string `json:"usage"`
			Data  string `json:"data"`
		}{
			attr.Usage.String(),
			hex.EncodeToString(attr.Data),
		})
	if err != nil {
		return nil, err
	}
	return j, nil
}
