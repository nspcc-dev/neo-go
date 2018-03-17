package transaction

import (
	"encoding/binary"
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
