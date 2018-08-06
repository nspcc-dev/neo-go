package transaction

import (
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/version"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

type Contract struct {
	*Base
}

func NewContract(ver version.TX) *Contract {
	basicTrans := createBaseTransaction(types.Contract, ver)

	contract := &Contract{
		basicTrans,
	}
	contract.encodeExclusive = contract.encodeExcl
	contract.decodeExclusive = contract.decodeExcl
	return contract
}

func (c *Contract) encodeExcl(bw *util.BinWriter) {
	return
}

func (c *Contract) decodeExcl(br *util.BinReader) {
	return
}
