package transaction

import (
	"bufio"

	"github.com/CityOfZion/neo-go/pkg/wire/payload/transaction/types"
)

func FromBytes(reader *bufio.Reader) (Transactioner, error) {

	t, err := reader.Peek(1)

	typ := types.TX(t[0])
	var trans Transactioner

	switch typ {
	case types.Miner:
		miner := NewMiner(0)
		err = miner.Decode(reader)
		trans = miner
	case types.Contract:
		contract := NewContract(0)
		err = contract.Decode(reader)
		trans = contract
	case types.Invocation:
		invoc := NewInvocation(0)
		err = invoc.Decode(reader)
		trans = invoc
	case types.Claim:
		claim := NewClaim(0)
		err = claim.Decode(reader)
		trans = claim
	}
	return trans, err
}
