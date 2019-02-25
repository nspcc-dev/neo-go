package transaction

import (
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
	Owner PublicKey

	Admin util.Uint160
}

func NewRegister(ver version.TX) *Register {
	basicTrans := createBaseTransaction(types.Register, ver)
	Register := &Register{}
	Register.Base = basicTrans
	Register.encodeExclusive = Register.encodeExcl
	Register.decodeExclusive = Register.decodeExcl
	return Register
}

func (r *Register) encodeExcl(bw *util.BinWriter) {
	bw.Write(r.AssetType)
	bw.VarString(r.Name)
	bw.Write(r.Amount)
	bw.Write(r.Precision)
	r.Owner.Encode(bw)
	bw.Write(r.Admin)
}

func (r *Register) decodeExcl(br *util.BinReader) {
	br.Read(&r.AssetType)
	r.Name = br.VarString()
	br.Read(&r.Amount)
	br.Read(&r.Precision)
	r.Owner.Decode(br)
	br.Read(&r.Admin)
}
