package transaction

import (
	"io"

	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
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
}

// DecodeBinary implements the Payload interface.
func (tx *PublishTX) DecodeBinary(r io.Reader) error {
	br := util.BinReader{R: r}
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

	br.ReadLE(&tx.NeedStorage)

	tx.Name = br.ReadString()
	tx.CodeVersion = br.ReadString()
	tx.Author = br.ReadString()
	tx.Email = br.ReadString()
	tx.Description = br.ReadString()

	return br.Err
}

// EncodeBinary implements the Payload interface.
func (tx *PublishTX) EncodeBinary(w io.Writer) error {
	return nil
}
