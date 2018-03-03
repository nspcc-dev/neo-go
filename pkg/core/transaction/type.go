package transaction

// TransactionType is the type of a transaction.
type TransactionType uint8

// All processes in NEO system are recorded in transactions.
// There are several types of transactions.
const (
	MinerType      TransactionType = 0x00
	IssueType      TransactionType = 0x01
	ClaimType      TransactionType = 0x02
	EnrollmentType TransactionType = 0x20
	VotingType     TransactionType = 0x24
	RegisterType   TransactionType = 0x40
	ContractType   TransactionType = 0x80
	StateType      TransactionType = 0x90
	AgencyType     TransactionType = 0xb0
	PublishType    TransactionType = 0xd0
	InvocationType TransactionType = 0xd1
)

// String implements the stringer interface.
func (t TransactionType) String() string {
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
