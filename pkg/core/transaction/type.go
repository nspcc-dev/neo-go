package transaction

// TXType is the type of a transaction.
type TXType uint8

const (
	MinerType      TXType = 0x00
	IssueType      TXType = 0x01
	ClaimType      TXType = 0x02
	EnrollmentType TXType = 0x20
	VotingType     TXType = 0x24
	RegisterType   TXType = 0x40
	ContractType   TXType = 0x80
	StateType      TXType = 0x90
	AgencyType     TXType = 0xb0
	PublishType    TXType = 0xd0
	InvocationType TXType = 0xd1
)

// String implements the stringer interface.
func (t TXType) String() string {
	switch t {
	case MinerType:
		return "MinerTransaction"
	case IssueType:
		return "IssueTransaction"
	case ClaimType:
		return "ClaimTransaction"
	case EnrollmentType:
		return "EnrollmentTransaction"
	case VotingType:
		return "VotingTransaction"
	case RegisterType:
		return "RegisterTransaction"
	case ContractType:
		return "ContractTransaction"
	case StateType:
		return "StateTransaction"
	case AgencyType:
		return "AgencyTransaction"
	case PublishType:
		return "PublishTransaction"
	case InvocationType:
		return "InvocationTransaction"
	default:
		return "UnkownTransaction"
	}
}
