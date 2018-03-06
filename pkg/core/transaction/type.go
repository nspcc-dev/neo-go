package transaction

// Type is the type of a transaction.
type Type uint8

// All processes in NEO system are recorded in transactions.
// There are several types of transactions.
const (
	MinerType      Type = 0x00
	IssueType      Type = 0x01
	ClaimType      Type = 0x02
	EnrollmentType Type = 0x20
	VotingType     Type = 0x24
	RegisterType   Type = 0x40
	ContractType   Type = 0x80
	StateType      Type = 0x90
	AgencyType     Type = 0xb0
	PublishType    Type = 0xd0
	InvocationType Type = 0xd1
)

// String implements the stringer interface.
func (t Type) String() string {
	switch t {
	case MinerType:
		return "miner transaction"
	case IssueType:
		return "issue transaction"
	case ClaimType:
		return "claim transaction"
	case EnrollmentType:
		return "enrollment transaction"
	case VotingType:
		return "voting transaction"
	case RegisterType:
		return "register transaction"
	case ContractType:
		return "contract transaction"
	case StateType:
		return "state transaction"
	case AgencyType:
		return "agency transaction"
	case PublishType:
		return "publish transaction"
	case InvocationType:
		return "invocation transaction"
	default:
		return ""
	}
}
