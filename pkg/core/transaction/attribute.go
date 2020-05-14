package transaction

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/io"
)

// Attribute represents a Transaction attribute.
type Attribute struct {
	Usage AttrUsage
	Data  []byte
}

// attrJSON is used for JSON I/O of Attribute.
type attrJSON struct {
	Usage string `json:"usage"`
	Data  string `json:"data"`
}

// DecodeBinary implements Serializable interface.
func (attr *Attribute) DecodeBinary(br *io.BinReader) {
	attr.Usage = AttrUsage(br.ReadB())

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
		var urllen = br.ReadB()
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
	bw.WriteB(byte(attr.Usage))
	switch attr.Usage {
	case ECDH02, ECDH03:
		bw.WriteBytes(attr.Data[1:])
	case Description, Remark, Remark1, Remark2, Remark3, Remark4,
		Remark5, Remark6, Remark7, Remark8, Remark9, Remark10, Remark11,
		Remark12, Remark13, Remark14, Remark15:
		bw.WriteVarBytes(attr.Data)
	case DescriptionURL:
		bw.WriteB(byte(len(attr.Data)))
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
	return json.Marshal(attrJSON{
		Usage: attr.Usage.String(),
		Data:  hex.EncodeToString(attr.Data),
	})
}

// UnmarshalJSON implements the json.Unmarshaller interface.
func (attr *Attribute) UnmarshalJSON(data []byte) error {
	aj := new(attrJSON)
	err := json.Unmarshal(data, aj)
	if err != nil {
		return err
	}
	binData, err := hex.DecodeString(aj.Data)
	if err != nil {
		return err
	}
	switch aj.Usage {
	case "ContractHash":
		attr.Usage = ContractHash
	case "ECDH02":
		attr.Usage = ECDH02
	case "ECDH03":
		attr.Usage = ECDH03
	case "Script":
		attr.Usage = Script
	case "Vote":
		attr.Usage = Vote
	case "CertURL":
		attr.Usage = CertURL
	case "DescriptionURL":
		attr.Usage = DescriptionURL
	case "Description":
		attr.Usage = Description
	case "Hash1":
		attr.Usage = Hash1
	case "Hash2":
		attr.Usage = Hash2
	case "Hash3":
		attr.Usage = Hash3
	case "Hash4":
		attr.Usage = Hash4
	case "Hash5":
		attr.Usage = Hash5
	case "Hash6":
		attr.Usage = Hash6
	case "Hash7":
		attr.Usage = Hash7
	case "Hash8":
		attr.Usage = Hash8
	case "Hash9":
		attr.Usage = Hash9
	case "Hash10":
		attr.Usage = Hash10
	case "Hash11":
		attr.Usage = Hash11
	case "Hash12":
		attr.Usage = Hash12
	case "Hash13":
		attr.Usage = Hash13
	case "Hash14":
		attr.Usage = Hash14
	case "Hash15":
		attr.Usage = Hash15
	case "Remark":
		attr.Usage = Remark
	case "Remark1":
		attr.Usage = Remark1
	case "Remark2":
		attr.Usage = Remark2
	case "Remark3":
		attr.Usage = Remark3
	case "Remark4":
		attr.Usage = Remark4
	case "Remark5":
		attr.Usage = Remark5
	case "Remark6":
		attr.Usage = Remark6
	case "Remark7":
		attr.Usage = Remark7
	case "Remark8":
		attr.Usage = Remark8
	case "Remark9":
		attr.Usage = Remark9
	case "Remark10":
		attr.Usage = Remark10
	case "Remark11":
		attr.Usage = Remark11
	case "Remark12":
		attr.Usage = Remark12
	case "Remark13":
		attr.Usage = Remark13
	case "Remark14":
		attr.Usage = Remark14
	case "Remark15":
		attr.Usage = Remark15
	default:
		return errors.New("wrong Usage")

	}
	attr.Data = binData
	return nil
}
