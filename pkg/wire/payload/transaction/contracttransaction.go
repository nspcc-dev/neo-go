package transaction

import "github.com/CityOfZion/neo-go/pkg/wire/util"

type ContractTransaction struct {
	*BasicTransaction
}

func NewContractTransaction() *ContractTransaction {
	basicTrans := createBasicTransaction(ContractType, ContractVersion)

	contract := &ContractTransaction{
		basicTrans,
	}
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
