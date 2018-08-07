package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Claim struct {
	*Base
	Claims []*Input
}

func NewClaim(ver version.TX) *Claim {
	basicTrans := createBaseTransaction(types.Contract, ver)

	claim := &Claim{}
	claim.Base = basicTrans
	claim.encodeExclusive = claim.encodeExcl
	claim.decodeExclusive = claim.decodeExcl
	return claim
}

func (c *Claim) encodeExcl(bw *util.BinWriter) {

	bw.VarUint(uint64(len(c.Claims)))
	for _, claim := range c.Claims {
		claim.Encode(bw)
	}
}

func (c *Claim) decodeExcl(br *util.BinReader) {
	lenClaims := br.VarUint()

	c.Claims = make([]*Input, lenClaims)
	for i := 0; i < int(lenClaims); i++ {
		c.Claims[i] = &Input{}
		c.Claims[i].Decode(br)
	}

}

// use encode and decode exclusive to make the interface
