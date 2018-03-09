package transaction

// TXType is the type of a transaction.
type TXType uint8

// All processes in NEO system are recorded in transactions.
// There are several types of transactions.
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
