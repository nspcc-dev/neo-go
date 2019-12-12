package transaction

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/io"
)

// Attribute represents a Transaction attribute.
type Attribute struct {
	Usage AttrUsage
	Data  []byte
}

// DecodeBinary implements Serializable interface.
func (attr *Attribute) DecodeBinary(br *io.BinReader) {
	attr.Usage = AttrUsage(br.ReadByte())

	// very special case
	if attr.Usage == ECDH02 || attr.Usage == ECDH03 {
		attr.Data = make([]byte, 33)
		attr.Data[0] = byte(attr.Usage)
		br.ReadBytes(attr.Data[1:])
		return
	}
	var datasize uint64
	switch attr.Usage {
	case ContractHash, Vote, Hash1, Hash2, Hash3, Hash4, Hash5,
		Hash6, Hash7, Hash8, Hash9, Hash10, Hash11, Hash12, Hash13,
		Hash14, Hash15:
		datasize = 32
	case Script:
		datasize = 20
	case DescriptionURL:
		// It's not VarUint as per C# implementation, dunno why
		var urllen = br.ReadByte()
		datasize = uint64(urllen)
	case Description, Remark, Remark1, Remark2, Remark3, Remark4,
		Remark5, Remark6, Remark7, Remark8, Remark9, Remark10, Remark11,
		Remark12, Remark13, Remark14, Remark15:
		datasize = br.ReadVarUint()
	default:
		br.Err = fmt.Errorf("failed decoding TX attribute usage: 0x%2x", int(attr.Usage))
		return
	}
	attr.Data = make([]byte, datasize)
	br.ReadBytes(attr.Data)
}

// EncodeBinary implements Serializable interface.
func (attr *Attribute) EncodeBinary(bw *io.BinWriter) {
	bw.WriteByte(byte(attr.Usage))
	switch attr.Usage {
	case ECDH02, ECDH03:
		bw.WriteBytes(attr.Data[1:])
	case Description, Remark, Remark1, Remark2, Remark3, Remark4,
		Remark5, Remark6, Remark7, Remark8, Remark9, Remark10, Remark11,
		Remark12, Remark13, Remark14, Remark15:
		bw.WriteVarBytes(attr.Data)
	case DescriptionURL:
		bw.WriteByte(byte(len(attr.Data)))
		fallthrough
	case Script, ContractHash, Vote, Hash1, Hash2, Hash3, Hash4, Hash5, Hash6,
		Hash7, Hash8, Hash9, Hash10, Hash11, Hash12, Hash13, Hash14, Hash15:
		bw.WriteBytes(attr.Data)
	default:
		bw.Err = fmt.Errorf("failed encoding TX attribute usage: 0x%2x", attr.Usage)
	}
}

// MarshalJSON implements the json Marshaller interface.
func (attr *Attribute) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"usage": attr.Usage.String(),
		"data":  hex.EncodeToString(attr.Data),
	})
}
