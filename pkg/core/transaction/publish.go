package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
)

// PublishTX represents a publish transaction.
// NOTE: This is deprecated and should no longer be used.
type PublishTX struct {
	Script      []byte
	ParamList   []smartcontract.ParamType
	ReturnType  smartcontract.ParamType
	NeedStorage bool
	Name        string
	CodeVersion string
	Author      string
	Email       string
	Description string
	Version     uint8 // Version of the parent struct Transaction. Used in reading NeedStorage flag.
}

// DecodeBinary implements the Payload interface.
func (tx *PublishTX) DecodeBinary(br *io.BinReader) error {
	tx.Script = br.ReadBytes()

	lenParams := br.ReadVarUint()
	tx.ParamList = make([]smartcontract.ParamType, lenParams)
	for i := 0; i < int(lenParams); i++ {
		var ptype uint8
		br.ReadLE(&ptype)
		tx.ParamList[i] = smartcontract.ParamType(ptype)
	}

	var rtype uint8
	br.ReadLE(&rtype)
	tx.ReturnType = smartcontract.ParamType(rtype)

	if tx.Version >= 1 {
		br.ReadLE(&tx.NeedStorage)
	} else {
		tx.NeedStorage = false
	}

	tx.Name = br.ReadString()
	tx.CodeVersion = br.ReadString()
	tx.Author = br.ReadString()
	tx.Email = br.ReadString()
	tx.Description = br.ReadString()

	return br.Err
}

// EncodeBinary implements the Payload interface.
func (tx *PublishTX) EncodeBinary(bw *io.BinWriter) error {
	bw.WriteBytes(tx.Script)
	bw.WriteVarUint(uint64(len(tx.ParamList)))
	for _, param := range tx.ParamList {
		bw.WriteLE(uint8(param))
	}
	bw.WriteLE(uint8(tx.ReturnType))
	if tx.Version >= 1 {
		bw.WriteLE(tx.NeedStorage)
	}
	bw.WriteString(tx.Name)
	bw.WriteString(tx.CodeVersion)
	bw.WriteString(tx.Author)
	bw.WriteString(tx.Email)
	bw.WriteString(tx.Description)
	return bw.Err
}

// Size returns serialized binary size for this transaction.
func (tx *PublishTX) Size() int {
	sz := io.GetVarSize(tx.Script) + io.GetVarSize(uint64(len(tx.ParamList)))
	sz += 1 * len(tx.ParamList)
	sz++
	if tx.Version >= 1 {
		sz++
	}
	sz += io.GetVarSize(tx.Name) + io.GetVarSize(tx.CodeVersion)
	sz += io.GetVarSize(tx.Author) + io.GetVarSize(tx.Email)
	sz += io.GetVarSize(tx.Description)
	return sz
}
