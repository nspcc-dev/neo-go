package types

import (
	"github.com/CityOfZion/neo-go/pkg/wire/util"
)

// TX is the type of a transaction.
type TX uint8

const (
	Miner      TX = 0x00
	Issue      TX = 0x01
	Claim      TX = 0x02
	Enrollment TX = 0x20
	Voting     TX = 0x24
	Register   TX = 0x40
	Contract   TX = 0x80
	State      TX = 0x90
	Agency     TX = 0xb0
	Publish    TX = 0xd0
	Invocation TX = 0xd1
)

func (t *TX) Encode(bw *util.BinWriter) {
	bw.Write(t)
}
func (t *TX) Decode(br *util.BinReader) {
	br.Read(t)
}

// String implements the stringer interface.
func (t TX) String() string {
	switch t {
	case Miner:
		return "MinerTransaction"
	case Issue:
		return "IssueTransaction"
	case Claim:
		return "ClaimTransaction"
	case Enrollment:
		return "EnrollmentTransaction"
	case Voting:
		return "VotingTransaction"
	case Register:
		return "RegisterTransaction"
	case Contract:
		return "ContractTransaction"
	case State:
		return "StateTransaction"
	case Agency:
		return "AgencyTransaction"
	case Publish:
		return "PublishTransaction"
	case Invocation:
		return "InvocationTransaction"
	default:
		return "UnkownTransaction"
	}
}
