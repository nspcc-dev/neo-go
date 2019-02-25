package transaction

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Publish struct {
	*Base
	Script      []byte
	ParamList   []ParamType
	ReturnType  ParamType
	NeedStorage byte
	Name        string
	CodeVersion string
	Author      string
	Email       string
	Description string
}

func NewPublish(ver version.TX) *Publish {
	basicTrans := createBaseTransaction(types.Publish, ver)

	Publish := &Publish{}
	Publish.Base = basicTrans
	Publish.encodeExclusive = Publish.encodeExcl
	Publish.decodeExclusive = Publish.decodeExcl
	return Publish
}

func (p *Publish) encodeExcl(bw *util.BinWriter) {
	bw.VarBytes(p.Script)
	bw.VarUint(uint64(len(p.ParamList)))
	for _, param := range p.ParamList {
		bw.Write(param)
	}

	bw.Write(p.ReturnType)
	switch p.Version {
	case 0:
		p.NeedStorage = byte(0)
	case 1:
		bw.Write(p.NeedStorage)
	default:
		bw.Err = errors.New("Version Number unknown for Publish Transaction")
	}

	bw.VarString(p.Name)
	bw.VarString(p.CodeVersion)
	bw.VarString(p.Author)
	bw.VarString(p.Email)
	bw.VarString(p.Description)

}

func (p *Publish) decodeExcl(br *util.BinReader) {
	p.Script = br.VarBytes()

	lenParams := br.VarUint()
	p.ParamList = make([]ParamType, lenParams)
	for i := 0; i < int(lenParams); i++ {
		var ptype uint8
		br.Read(&ptype)
		p.ParamList[i] = ParamType(ptype)
	}

	var rtype uint8
	br.Read(&rtype)
	p.ReturnType = ParamType(rtype)

	switch p.Version {
	case 0:
		p.NeedStorage = byte(0)
	case 1:
		br.Read(&p.NeedStorage)
	default:
		br.Err = errors.New("Version Number unknown for Publish Transaction")
	}

	p.Name = br.VarString()
	p.CodeVersion = br.VarString()
	p.Author = br.VarString()
	p.Email = br.VarString()
	p.Description = br.VarString()
}
