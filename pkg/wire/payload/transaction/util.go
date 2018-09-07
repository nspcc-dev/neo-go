package transaction

import (
	"bufio"
	"encoding/hex"
	"errors"

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
	case types.Register:
		reg := NewRegister(0)
		err = reg.Decode(reader)
		trans = reg
	case types.Issue:
		iss := NewIssue(0)
		err = iss.Decode(reader)
		trans = iss
	case types.Publish:
		pub := NewPublish(0)
		err = pub.Decode(reader)
		trans = pub
	case types.State:
		sta := NewStateTX(0)
		err = sta.Decode(reader)
		trans = sta
	case types.Enrollment:
		enr := NewEnrollment(0)
		err = enr.Decode(reader)
		trans = enr
	case types.Agency:
		err = errors.New("Unsupported transaction type: Agency")
	default:
		err = errors.New("Unsupported transaction with byte type " + hex.EncodeToString([]byte{t[0]}))
	}
	return trans, err
}
