package transaction

import "github.com/CityOfZion/neo-go/pkg/wire/util"

type ContractTransaction struct {
	BasicTransaction
}

func NewContractTransaction() *ContractTransaction {
	contract := &ContractTransaction{}
	contract.encodeExclusive = contract.encodeExcl
	contract.decodeExclusive = contract.decodeExcl
	return contract
}

func (c *ContractTransaction) encodeExcl(bw *util.BinWriter) {
	return
}

func (c *ContractTransaction) decodeExcl(br *util.BinReader) {
	return
}
