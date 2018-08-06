package transaction

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/CityOfZion/neo-go/pkg/wire/util/fixed8"
)

type Register struct {
	*Base
	// The type of the asset being registered.
	AssetType AssetType

	// Name of the asset being registered.
	Name string

	// Amount registered
	// Unlimited mode -0.00000001
	Amount fixed8.Fixed8

	// Decimals
	Precision uint8

	// Public key of the owner
	Owner []byte

	Admin util.Uint160
}

func NewRegister(ver version.TX) *Register {
	basicTrans := createBaseTransaction(types.Register, ver)

	Register := &Register{
		basicTrans,
		0,
		"",
		0,
		0,
		nil,
		util.Uint160{},
	}
	Register.encodeExclusive = Register.encodeExcl
	Register.decodeExclusive = Register.decodeExcl
	return Register
}

func (r *Register) encodeExcl(bw *util.BinWriter) {
	bw.Write(r.AssetType)
	bw.VarString(r.Name)
	bw.Write(r.Amount)
	bw.Write(r.Precision)
	bw.Write(r.Owner)
	bw.Write(r.Admin)
	return
}

func (r *Register) decodeExcl(br *util.BinReader) {
	br.Read(&r.AssetType)
	r.Name = br.VarString()
	br.Read(&r.Amount)
	br.Read(&r.Precision)

	var prefix uint8
	br.Read(&prefix)

	// Compressed public keys.
	if prefix == 0x02 || prefix == 0x03 {
		r.Owner = make([]byte, 32)
		br.Read(r.Owner)
	} else if prefix == 0x04 {
		r.Owner = make([]byte, 65)
		br.Read(r.Owner)
	} else {
		br.Err = errors.New("Prefix not recognised for public key")
	}

	r.Owner = append([]byte{prefix}, r.Owner...)
	br.Read(&r.Admin)
}
