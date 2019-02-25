package transaction

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
	"github.com/CityOfZion/neo-go/pkg/wire/util/fixed8"
)

type Invocation struct {
	*Base
	Script []byte
	Gas    fixed8.Fixed8
}

func NewInvocation(ver version.TX) *Invocation {
	basicTrans := createBaseTransaction(types.Invocation, ver)

	invocation := &Invocation{}
	invocation.Base = basicTrans
	invocation.encodeExclusive = invocation.encodeExcl
	invocation.decodeExclusive = invocation.decodeExcl
	return invocation
}

func (c *Invocation) encodeExcl(bw *util.BinWriter) {
	bw.VarUint(uint64(len(c.Script)))
	bw.Write(c.Script)

	switch c.Version {
	case 0:
		c.Gas = fixed8.Fixed8(0)
	case 1:
		bw.Write(&c.Gas)
	default:
		bw.Write(&c.Gas)
	}

	return
}

func (c *Invocation) decodeExcl(br *util.BinReader) {

	lenScript := br.VarUint()
	c.Script = make([]byte, lenScript)
	br.Read(&c.Script)

	switch c.Version {
	case 0:
		c.Gas = fixed8.Fixed8(0)
	case 1:
		br.Read(&c.Gas)
	default:
		br.Err = errors.New("Invalid Version Number for Invocation Transaction")
	}
	return
}
