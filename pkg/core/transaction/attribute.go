package transaction

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Attribute represents a Transaction attribute.
type Attribute struct {
	Usage AttrUsage
	Data  []byte
}

// DecodeBinary implements the Payloader interface.
func (attr *Attribute) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &attr.Usage); err != nil {
		return err
	}

	if attr.Usage == ContractHash ||
		attr.Usage == Vote ||
		(attr.Usage >= Hash1 && attr.Usage <= Hash15) {
		attr.Data = make([]byte, 32)
		if err := binary.Read(r, binary.LittleEndian, attr.Data); err != nil {
			return err
		}
	} else if attr.Usage == ECDH02 || attr.Usage == ECDH03 {
		attr.Data = make([]byte, 33)
		attr.Data[0] = byte(attr.Usage)
		if err := binary.Read(r, binary.LittleEndian, attr.Data[1:]); err != nil {
			return err
		}
	} else if attr.Usage == Script {
		attr.Data = make([]byte, 20)
		if err := binary.Read(r, binary.LittleEndian, attr.Data); err != nil {
			return err
		}
	} else if attr.Usage == DescriptionUrl {
		attr.Data = make([]byte, 1)
		if err := binary.Read(r, binary.LittleEndian, attr.Data); err != nil {
			return err
		}
	} else if attr.Usage == Description || attr.Usage >= Remark {
		lenData := util.ReadVarUint(r)
		attr.Data = make([]byte, lenData)
		if err := binary.Read(r, binary.LittleEndian, attr.Data); err != nil {
			return err
		}
	} else {
		return errors.New("format error in decoding transaction attribute")
	}
	return nil
}

// EncodeBinary implements the Payload interface.
func (attr *Attribute) EncodeBinary(w io.Writer) error { return nil }
