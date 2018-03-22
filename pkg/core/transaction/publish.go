package transaction

import (
	"encoding/binary"
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
	var err error

	tx.Script, err = util.ReadVarBytes(r)
	if err != nil {
		return err
	}

	lenParams := util.ReadVarUint(r)
	tx.ParamList = make([]smartcontract.ParamType, lenParams)
	for i := 0; i < int(lenParams); i++ {
		var ptype uint8
		if err := binary.Read(r, binary.LittleEndian, &ptype); err != nil {
			return err
		}
		tx.ParamList[i] = smartcontract.ParamType(ptype)
	}

	var rtype uint8
	if err := binary.Read(r, binary.LittleEndian, &rtype); err != nil {
		return err
	}
	tx.ReturnType = smartcontract.ParamType(rtype)

	if err := binary.Read(r, binary.LittleEndian, &tx.NeedStorage); err != nil {
		return err
	}

	tx.Name, err = util.ReadVarString(r)
	if err != nil {
		return err
	}
	tx.CodeVersion, err = util.ReadVarString(r)
	if err != nil {
		return err
	}
	tx.Author, err = util.ReadVarString(r)
	if err != nil {
		return err
	}
	tx.Email, err = util.ReadVarString(r)
	if err != nil {
		return err
	}
	tx.Description, err = util.ReadVarString(r)
	if err != nil {
		return err
	}

	return nil
}

// EncodeBinary implements the Payload interface.
func (tx *PublishTX) EncodeBinary(w io.Writer) error {
	return nil
}
