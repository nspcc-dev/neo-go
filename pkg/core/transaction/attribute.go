package transaction

import (
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
	br := util.NewBinReaderFromIO(r)
	br.ReadLE(&attr.Usage)

	// very special case
	if attr.Usage == ECDH02 || attr.Usage == ECDH03 {
		attr.Data = make([]byte, 33)
		attr.Data[0] = byte(attr.Usage)
		br.ReadLE(attr.Data[1:])
		return br.Err
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
		var urllen uint8
		br.ReadLE(&urllen)
		datasize = uint64(urllen)
	case Description, Remark, Remark1, Remark2, Remark3, Remark4,
		Remark5, Remark6, Remark7, Remark8, Remark9, Remark10, Remark11,
		Remark12, Remark13, Remark14, Remark15:
		datasize = br.ReadVarUint()
	default:
		return fmt.Errorf("failed decoding TX attribute usage: 0x%2x", int(attr.Usage))
	}
	attr.Data = make([]byte, datasize)
	br.ReadLE(attr.Data)
	return br.Err
}

// EncodeBinary implements the Payload interface.
func (attr *Attribute) EncodeBinary(w io.Writer) error {
	bw := util.NewBinWriterFromIO(w)
	bw.WriteLE(&attr.Usage)
	switch attr.Usage {
	case ECDH02, ECDH03:
		bw.WriteLE(attr.Data[1:])
	case Description, Remark, Remark1, Remark2, Remark3, Remark4,
		Remark5, Remark6, Remark7, Remark8, Remark9, Remark10, Remark11,
		Remark12, Remark13, Remark14, Remark15:
		bw.WriteBytes(attr.Data)
	case DescriptionURL:
		var urllen = uint8(len(attr.Data))
		bw.WriteLE(urllen)
		fallthrough
	case Script, ContractHash, Vote, Hash1, Hash2, Hash3, Hash4, Hash5, Hash6,
		Hash7, Hash8, Hash9, Hash10, Hash11, Hash12, Hash13, Hash14, Hash15:
		bw.WriteLE(attr.Data)
	default:
		return fmt.Errorf("failed encoding TX attribute usage: 0x%2x", attr.Usage)
	}

	return bw.Err
}

// Size returns the size in number bytes of the Attribute
func (attr *Attribute) Size() int {
	sz := 1 // usage
	switch attr.Usage {
	case ContractHash, ECDH02, ECDH03, Vote,
		Hash1, Hash2, Hash3, Hash4, Hash5, Hash6, Hash7, Hash8, Hash9, Hash10, Hash11, Hash12, Hash13, Hash14, Hash15:
		sz += 32 // uint8 + 32 = size(attrUsage) + 32
	case Script:
		sz += 20 // uint8 + 20 = size(attrUsage) + 20
	case DescriptionURL:
		sz += 1 + len(attr.Data)
	default:
		sz += util.GetVarSize(attr.Data)
	}
	return sz
}

// MarshalJSON implements the json Marschaller interface
func (attr *Attribute) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"usage": attr.Usage.String(),
		"data":  hex.EncodeToString(attr.Data),
	})
}
